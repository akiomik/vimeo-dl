.PHONY: build clean fmt test

build:
	go build -v .

clean:
	go clean

fmt:
	go fmt ./...

test:
	go test -v ./...
