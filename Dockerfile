FROM golang:1.27.3-alpine3.24 AS build

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /vimeo-dl .


############################################################

FROM alpine:3.24 AS no-ffmpeg
COPY --from=build /vimeo-dl /usr/bin/vimeo-dl
WORKDIR /downloads
ENTRYPOINT [ "vimeo-dl" ]

############################################################

FROM lscr.io/linuxserver/ffmpeg:5.1.2 AS with-ffmpeg
COPY --from=build /vimeo-dl /usr/bin/vimeo-dl
WORKDIR /downloads
ENTRYPOINT [ "vimeo-dl" ]

