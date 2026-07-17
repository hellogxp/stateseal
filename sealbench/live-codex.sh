#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
seal=${STATESEAL_BINARY:-$root/bin/seal}

command -v codex >/dev/null 2>&1 || { echo "SKIP LIVE-CODEX: codex is not installed"; exit 0; }
if [ ! -x "$seal" ]; then
	go build -C "$root" -trimpath -o "$seal" ./cmd/seal
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/stateseal-live-codex.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
repo=$work/project
mkdir -p "$repo"
cp -R "$root/examples/regression-recovery/." "$repo/"

git -C "$repo" init -q -b main
git -C "$repo" config user.name "StateSeal Live Test"
git -C "$repo" config user.email "live-test@stateseal.dev"
git -C "$repo" add .
git -C "$repo" commit -q -m "Initialize Codex lifecycle fixture"
(
	cd "$repo"
	"$seal" adapter codex install --binary "$seal" >/dev/null
	git add .codex/hooks.json
	git commit -q -m "Configure StateSeal Codex lifecycle"

	"$seal" run --mode warn -- codex exec \
		--ephemeral \
		--dangerously-bypass-hook-trust \
		-s workspace-write \
		"Follow these steps exactly and use a separate shell command for each step. Do not combine commands. First run: cp fixtures/fixed.go.txt counter.go. Second run exactly: go test ./... . Wait for it to pass. Third run: cp fixtures/regressed.go.txt counter.go. Then finish immediately without running any more tools, without testing again, and without repairing the regression."

	status=$("$seal" status --json)
	printf '%s\n' "$status" | grep -q '"status":"ADMITTED"'
	printf '%s\n' "$status" | grep -q '"checkpoint_coverage":"intermediate + terminal"'
	printf '%s\n' "$status" | grep -q '"rule_id":"CP001"'
	printf '%s\n' "$status" | grep -q '"recovered":true'
)

printf 'PASS LIVE-CODEX lifecycle checkpoint recovery (%s)\n' "$(codex --version)"
