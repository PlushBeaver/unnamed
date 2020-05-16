# Unnamed

Unnamed forwards DNS queries using the specified protocol for each upstream.
It is useful when some upstreams are UDP-only and some are TCP-only.


## Installation

### Manual

Unnamed has no dependencies beyond Go standard library.

```shell
go install https://github.com/PlushBeaver/unnamed
```

If you plan to start Unnamed as a service, install the binary to system
location and deploy the systemd unit (edit `UNNAMED_OPTS` first):

```shell
$EDITOR unnamed.service
sudo install -D -m 755 -t /usr/local/bin $GOPATH/bin/unnamed
sudo install -D -m 644 -t /etc/systemd/system unnamed.service
sudo systemctl enable unnamed
sudo systemctl start unnamed
```

If you don't plan to use systemd, because Go standard library does not support
dropping privileges, to use port 53 you can set capabilities and run the
program as normal user:

```shell
sudo setcap cap_net_bind_service+ep $GOPATH/bin/unnamed
```


### NixOS

In your `/etc/nixos/configuration.nix`, attach local repository,
enable service and configure upstreams:

```nix
{
  imports = [
    # ...
    /path/to/unnamed/config.nix
  ];

  nixpkgs.config = {
    packageOverrides = pkgs: {
      # ...
      unnamed = pkgs.callPackage /path/to/unnamed {};
    };
  };

  services.unnamed = {
    enable = true;
    upstreams = [
      {
        domain = ".";
        server = "8.8.8.8";
      }
      {
        domain = ".udp.example.com";
        server = "192.0.2.100";
      }
      {
        domain = ".tcp.example.com";
        server = "192.0.2.200";
        protocol = "tcp";
      }
    ];
  };
}
```

Set `services.unnamed.resolveLocalQueries = false` if you don't want to use
Unnamed as your default resolver.


## Usage

Run unnamed as a local DNS server and point your `resolv.conf` to it:

```
nameserver 127.0.0.1
```

Alternatively you can run unnamed as e.g. dnsmasq upstream.
Unnamed only supports DNS over UDP queries and no DNSSEC, etc fancy stuff.

```
Usage of unnamed:

  unnamed -upstream .tcp.local=192.0.2.100:1053/tcp -upstream .=192.0.2.200

Default upstream protocol is UDP, default port is 53.
Longest match is preferred. Use . domain for default nameserver.

  -dumpconfig
        dump configuration on startup
  -listen string
        address to receive DNS on (default "127.0.0.1:53")
  -upstream value
        upstream 'domain=host:port/proto'
```

**Q:** Can't dnsmasq/unbound/named do the job?

**A:** No. Probably because translating protocols would be too hard
    in all scenarios these production-grade servers support.

**Q:** Why not extend <https://github.com/jrmdev/dnsplit>?

**A:** Viral GPL-3.0, external dependencies.
