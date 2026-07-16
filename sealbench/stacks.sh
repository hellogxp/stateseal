#!/usr/bin/env bash
set -euo pipefail

SEAL=${1:-seal}
SEAL=$(cd "$(dirname "$SEAL")" && pwd)/$(basename "$SEAL")
ROOT=$(mktemp -d "${TMPDIR:-/tmp}/stateseal-stacks.XXXXXX")
trap 'rm -r "$ROOT"' EXIT
export XDG_STATE_HOME="$ROOT/state"

init_repo() {
  local repo=$1
  git -C "$repo" init -q -b main
  git -C "$repo" config user.name "StateSeal Stack Matrix"
  git -C "$repo" config user.email "stack-matrix@stateseal.invalid"
  git -C "$repo" add .
  git -C "$repo" commit -q -m 'Initialize stack fixture'
  (cd "$repo" && "$SEAL" init >/dev/null)
  git -C "$repo" add seal.yaml
  git -C "$repo" commit -q -m 'Configure StateSeal policy'
}

assert_admitted() {
  local repo=$1
  (cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED" and .freshness == "CURRENT" and (.evidence | length) == 2' >/dev/null)
}

# Go: native test discovery and a managed terminal candidate.
go_repo="$ROOT/go"
mkdir -p "$go_repo"
printf 'module example.com/stateseal/stack\n\ngo 1.24\n' > "$go_repo/go.mod"
printf 'package stack\n\nfunc Add(a, b int) int { return a + b }\n' > "$go_repo/add.go"
printf 'package stack\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) { if Add(2, 3) != 5 { t.Fatal("bad sum") } }\n' > "$go_repo/add_test.go"
init_repo "$go_repo"
(cd "$go_repo" && "$SEAL" run -- sh -c 'printf "go\n" > compatibility-probe.txt' >/dev/null)
assert_admitted "$go_repo"
printf 'PASS STACK-GO managed go test\n'

# Python: pyproject detection with a hermetic pytest-compatible shim.
python_repo="$ROOT/python"
shim_dir="$ROOT/python-tools"
mkdir -p "$python_repo/tests" "$shim_dir"
printf '__pycache__/\n*.py[cod]\n' > "$python_repo/.gitignore"
printf '[project]\nname = "stateseal-stack"\nversion = "0.0.0"\n' > "$python_repo/pyproject.toml"
printf 'def add(a, b):\n    return a + b\n' > "$python_repo/example.py"
printf 'import unittest\nfrom example import add\n\nclass AddTest(unittest.TestCase):\n    def test_add(self):\n        self.assertEqual(add(2, 3), 5)\n' > "$python_repo/tests/test_example.py"
printf '#!/bin/sh\nexec python3 -m unittest discover -s tests\n' > "$shim_dir/pytest"
chmod +x "$shim_dir/pytest"
PATH="$shim_dir:$PATH" init_repo "$python_repo"
(cd "$python_repo" && PATH="$shim_dir:$PATH" "$SEAL" run -- sh -c 'printf "python\n" > compatibility-probe.txt' >/dev/null)
PATH="$shim_dir:$PATH" assert_admitted "$python_repo"
printf 'PASS STACK-PYTHON managed pytest policy\n'

# Node: evaluator must reuse an ignored node_modules dependency graph.
node_repo="$ROOT/node"
mkdir -p "$node_repo/node_modules/probe-package"
printf 'node_modules/\n' > "$node_repo/.gitignore"
printf '{"type":"module","scripts":{"test":"node --test"}}\n' > "$node_repo/package.json"
printf '{"name":"probe-package","type":"module","exports":"./index.js"}\n' > "$node_repo/node_modules/probe-package/package.json"
printf 'export const value = 42;\n' > "$node_repo/node_modules/probe-package/index.js"
printf 'import test from "node:test";\nimport assert from "node:assert/strict";\nimport {value} from "probe-package";\ntest("dependency resolution", () => assert.equal(value, 42));\n' > "$node_repo/test.js"
init_repo "$node_repo"
(cd "$node_repo" && "$SEAL" run -- sh -c 'printf "node\n" > compatibility-probe.txt' >/dev/null)
assert_admitted "$node_repo"
printf 'PASS STACK-NODE managed npm test with ignored dependencies\n'

printf 'Stack matrix passed 3/3 ecosystems.\n'
