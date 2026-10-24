vimeo-dl
========

[![Go](https://github.com/akiomik/vimeo-dl/actions/workflows/go.yml/badge.svg)](https://github.com/akiomik/vimeo-dl/actions/workflows/go.yml)

A tool to download Vimeo videos when you have access to their JSON playback
manifest.

## Usage

```sh
vimeo-dl --combine -i "${MASTER_JSON_URL}"
```

The input must be a `master.json` or `playlist.json` playback manifest URL,
including any signed query string. A Vimeo video page URL or an HLS `.m3u8`
URL cannot be used as input. If the JSON manifest uses path-like init segments,
request it with `base64_init=1`. If multiple segments resolve to the same media
URL, request it with `query_string_ranges=1`. Quote the URL in the shell so `&`
stays inside the argument. Signed manifest URLs can expire; obtain a fresh one
if the request returns an HTTP error. The tool does not print signed media URLs.
On filesystems with POSIX permissions, downloaded media is readable only by the
current user.

## Advanced Usage

```sh
# Download a video as ${clip_id}-video.mp4 (1080p).
# The highest bitrate track is automatically selected.
vimeo-dl -i "https://skyfire.vimeocdn.com/xxx/yyy/live-archive/video/240p,360p,540p,720p,1080p/master.json?base64_init=1&query_string_ranges=1"
```

```sh
# Download a video as ${clip_id}-video.mp4 (720p) with the specified user-agent.
vimeo-dl -i "https://skyfire.vimeocdn.com/xxx/yyy/live-archive/video/240p,360p,540p,720p,1080p/master.json?base64_init=1&query_string_ranges=1" \
         --video-id "720p" \
         --user-agent "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.89 Safari/537.36"
# you can use jq command to list available formats associated ids (https://jqlang.org)
curl -s $URL | jq '[.video[] | {id, height, width, codecs}] | sort_by(.height)'
```

```sh
# Download a video as ${clip_id}.mp4.
vimeo-dl -i "https://8vod-adaptive.akamaized.net/xxx/yyy/sep/video/9f88d1ff,b83d0f9d,da44206b,f34fd50d,f9ebc26f/master.json?base64_init=1" \
         --video-id "b83d0f9d" \
         --audio-id "b83d0f9d" \
         --combine

# Download a video as my-video-file-name.mp4.
vimeo-dl -i "https://8vod-adaptive.akamaized.net/xxx/yyy/sep/video/9f88d1ff,b83d0f9d,da44206b,f34fd50d,f9ebc26f/master.json?base64_init=1" \
         --video-id "b83d0f9d" \
         --audio-id "b83d0f9d" \
         --combine \
         --output-file-name "my-video-file-name"

# The combine option is equivalent to the following command.
vimeo-dl -i "https://8vod-adaptive.akamaized.net/xxx/yyy/sep/video/9f88d1ff,b83d0f9d,da44206b,f34fd50d,f9ebc26f/master.json?base64_init=1" \
         --video-id "b83d0f9d" \
         --audio-id "b83d0f9d"
ffmpeg -i ${clip_id}-video.mp4 -i ${clip_id}-audio.mp4 -c copy ${clip_id}.mp4
```

## Options

```
Usage:
  vimeo-dl [flags]

Flags:
      --audio-id string           audio id
      --combine                   combine video and audio into a single mp4 (ffmpeg is required)
      --ffmpeg-path string        path to the ffmpeg executable for --combine
  -h, --help                      help for vimeo-dl
  -i, --input string              url for master.json or playlist.json (required)
  -o, --output-file-name string   output file name
      --user-agent string         user-agent for request
  -v, --version                   version for vimeo-dl
      --video-id string           video id
```

Go does not run `ffmpeg` when it is found implicitly in the current directory.
If `ffmpeg.exe` is beside `vimeo-dl.exe` on Windows, select it explicitly:

```powershell
.\vimeo-dl.exe --combine --ffmpeg-path ".\ffmpeg.exe" -i "$env:MASTER_JSON_URL"
```

Alternatively, install FFmpeg in an absolute `PATH` directory and run the
downloader from a directory without another `ffmpeg.exe`.

## Install

Installing or building from source requires the Go version specified in [.go-version](.go-version) or newer.

### Pre-compiled binaries

Currently, Windows, macOS and linux are supported.

- Download the latest release from [the release page](https://github.com/akiomik/vimeo-dl/releases/latest).
- Extract the downloaded `.tar.gz` file.

### go install

```sh
go install github.com/akiomik/vimeo-dl@latest
```

## Build

```sh
make build
```

## Test

```sh
make test
```

## Docker

Using Docker images is useful if you do not have ffmpeg installed.

Build docker image:

```
# build docker image (with ffmpeg)
docker build -t vimeo-dl .

# build docker image without ffmpeg
docker build -t vimeo-dl:no-ffmpeg --target=no-ffmpeg .
```

Use the image and choose an output name inside the mounted downloads directory:

```sh
mkdir -p downloads
docker run --rm -v "$(pwd)/downloads:/downloads" vimeo-dl \
  --combine -i "${MASTER_JSON_URL}" --output-file-name "my-video"
```

