{
  description = "A tool to download private videos on vimeo";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-26.05-darwin";

  outputs = { self, nixpkgs }:
    let

      version = builtins.replaceStrings [ "\n" ] [ "" ] (builtins.readFile ./VERSION);

      # System types to support.
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];

      # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });

    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = (pkgs.buildGoModule.override { go = pkgs.go_1_27; }) rec {
            pname = "vimeo-dl";
            inherit version;
            src = ./.;
            vendorHash = "sha256-9jK3jKbFp+5WSQfMbNzwIB55bC5KScZOaFHItffTF00=";
            ldflags = [ "-X github.com/akiomik/vimeo-dl/config.Version=${version}" ];
            nativeBuildInputs = with pkgs; [ makeWrapper ];
            postInstall = ''
              wrapProgram $out/bin/${pname} \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.ffmpeg ]}
            '';
          };
        });
    };
}
