# Contributing

StateSeal accepts focused changes that preserve its authority boundary and keep adapters thin.

Before opening a pull request:

1. add a deterministic failure-injection test for changes to core admission behavior;
2. run `go test -race ./...`, `go vet ./...`, and `make sealbench`;
3. document both the guarantee and its limit;
4. avoid adding an LLM dependency to the admission path;
5. keep protocol changes backward-compatible or include migration notes.

Release packaging changes must also pass:

```bash
VERSION=v0.0.0-test TARGETS="$(go env GOOS)/$(go env GOARCH)" scripts/build-release.sh
```

Inspect the packaged binary with `seal version --json` and verify that version,
commit, build date, Go version, and platform are present.

Use professional, imperative commit subjects. Do not include generated credentials, local receipts, or private task state.
