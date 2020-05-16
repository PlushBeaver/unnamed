{ ... }:
let ghUser = "PlushBeaver";
    ghRepo = "unnamed";
in with import <nixpkgs> {};
buildGoPackage rec {
  name = "unnamed-${version}";
  version = "0.3";

  src = fetchFromGitHub {
    owner  = ghUser;
    repo   = ghRepo;
    rev    = "477f73f";
    sha256 = "1qh69sj2hmz18dm2vrvaaw11xdrz92581nqs5i6qpdyii02i5ga3";
  };

  goPackagePath = "github.com/${ghUser}/${ghRepo}";
  subPackages = ["."];

  meta = with lib; {
    homepage = "https://github.com/${ghUser}/${ghRepo}";
    description = "Forward DNS using the specified protocol for each upstream";
    longDescription = ''
      Unnamed is useful when some upstreams are UDP-only and some are TCP-only.
      It can be run be used as a standalone forwarder or as an upstream.
      This program only supports plaintext DNS over UDP queries.
    '';
    license = licenses.bsd3;
  };
}
