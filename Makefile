BINARY_NAME=unsample
GO=go
GOFLAGS=-trimpath
LDFLAGS=-s -w -X github.com/unsample/unsample/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: all build test lint clean install

all: build

build:
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY_NAME) ./cmd/unsample

test:
	$(GO) test -v -race -count=1 ./...

test-coverage:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

install:
	$(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/unsample
