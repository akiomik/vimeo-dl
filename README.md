vimeo-dl
========

[![Go](https://github.com/akiomik/vimeo-dl/actions/workflows/go.yml/badge.svg)](https://github.com/akiomik/vimeo-dl/actions/workflows/go.yml)

A tool to download private videos on vimeo.

## Usage

```sh
vimeo-dl --combine -i ${MASTER_JSON_URL}
```

## Advanced Usage

```sh
# Download a video as ${clip_id}-video.mp4 (1080p).
# The highest resolution is automatically selected.
vimeo-dl -i "https://skyfire.vimeocdn.com/xxx/yyy/live-archive/video/240p,360p,540p,720p,1080p/master.json?base64_init=1&query_string_ranges=1"
```

```sh
# Download a video as ${clip_id}-video.mp4 (720p) with the specified user-agent.
vimeo-dl -i "https://skyfire.vimeocdn.com/xxx/yyy/live-archive/video/240p,360p,540p,720p,1080p/master.json?base64_init=1&query_string_ranges=1" \
         --video-id "720p" \
         --user-agent "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.89 Safari/537.36"
```

```sh
# Download a video as ${clip_id}.mp4.
vimeo-dl -i "https://8vod-adaptive.akamaized.net/xxx/yyy/sep/video/9f88d1ff,b83d0f9d,da44206b,f34fd50d,f9ebc26f/master.json?base64_init=1" \
         --video-id "b83d0f9d" \
         --audio-id "b83d0f9d" \
         --combine

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
      --audio-id string     audio id
      --combine             combine video and audio into a single mp4 (ffmpeg is required)
  -h, --help                help for vimeo-dl
  -i, --input string        url for master.json (required)
      --user-agent string   user-agent for request
  -v, --version             version for vimeo-dl
      --video-id string     video id
```

## Install

### Pre-compiled binaries

Currently, Windows, macOS and linux are supported.

- Download the latest release from [the release page](https://github.com/akiomik/vimeo-dl/releases/latest).
- Extract the downloaded `.tar.gz` file.

### On a recent Mac

1. Download one of the Darwin files [from the release page](https://github.com/akiomik/vimeo-dl/releases/latest).
2. Unzip the file, and run `vimeo-dl` from the Terminal.
3. You will get an error message: `“vimeo-dl” cannot be opened because the developer cannot be verified.`
4. Close the error message and go to "System Preferences > Security & Privacy".
5. Find the text `“vimeo-dl” was blocked` and click on the `Allow anyway` button next to it.
6. Go back in terminal and run `vimeo-dl` again: it should work!

### go install

```sh
# go <1.6
go get -u github.com/akiomik/vimeo-dl

# go >=1.6
go install github.com/akiomik/vimeo-dl
```

## Build

```sh
make build
```

## Test

```sh
make test
```
