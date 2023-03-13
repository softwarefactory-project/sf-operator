{
  description = "SF Operator Project";
  nixConfig.bash-prompt = "[nix(monocle-operator)] ";
  inputs = { nixpkgs.url = "github:nixos/nixpkgs/22.11"; };

  outputs = { self, nixpkgs }:
    let pkgs = nixpkgs.legacyPackages.x86_64-linux.pkgs;
    in {
      devShells.x86_64-linux.default = pkgs.mkShell {
        name = "SF-Operator dev shell";
        buildInputs = [
          # 1.25.4 in nixpkgs 22.11
          pkgs.kubectl
          # 1.19.3 in nixpkgs 22.11
          pkgs.go
          # 0.10.1 in nixpkgs 22.11
          pkgs.gopls
        ];
        shellHook = ''
          echo "Welcome in $name"
        '';
      };
    };
}
