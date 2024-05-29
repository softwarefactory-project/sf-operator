{
  description = "SF Operator Project";
  nixConfig.bash-prompt = "[nix(sf-operator)] ";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/23.11";

  outputs = { self, nixpkgs }:
    let pkgs = nixpkgs.legacyPackages.x86_64-linux.pkgs;
    in {
      devShells.x86_64-linux.default = pkgs.mkShell {
        name = "SF-Operator dev shell";
        buildInputs = [
          pkgs.openshift
          pkgs.kubectl
          pkgs.go
          pkgs.gopls
          pkgs.ansible
          pkgs.jq
          pkgs.k9s
          pkgs.bashInteractive
          pkgs.python311Packages.websocket-client
          pkgs.python311Packages.kubernetes
          pkgs.openssl
        ];
        shellHook = ''
          echo "Welcome in $name"
          export LC_ALL=C.UTF-8
          export GOPATH=$(go env GOPATH)
          export GOBIN=$(go env GOPATH)/bin
        '';
      };
    };
}
