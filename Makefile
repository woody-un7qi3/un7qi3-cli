BIN := uq
PREFIX ?= $(HOME)/.local
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/un7qi3inc/un7qi3-cli/internal/version.Version=$(VERSION) \
           -X github.com/un7qi3inc/un7qi3-cli/internal/version.Commit=$(COMMIT) \
           -X github.com/un7qi3inc/un7qi3-cli/internal/version.Date=$(DATE)

# 중복 설치 방지: PATH에 잡힐 수 있는 흔한 위치들을 install 전에 모두 청소.
KNOWN_BIN_DIRS := $(HOME)/.local/bin /usr/local/bin $(HOME)/go/bin

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/uq

uninstall:
	@for dir in $(KNOWN_BIN_DIRS); do \
	  if [ -e "$$dir/$(BIN)" ]; then \
	    echo "remove $$dir/$(BIN)"; \
	    rm -f "$$dir/$(BIN)" 2>/dev/null || sudo rm -f "$$dir/$(BIN)"; \
	  fi; \
	done

install: uninstall build
	@mkdir -p $(PREFIX)/bin
	install -m 0755 bin/$(BIN) $(PREFIX)/bin/$(BIN)
	@echo "installed: $(PREFIX)/bin/$(BIN)"

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/

.PHONY: build install uninstall test lint clean
