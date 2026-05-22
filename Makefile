BIN := uq
PREFIX ?= /usr/local
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/un7qi3inc/un7qi3-cli/internal/version.Version=$(VERSION) \
           -X github.com/un7qi3inc/un7qi3-cli/internal/version.Commit=$(COMMIT) \
           -X github.com/un7qi3inc/un7qi3-cli/internal/version.Date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/uq

install: build
	install -m 0755 bin/$(BIN) $(PREFIX)/bin/$(BIN)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/

.PHONY: build install test lint clean
