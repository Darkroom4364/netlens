.PHONY: build test lint clean install

BINARY := netlens
MODULE := github.com/Darkroom4364/netlens
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/netlens

install:
	go install $(LDFLAGS) ./cmd/netlens

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

bench:
	go test -bench=. -benchmem ./tomo/...

clean:
	rm -f $(BINARY) coverage.out coverage.html

fmt:
	gofmt -s -w .
	goimports -w .
