{
  description = "Panasonic TV Remote PWA";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
  };

  outputs =
    {
      self,
      nixpkgs,
      ...
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;

      mkPackage =
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        pkgs.buildGoModule {
          pname = "panaremote";
          version = "0.1.0";

          src = ./.;

          vendorHash = "sha256-6y6fVvqWcGvwIhDqWTstXRdxHjKQmWTbOP8BolZMsKs=";
        };
    in
    {
      packages = forAllSystems (system: {
        default = mkPackage system;
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
            ];
          };
        }
      );

      nixosModules.default =
        {
          lib,
          pkgs,
          config,
          ...
        }:
        let
          cfg = config.services.panaremote;
        in
        {
          options.services.panaremote = {
            enable = lib.mkEnableOption "panaremote service";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
              description = "panaremote package to use";
            };

            port = lib.mkOption {
              type = lib.types.port;
              default = 3000;
              description = "Port the server listens on";
            };
          };

          config = lib.mkIf cfg.enable {
            systemd.services.panaremote = {
              description = "Panasonic TV Remote PWA";
              wantedBy = [ "multi-user.target" ];
              after = [ "network.target" ];

              serviceConfig = {
                DynamicUser = true;
                ExecStart = "${cfg.package}/bin/panaremote";
                Restart = "on-failure";
                RestartSec = "5s";
              };
            };
          };
        };
    };
}
