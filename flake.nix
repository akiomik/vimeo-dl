{
  description = "A tool to download private videos on vimeo";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-24.05-darwin";

  outputs = { self, nixpkgs }:
    let

      # to work with older version of flakes
      lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";

      # Generate a user-friendly version number.
      version = builtins.substring 0 8 lastModifiedDate;

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
          default = pkgs.buildGoModule rec {
            pname = "vimeo-dl";
            inherit version;
            src = ./.;
            vendorHash = "sha256-hocnLCzWN8srQcO3BMNkd2lt0m54Qe7sqAhUxVZlz1k=";
            nativeBuildInputs = with pkgs; [ makeWrapper ];
            postInstall = ''
              wrapProgram $out/bin/${pname} \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.ffmpeg ]}
            '';
          };
        });
    };
}
