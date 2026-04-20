BINARY := incus-tui
PKG := ./cmd/incus-tui
VERSION ?= dev
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)

.PHONY: fmt vet test build

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)
