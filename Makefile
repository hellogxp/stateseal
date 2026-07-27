.PHONY: build install test lint sealbench stackbench experience live-codex release-snapshot

BUILD_VERSION ?= v0.1.0-alpha.dev.$(shell git rev-parse --short=7 HEAD)
BUILD_COMMIT ?= $(shell git rev-parse HEAD)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
INSTALL_DIR ?= $(HOME)/.local/bin
BUILDINFO_MODULE := github.com/hellogxp/stateseal/internal/buildinfo
BUILD_LDFLAGS := -s -w \
	-X $(BUILDINFO_MODULE).Version=$(BUILD_VERSION) \
	-X $(BUILDINFO_MODULE).Commit=$(BUILD_COMMIT) \
	-X $(BUILDINFO_MODULE).Date=$(BUILD_DATE)

build:
	go build -trimpath -ldflags "$(BUILD_LDFLAGS)" -o bin/seal ./cmd/seal

install: build
	mkdir -p "$(INSTALL_DIR)"
	install -m 0755 bin/seal "$(INSTALL_DIR)/seal"
	@printf 'Installed StateSeal to %s/seal\n' "$(INSTALL_DIR)"
	@printf 'If seal is not on PATH, run: export PATH="%s:$$PATH"\n' "$(INSTALL_DIR)"
	@"$(INSTALL_DIR)/seal" version

test:
	go test -race ./...

lint:
	go vet ./...

sealbench: build
	./sealbench/run.sh ./bin/seal

stackbench: build
	./sealbench/stacks.sh ./bin/seal

experience: build
	STATESEAL_BINARY="$(CURDIR)/bin/seal" ./scripts/experience.sh

live-codex: build
	STATESEAL_BINARY="$(CURDIR)/bin/seal" ./sealbench/live-codex.sh

release-snapshot:
	@test -n "$(VERSION)" || (echo "VERSION is required" >&2; exit 2)
	VERSION="$(VERSION)" ./scripts/build-release.sh
