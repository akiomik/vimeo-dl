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
vimeo-dl -i "https://skyfire.vimeocdn.com/xxx/yyy/live-archive/video/240p,360p,540p,720p,1080p/master.json?base64_init=1&query_string_ranges=1" \
         -s "1080p" \
         --user-agent "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.89 Safari/537.36"
```

## Testing

```sh
go test github.com/akiomik/vimeo-dl/vimeo
```
