.PHONY: build test lint install clean

build:
	go build -o bin/miro-developer-platform-pp-cli ./cmd/miro-developer-platform-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/miro-developer-platform-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/miro-developer-platform-pp-mcp ./cmd/miro-developer-platform-pp-mcp

install-mcp:
	go install ./cmd/miro-developer-platform-pp-mcp

build-all: build build-mcp
