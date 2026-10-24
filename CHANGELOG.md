# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

For `0.x.y` releases, `x` increases for breaking changes, while `y` increases for compatible changes and bug fixes.

## [Unreleased]

## [0.3.1] - 2026-10-24

### Fixed

- Allow `--combine` to use an explicitly selected FFmpeg executable when current-directory lookup is rejected.
- Retry interrupted or truncated JSON media segments without duplicating partial bytes in the output.
- Handle the inclusive length of range-qualified JSON segments and reject missing or inconsistent segment sizes before downloading.
- Report HTTP and HTML responses clearly when a JSON manifest or media segment cannot be fetched.
- Reject repeated segment URLs and invalid initialization data before downloading, with guidance for the required manifest query parameters.
- Verify known segment sizes and resolve segment URLs against the final manifest URL after redirects.
- Stage segment downloads and clean up partial output after recoverable errors so failed downloads can be retried.
- Publish output without overwriting existing files even on filesystems that do not support hard links.

### Security

- Keep downloaded media owner-only on POSIX filesystems and omit signed media URLs from progress and request error messages.
- Exclude local private captures from Git and Docker build contexts.

### Known limitations

- An authorized `master.json` or `playlist.json` URL is still required; Vimeo video pages and HLS `.m3u8` URLs are not supported inputs.
- Browser-versus-CLI access differences for some signed JSON manifests remain under investigation in [#65](https://github.com/akiomik/vimeo-dl/issues/65).

## [0.3.0] - 2026-10-22

### Added

- Add a Nix flake for Linux and macOS builds.

### Changed

- **Breaking:** Require Go 1.27.3 or newer for source builds; release binaries require macOS 13 or newer.
- Use `go install ...@latest` in the installation instructions.
- Update Docker images to Alpine 3.24 and FFmpeg 9.0.2.
- Update the Nix package to nixpkgs 26.05 and Go 1.27.3.
- Update Cobra to 1.10.2 and pflag to 1.0.10.

### Fixed

- Show the release version in `--version` instead of the stale `0.1.0`; local builds now show `dev`.
- Restore publishing from version tags with GoReleaser.

### Security

- Build release and Docker binaries with patched Go 1.27.3.

## [0.2.0] - 2023-11-22

### Added

- Add Docker images for running with or without FFmpeg.
- Add `--output-file-name` to choose the output file name.

### Changed

- Update installation instructions and use Staticcheck for linting.

## [0.1.0] - 2022-11-03

### Added

- Add Windows release binaries.

### Changed

- Build with Go 1.19.

## [0.0.3] - 2022-01-13

### Added

- Add the Apache 2.0 license file.

### Changed

- Build with Go 1.16.

## [0.0.2] - 2022-01-13

### Added

- Add `--combine` to merge downloaded video and audio with FFmpeg.
- Add automated binary releases with GoReleaser.

### Fixed

- Improve error messages.

## [0.0.1] - 2020-07-27

### Added

- Initial release of the Vimeo video downloader.

[Unreleased]: https://github.com/akiomik/vimeo-dl/compare/v0.3.1...main
[0.3.1]: https://github.com/akiomik/vimeo-dl/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/akiomik/vimeo-dl/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/akiomik/vimeo-dl/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/akiomik/vimeo-dl/compare/v0.0.3...v0.1.0
[0.0.3]: https://github.com/akiomik/vimeo-dl/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/akiomik/vimeo-dl/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/akiomik/vimeo-dl/releases/tag/v0.0.1
