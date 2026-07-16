.PHONY: build test lint sealbench

build:
	go build -trimpath -ldflags "-s -w" -o bin/seal ./cmd/seal

test:
	go test -race ./...

lint:
	go vet ./...

sealbench: build
	./sealbench/run.sh ./bin/seal
