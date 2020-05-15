// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2020 Dmitry Kozlyuk <dmitry.kozliuk@gmail.com>

package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sort"
	"strings"
)

type Proto string

const (
	TCP Proto = "tcp"
	UDP       = "udp"
)

// Upstream describes a DNS domain, its upstream server, and protocol.
type Upstream struct {
	Domain string
	Proto  Proto
	Server net.Addr
}

// Upstreams are a collection of Upstream records
// sorted from longest to shortest.
// It can be read from command line as a repeated argument.
type Upstreams []*Upstream

func (us *Upstreams) String() string {
	var res string
	for i, u := range *us {
		res += fmt.Sprintf("-upstream %s=%s/%s", u.Domain, u.Server.String(), u.Proto)
		if i+1 < len(*us) {
			res += " "
		}
	}
	return res
}

func (us *Upstreams) Set(value string) error {
	var (
		upstream Upstream
		err      error
	)

	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return errors.New("upstream format: 'domain=host:port/proto")
	}

	upstream.Domain = parts[0]
	if !strings.HasSuffix(upstream.Domain, ".") {
		upstream.Domain += "."
	}

	parts = strings.SplitN(parts[1], "/", 2)

	proto := "udp"
	if len(parts) == 2 {
		proto = parts[1]
	}

	address := parts[0]
	if lastColon := strings.LastIndexByte(address, ':'); lastColon >= 0 {
		if firstColon := strings.IndexByte(address, ':'); firstColon != lastColon {
			if address[0] != '[' {
				address = "[" + address + "]:53"
			} else {
				address += ":53"
			}
		}
	} else {
		address += ":53"
	}

	if strings.HasPrefix(proto, "tcp") {
		upstream.Proto = TCP
		upstream.Server, err = net.ResolveTCPAddr(proto, address)
	} else if strings.HasPrefix(proto, "udp") {
		upstream.Proto = UDP
		upstream.Server, err = net.ResolveUDPAddr(proto, address)
	} else {
		return errors.New("protocol must be udp* or tcp*")
	}

	if err != nil {
		return errors.New(fmt.Sprintf("resolving upstream for %v: %v", upstream.Domain, err))
	}

	*us = append(*us, &upstream)
	sort.Sort(us)
	return nil
}

func (us *Upstreams) Len() int {
	return len(*us)
}

func (us *Upstreams) Less(i, j int) bool {
	return len((*us)[i].Domain) > len((*us)[j].Domain)
}

func (us *Upstreams) Swap(i, j int) {
	t := (*us)[i]
	(*us)[i] = (*us)[j]
	(*us)[j] = t
}

func (us *Upstreams) Find(domain string) (*Upstream, bool) {
	for _, u := range *us {
		if (u.Domain == domain) || strings.HasSuffix(domain, u.Domain) {
			return u, true
		}
	}
	return nil, false
}

// Config holds program configuration.
type Config struct {
	ListenAddress string
	Upstreams     Upstreams
}

// Job describes a name resolution parameters from query.
type Job struct {
	Config *Config
	Socket *net.UDPConn
	Client *net.UDPAddr
	Query  []byte
}

func main() {
	var config Config

	dumpConfig := flag.Bool("dumpconfig", false, "dump configuration on startup")
	flag.StringVar(&config.ListenAddress, "listen", "127.0.0.1:53", "address to receive DNS on")
	flag.Var(&config.Upstreams, "upstream", "upstream 'domain=host:port/proto'")
	flag.Usage = showUsage
	flag.Parse()

	if *dumpConfig {
		log.Println("Listen address:", config.ListenAddress)
		for _, u := range config.Upstreams {
			log.Printf("%s upstream %s for %s", strings.ToUpper(string(u.Proto)), u.Server.String(), u.Domain)
		}
	}

	address, err := net.ResolveUDPAddr("udp", config.ListenAddress)
	if err != nil {
		log.Fatalf("resolving listen address: %v\n", err)
	}

	sock, err := net.ListenUDP("udp", address)
	if err != nil {
		log.Fatalf("starting to listen: %v\n", err)
	}

	defer sock.Close()

	for {
		request := makePacketBuffer()
		size, client, err := sock.ReadFromUDP(request)
		if err != nil {
			log.Fatalf("reading request: %v\n", err)
		}

		forward(&Job{
			Config: &config,
			Socket: sock,
			Client: client,
			Query:  request[:size],
		})
	}
}

func showUsage() {
	out := flag.CommandLine.Output()
	self := os.Args[0]
	fmt.Fprintf(out, "Usage of %s:\n\n", self)
	fmt.Fprintf(out, "  %s -upstream .tcp.local=tcp://192.0.2.100:1053 -upstream .=192.0.2.200\n\n", self)
	fmt.Fprintf(out, "Default upstream protocol is UDP, default port is 53.\n")
	fmt.Fprintf(out, "Longest match is preferred. Use . domain for default nameserver.\n\n")
	flag.PrintDefaults()
}

func forward(job *Job) {
	const dnsHeaderSize = 12

	logger := log.New(log.Writer(), fmt.Sprintf("from %v: ", *job.Client), log.Flags())

	if size := len(job.Query); size < dnsHeaderSize {
		logger.Printf("dropping %v bogus bytes\n", size)
		return
	}

	flags := binary.BigEndian.Uint16(job.Query[2:4])
	if (flags & 0x8000) != 0 {
		logger.Printf("dropping response packet\n")
		return
	}

	domain, err := parseDomain(job.Query[dnsHeaderSize:])
	if err != nil {
		logger.Printf("parsing domain: %v\n", err)
		return
	}

	upstream, ok := job.Config.Upstreams.Find(domain)
	if !ok {
		logger.Printf("upstream not found for %v\n", domain)
		return
	}

	go resolve(job, upstream, logger)
}

func parseDomain(labels []byte) (string, error) {
	const (
		maxDomainLength = 255
		maxLabelLength  = 63
	)

	var domain string

	for i := 0; (i < maxDomainLength) && (i < len(labels)); {
		size := int(labels[i])

		if size == 0 {
			break
		}

		if labelEnd, packetEnd := i+size, len(labels); labelEnd > packetEnd {
			return "", errors.New(fmt.Sprintf("label length %v exceeds packet size %v", labelEnd, packetEnd))
		}

		if size >= maxLabelLength {
			return "", errors.New("compressed question label not supported")
		}

		domain += string(labels[i+1:i+1+size]) + "."

		i += 1 + size
	}

	return domain, nil
}

func resolve(job *Job, upstream *Upstream, logger *log.Logger) {
	var err error

	if upstream.Proto == UDP {
		err = resolveUDP(job, upstream.Server.(*net.UDPAddr))
	} else {
		err = resolveTCP(job, upstream.Server.(*net.TCPAddr))
	}

	if err != nil {
		logger.Printf("resolving: %v\n", err)
	}
}

func resolveUDP(job *Job, server *net.UDPAddr) error {
	conn, err := net.DialUDP(server.Network(), nil, server)
	if err != nil {
		return wrap(err, "dialing")
	}

	defer conn.Close()

	written, err := conn.Write(job.Query)
	if err != nil {
		return wrap(err, "writing query")
	}
	if written != len(job.Query) {
		return errors.New(fmt.Sprintf("only managed to forward %v query bytes of %v", written, len(job.Query)))
	}

	response := makePacketBuffer()

	read, err := conn.Read(response)
	if err != nil {
		return wrap(err, "reading response")
	}

	written, err = job.Socket.WriteToUDP(response[:read], job.Client)
	if err != nil {
		return wrap(err, "writing response")
	}
	if written != read {
		return errors.New(fmt.Sprintf("only managed to forward %v response bytes of %v", written, read))
	}

	return nil
}

func resolveTCP(job *Job, server *net.TCPAddr) error {
	conn, err := net.DialTCP(server.Network(), nil, server)
	if err != nil {
		return wrap(err, "dialing")
	}

	defer conn.Close()

	// DNS over TCP messages have a 2-byte length prefix.
	buffer := make([]byte, 2+len(job.Query))
	binary.BigEndian.PutUint16(buffer, uint16(len(job.Query)))
	copy(buffer[2:], job.Query)

	if err := writeAll(buffer, conn); err != nil {
		return wrap(err, "writing query")
	}

	if err := readAll(conn, 2, buffer); err != nil {
		return wrap(err, "reading response size")
	}

	size := binary.BigEndian.Uint16(buffer[0:2])
	response := make([]byte, size)
	if err := readAll(conn, int(size), response); err != nil {
		return wrap(err, "reading response")
	}

	written, err := job.Socket.WriteToUDP(response, job.Client)
	if err != nil {
		return wrap(err, "writing response")
	}
	if written != len(response) {
		return errors.New(fmt.Sprintf("only managed to forward %v response bytes of %v", written, len(response)))
	}

	return nil
}

func readAll(r io.Reader, size int, buffer []byte) error {
	totalRead := 0
	for totalRead < size {
		read, err := r.Read(buffer[totalRead:size])
		if err != nil {
			return err
		}
		totalRead += read
	}
	return nil
}

func writeAll(buffer []byte, w io.Writer) error {
	var totalWritten int
	for totalWritten < len(buffer) {
		written, err := w.Write(buffer[totalWritten:])
		if err != nil {
			return err
		}
		totalWritten += written
	}
	return nil
}

func makePacketBuffer() []byte {
	return make([]byte, 1536)
}

func wrap(err error, prefix string, args ...interface{}) error {
	return errors.New(fmt.Sprintf("%v: %v", prefix, err))
}
