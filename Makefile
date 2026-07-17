.PHONY: build test lint sealbench stackbench experience live-codex release-snapshot

build:
	go build -trimpath -ldflags "-s -w" -o bin/seal ./cmd/seal

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
