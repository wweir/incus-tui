MAKEFLAGS += --jobs all
SHELL := /bin/sh

BINARY_NAME := incus-tui
PKG := ./cmd/incus-tui
GO := CGO_ENABLED=0 go
GOIMPORTS ?= $(shell command -v goimports 2>/dev/null)
BIN_DIR := bin
DIST_DIR ?= dist
LOCAL_GOEXE := $(shell go env GOEXE)
LOCAL_BINARY := $(BIN_DIR)/$(BINARY_NAME)$(LOCAL_GOEXE)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
RELEASE_PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
GO_LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildDate=$(BUILD_TIME)"

.DEFAULT_GOAL := default

.PHONY: default help fmt vet test check build run clean package release

default: vet test build

help:
	@printf '%s\n' \
		'Available targets:' \
		'  fmt      Format Go sources with gofmt and optional goimports' \
		'  vet      Run go vet ./...' \
		'  test     Run go test ./...' \
		'  check    Run fmt, vet and test' \
		'  build    Build $(LOCAL_BINARY) with version metadata' \
		'  run      Build and run the local binary' \
		'  clean    Remove build and package artifacts' \
		'  package  Build release archives for $(RELEASE_PLATFORMS)' \
		'  release  Run check and package'

fmt:
	$(GO) fmt ./...
ifneq ($(strip $(GOIMPORTS)),)
	$(GOIMPORTS) -w $$(find . -type f -name '*.go' -not -path './vendor/*')
else
	@echo "goimports not found; skipping import formatting"
endif

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

check: fmt vet test

build:
	mkdir -p $(BIN_DIR)
	$(GO) build $(GO_LDFLAGS) -o $(LOCAL_BINARY) $(PKG)

run: build
	INCUS_TUI_ELEVATION_EXECUTABLE="$(abspath $(LOCAL_BINARY))" $(LOCAL_BINARY)

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

package:
	mkdir -p $(DIST_DIR)
	@set -eu; \
	version="$(VERSION)"; \
	version="$${version#v}"; \
	for target in $(RELEASE_PLATFORMS); do \
		goos="$${target%/*}"; \
		goarch="$${target#*/}"; \
		ext=""; \
		if [ "$$goos" = "windows" ]; then ext=".exe"; fi; \
		stage_dir="$(DIST_DIR)/stage_$${goos}_$${goarch}"; \
		archive_root="$(BINARY_NAME)_$${version}_$${goos}_$${goarch}"; \
		rm -rf "$$stage_dir"; \
		mkdir -p "$$stage_dir/$$archive_root"; \
		CGO_ENABLED=0 GOOS="$$goos" GOARCH="$$goarch" go build $(GO_LDFLAGS) -o "$$stage_dir/$$archive_root/$(BINARY_NAME)$$ext" $(PKG); \
		cp LICENSE README.md "$$stage_dir/$$archive_root/"; \
		tar -C "$$stage_dir" -czf "$(DIST_DIR)/$$archive_root.tar.gz" "$$archive_root"; \
		rm -rf "$$stage_dir"; \
	done

release: check package
