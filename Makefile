.PHONY: build test lint install clean sync-spec

build:
	go build -o bin/miro-cli ./cmd/miro-cli

test:
	go test -race -failfast ./...

lint:
	golangci-lint run

install:
	go install ./cmd/miro-cli

clean:
	rm -rf bin/

sync-spec:
	curl -fsSL https://raw.githubusercontent.com/miroapp/api-clients/main/packages/generator/spec.json -o spec.json
	@echo "Synced spec.json from miroapp/api-clients ($$(wc -c < spec.json) bytes)"
