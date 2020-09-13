.PHONY: build clean test

build:
	go build -v .

clean:
	go clean

test:
	go test -v ./...
