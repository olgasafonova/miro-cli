.PHONY: build test lint install clean

build:
	go build -o bin/miro-cli ./cmd/miro-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/miro-cli

clean:
	rm -rf bin/
