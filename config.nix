{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.unnamed;
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
        type = types.listOf types.str;
        default = [];
        example = [
          "8.8.8.8"
          "192.0.2.100/tcp"
          "192.0.2.200:1053/udp"
        ];
        description = ''
          DNS servers to forward queries (IPv4 or IPv6).
          Default port is 53, default protocol is UDP.
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
          (map (spec: "-upstream " + spec) cfg.upstreams)
        ]);
      };
    };
  };
}
