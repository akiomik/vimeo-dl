vimeo-dl
========

## Usage

```
Usage:
  vimeo-dl [flags]

Flags:
  -h, --help                help for vimeo-dl
  -i, --input string        url for master.json (required)
  -s, --scale string        scale
      --user-agent string   user-agent for request
```

## Example

```sh
# Download a video as ${clip_id}-video.mp4 (1080p)
vimeo-dl -i "https://skyfire.vimeocdn.com/xxx/yyy/live-archive/video/240p,360p,540p,720p,1080p/master.json?base64_init=1&query_string_ranges=1"
```

```sh
# Download a video as ${clip_id}-video.mp4 (720p) with user-agent
vimeo-dl -i "https://skyfire.vimeocdn.com/xxx/yyy/live-archive/video/240p,360p,540p,720p,1080p/master.json?base64_init=1&query_string_ranges=1" \
         --video-id "720p" \
         --user-agent "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.89 Safari/537.36"
```

```sh
# Download a video as ${clip_id}-video.mp4 and ${clip_id}-audio.mp4
vimeo-dl -i "https://8vod-adaptive.akamaized.net/xxx/yyy/sep/video/9f88d1ff,b83d0f9d,da44206b,f34fd50d,f9ebc26f/master.json?base64_init=1" \
         --video-id "b83d0f9d" \
         --audio-id "b83d0f9d"

# Combine both files
ffmpeg -i ${clip_id}-video.mp -i ${clip_id}-audio.mp4 -c copy ${clip_id}.mp4
```

## Testing

```sh
go test github.com/akiomik/vimeo-dl/vimeo
```
