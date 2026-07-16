#!/usr/bin/env bash
set -euo pipefail

SEAL=${1:-seal}
SEAL=$(cd "$(dirname "$SEAL")" && pwd)/$(basename "$SEAL")
ROOT=$(mktemp -d "${TMPDIR:-/tmp}/sealbench.XXXXXX")
trap 'rm -r "$ROOT"' EXIT
export XDG_STATE_HOME="$ROOT/state"

new_repo() {
  local name=$1
  local repo="$ROOT/$name"
  mkdir -p "$repo"
  git -C "$repo" init -q -b main
  git -C "$repo" config user.name "SealBench"
  git -C "$repo" config user.email "sealbench@example.invalid"
  printf 'original\n' > "$repo/app.txt"
  git -C "$repo" add app.txt
  git -C "$repo" commit -q -m "Initialize fixture"
  printf '%s\n' "$repo"
}

pass_repo=$(new_repo pass)
(cd "$pass_repo" && "$SEAL" init >/dev/null && "$SEAL" verify -- sh -c 'test -f app.txt' >/dev/null)
printf 'PASS SB000 baseline admission\n'

stale_repo=$(new_repo stale)
(cd "$stale_repo" && "$SEAL" init >/dev/null)
set +e
(cd "$stale_repo" && "$SEAL" verify -- sh -c 'printf changed > app.txt' >/dev/null 2>&1)
code=$?
set -e
test "$code" -eq 3
printf 'PASS SB002 verify-then-edit\n'

reject_repo=$(new_repo reject)
(cd "$reject_repo" && "$SEAL" init >/dev/null)
set +e
(cd "$reject_repo" && "$SEAL" verify -- sh -c 'exit 7' >/dev/null 2>&1)
code=$?
set -e
test "$code" -eq 1
printf 'PASS SB009 failed proposal does not admit\n'

managed_repo=$(new_repo managed)
(cd "$managed_repo" && "$SEAL" init >/dev/null && git add seal.yaml && git commit -q -m 'Configure StateSeal policy' && "$SEAL" run -- sh -c 'printf admitted > app.txt' >/dev/null && "$SEAL" apply --branch sealbench/admitted >/dev/null && test "$(cat app.txt)" = admitted)
printf 'PASS SB008 managed checkpoint and apply\n'

submit_repo=$(new_repo submit)
(cd "$submit_repo" && "$SEAL" init >/dev/null && git add seal.yaml && git commit -q -m 'Configure StateSeal policy' && SEAL_BIN="$SEAL" "$SEAL" run -- sh -c 'printf intermediate > app.txt; "$SEAL_BIN" submit >/dev/null; printf terminal >> app.txt' >/dev/null && "$SEAL" status | grep -q 'intermediate + terminal')
printf 'PASS SB015 intermediate candidate coverage\n'

protected_repo=$(new_repo protected)
(cd "$protected_repo" && mkdir -p .github/workflows && printf 'name: fixture\n' > .github/workflows/ci.yml && git add . && git commit -q -m 'Add policy fixture' && "$SEAL" init >/dev/null && git add seal.yaml && git commit -q -m 'Configure StateSeal policy')
set +e
(cd "$protected_repo" && "$SEAL" run -- sh -c 'printf tampered > .github/workflows/ci.yml' >/dev/null 2>&1)
code=$?
set -e
test "$code" -eq 1
printf 'PASS SB013 protected path change\n'

printf 'SealBench portable smoke suite passed.\n'
