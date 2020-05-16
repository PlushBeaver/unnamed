{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.unnamed;

  upstreamType = types.submodule ({ ... }: {
    options = {
      domain = mkOption {
        type = types.str;
        default = ".";
        description = ''
          Domain name, usually starts with a dot.
          The . domain is a default upstream.
          '';
      };

      server = mkOption {
        type = types.str;
        description = ''
          Server address, an IPv4, an IPv6, or a name from /etc/hosts.
          '';
      };

      port = mkOption {
        type = types.port;
        default = 53;
        description = ''
          DNS port.
          '';
      };

      protocol = mkOption {
        type = types.enum ["udp" "tcp"];
        default = "udp";
        description = ''
          DNS transport protocol.
        '';
      };
    };
  });

in {
  options = {
    services.unnamed = {
      enable = mkOption {
        type = types.bool;
        default = false;
        description = ''
          Whether to run unnamed.
        '';
      };

      resolveLocalQueries = mkOption {
        type = types.bool;
        default = true;
        description = ''
          Whether unnamed should resolve local queries
          (i.e. add 127.0.0.1 to /etc/resolv.conf).
        '';
      };

      upstreams = mkOption {
        type = types.listOf upstreamType;
        default = [];
        example = [
          {
            domain = ".";
            server = "192.0.2.100";
          }
          {
            domain = ".local";
            server = "192.0.2.200";
            protocol = "tcp";
            port = "1053";
          }
        ];
        description = ''
          Domains and servers to which queries for them are forwarded.
        '';
      };
    };
  };

  config = mkIf cfg.enable {
    networking.nameservers = optional cfg.resolveLocalQueries "127.0.0.1";

    networking.resolvconf = mkIf cfg.resolveLocalQueries {
     useLocalResolver = mkDefault true;
    };

    systemd.services.unnamed = {
      description = "Unnamed DNS forwarder";
      after = [ "network.target" "systemd-resolved.service" ];
      wantedBy = [ "multi-user.target" ];
      path = [ pkgs.unnamed ];
      serviceConfig = {
        Type = "simple";
        User = "nobody";
        Group = "nogroup";
        AmbientCapabilities = "CAP_NET_BIND_SERVICE";
        ExecStart = toString([
          "${pkgs.unnamed}/bin/unnamed"
          "-dumpconfig"
          (map
            (u: "-upstream ${u.domain}=${u.server}:${toString(u.port)}/${u.protocol}")
            cfg.upstreams)
        ]);
      };
    };
  };
}
