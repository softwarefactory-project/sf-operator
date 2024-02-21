{
  description = "SF Operator Project";
  nixConfig.bash-prompt = "[nix(sf-operator)] ";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/23.05";
  inputs.nixpkgs-latest.url = "github:nixos/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs, nixpkgs-latest }:
    let pkgs = nixpkgs.legacyPackages.x86_64-linux.pkgs;
        pkgsLatest = nixpkgs-latest.legacyPackages.x86_64-linux.pkgs;
    in {
      devShells.x86_64-linux.default = pkgs.mkShell {
        name = "SF-Operator dev shell";
        buildInputs = [
          # 4.12.0 in nixpkgs
          pkgs.openshift
          # 1.27.1 in nixpkgs
          pkgs.kubectl
          # 1.20.4 in nixpkgs
          pkgs.go
          # 0.11.0 in nixpkgs
          pkgs.gopls
          # 2.15.0 in nixpkgs
          pkgs.ansible
          # 1.6 in nixpkgs
          pkgs.jq
          # 0.27.4 in nixpkgs
          pkgs.k9s
          # 1.5.1 in nixpkgs
          pkgs.python310Packages.websocket-client
          # 26.1.0 in nixpkgs
          pkgs.python310Packages.kubernetes
          # 3.0.10 in nixpkgs
          pkgs.openssl
          pkgsLatest.python311Packages.mkdocs-material
        ];
        shellHook = ''
          echo "Welcome in $name"
          export GOPATH=$(go env GOPATH)
          export GOBIN=$(go env GOPATH)/bin
        '';
      };
    };
}
