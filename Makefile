.PHONY: build clean update fmt test

build:
	go build -v .

clean:
	go clean
	go mod tidy

update:
	go get -u

fmt:
	go fmt ./...

lint:
	staticcheck ./...

test:
	go test -v ./...
