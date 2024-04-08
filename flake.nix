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
          pkgs.python310Packages.websocket-client
          pkgs.python310Packages.kubernetes
          pkgs.openssl
        ];
        shellHook = ''
          echo "Welcome in $name"
          export GOPATH=$(go env GOPATH)
          export GOBIN=$(go env GOPATH)/bin
        '';
      };
    };
}
