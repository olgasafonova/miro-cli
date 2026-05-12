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
