.PHONY: build test lint

BINARY := pearl

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/pearl

test:
	go test ./...

lint:
	go vet ./...
