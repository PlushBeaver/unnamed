# Unnamed

Unnamed forwards DNS queries using the specified protocol for each upstream.
It is useful when some upstreams are UDP-only and some are TCP-only.


## Installation

Unnamed has no dependencies beyond Go standard library.

```sh
go install https://github.com/PlushBeaver/unnamed
```

Because Go standard library does not support dropping privileges,
to use port 53 you can set capabilities and run the program as normal user:

```
sudo setcap cap_net_bind_service+ep $GOPATH/bin/unnamed
```


## Usage

Run unnamed as a local DNS server and point your `resolv.conf` to it:

```
nameserver 127.0.0.1
```

Alternatively you can run unnamed as e.g. dnsmasq upstream.
Unnamed only supports DNS over UDP queries and no DNSSEC, etc fancy stuff.

```
Usage of unnamed:

  unnamed -upstream .tcp.local=tcp://192.0.2.100:1053 -upstream .=192.0.2.200

Default upstream protocol is UDP, default port is 53.
Longest match is preferred. Use . domain for default nameserver.

  -listen string
        address to receive DNS on (default "127.0.0.1:53")
  -upstream value
        upstream 'domain=proto://host:port'
```

Q: Why not extend <https://github.com/jrmdev/dnsplit>?
A: Viral GPL-3.0, external dependencies.
