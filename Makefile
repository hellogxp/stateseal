.PHONY: build test lint sealbench stackbench release-snapshot

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

release-snapshot:
	@test -n "$(VERSION)" || (echo "VERSION is required" >&2; exit 2)
	VERSION="$(VERSION)" ./scripts/build-release.sh
