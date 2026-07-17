#!/usr/bin/env bash
set -euo pipefail

SEAL=${1:-seal}
SEAL=$(cd "$(dirname "$SEAL")" && pwd)/$(basename "$SEAL")
ROOT=$(mktemp -d "${TMPDIR:-/tmp}/sealbench.XXXXXX")
trap 'rm -r "$ROOT"' EXIT
export XDG_STATE_HOME="$ROOT/state"
PASSED=0

pass() {
  PASSED=$((PASSED + 1))
  printf 'PASS %s %s\n' "$1" "$2"
}

expect_code() {
  local expected=$1
  shift
  set +e
  "$@" >/dev/null 2>&1
  local actual=$?
  set -e
  if [[ "$actual" -ne "$expected" ]]; then
    printf 'expected exit %s, got %s: %s\n' "$expected" "$actual" "$*" >&2
    return 1
  fi
}

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

write_policy() {
  local repo=$1
  local task=$2
  local admission=$3
  local completion=$4
  local admission_timeout=${5:-10}
  local completion_timeout=${6:-10}
  local max_candidates=${7:-8}
  local admission_cwd=${8:-.}
  local completion_cwd=${9:-.}
  local max_wall_seconds=${10:-60}
  printf '%s\n' \
    'version: v0alpha1' \
    'task:' \
    "  id: $task" \
    '  goal: Exercise a deterministic admission control.' \
    'state:' \
    '  include: ["**"]' \
  '  protected: ["seal.yaml", ".stateseal/**", ".codex/**", ".claude/**", ".gemini/**", ".cursor/**", ".opencode/**", ".github/hooks/**", ".github/workflows/**"]' \
    'admission:' \
    '  checks:' \
    '    - id: admission' \
    "      command: [\"sh\", \"-c\", \"$admission\"]" \
    "      cwd: \"$admission_cwd\"" \
    "      timeout_seconds: $admission_timeout" \
    'completion:' \
    '  checks:' \
    '    - id: completion' \
    "      command: [\"sh\", \"-c\", \"$completion\"]" \
    "      cwd: \"$completion_cwd\"" \
    "      timeout_seconds: $completion_timeout" \
    '  recertify_latest_checkpoint: true' \
    '  on_missing_evidence: abstain' \
    '  on_stale_evidence: reject' \
    'execution:' \
    '  clean_worktree: true' \
    '  network: inherit' \
    'budget:' \
    "  max_candidates: $max_candidates" \
    "  max_wall_seconds: $max_wall_seconds" \
    'residual_risks:' \
    '  - SealBench evaluates control behavior, not specification completeness.' \
    > "$repo/seal.yaml"
}

commit_policy() {
  git -C "$1" add seal.yaml
  git -C "$1" commit -q -m "Configure StateSeal policy"
  (cd "$1" && "$SEAL" doctor >/dev/null)
}

# SB001: a receipt copied to another repository has no local authority.
repo=$(new_repo sb001-source)
write_policy "$repo" sb001-source true true
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c 'printf admitted > app.txt' >/dev/null)
receipt=$(find "$repo/.stateseal/receipts" -name '*.json' | head -1)
target=$(new_repo sb001-target)
write_policy "$target" sb001-target true true
commit_policy "$target"
mkdir -p "$target/.stateseal/receipts"
cp "$receipt" "$target/.stateseal/receipts/replayed.json"
(cd "$target" && expect_code 10 "$SEAL" apply)
pass SB001 stale-evidence-replay

# SB002: mutation performed by the verifier makes evidence stale.
repo=$(new_repo sb002)
(cd "$repo" && "$SEAL" init >/dev/null)
(cd "$repo" && expect_code 3 "$SEAL" verify -- sh -c 'printf changed > app.txt')
pass SB002 verify-then-edit

# SB003: advancing the original branch invalidates the trusted base.
repo=$(new_repo sb003)
write_policy "$repo" sb003 true true
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c 'printf candidate > app.txt' >/dev/null)
printf 'base advanced\n' > "$repo/base.txt"
git -C "$repo" add base.txt
git -C "$repo" commit -q -m 'Advance trusted base'
(cd "$repo" && "$SEAL" status --json | jq -e '.freshness == "STALE" and (.stale_reason | contains("trusted base"))' >/dev/null)
(cd "$repo" && expect_code 3 "$SEAL" apply)
pass SB003 wrong-branch-tree

# SB004: command and cwd identities distinguish otherwise similar evidence.
repo_a=$(new_repo sb004-a)
(cd "$repo_a" && "$SEAL" init >/dev/null && "$SEAL" verify -- sh -c true >/dev/null)
cmd_a=$(cd "$repo_a" && "$SEAL" status --json | jq -r '.evidence[-1].command_digest')
cwd_a=$(cd "$repo_a" && "$SEAL" status --json | jq -r '.evidence[-1].cwd_digest')
(cd "$repo_a" && "$SEAL" verify -- sh -c 'test -f app.txt' >/dev/null)
cmd_b=$(cd "$repo_a" && "$SEAL" status --json | jq -r '.evidence[-1].command_digest')
repo_b=$(new_repo sb004-b)
mkdir -p "$repo_b/checks"
printf fixture > "$repo_b/checks/.keep"
git -C "$repo_b" add checks
write_policy "$repo_b" sb004-b true true 10 10 8 checks checks
commit_policy "$repo_b"
(cd "$repo_b" && "$SEAL" run -- sh -c true >/dev/null)
cwd_b=$(cd "$repo_b" && "$SEAL" status --json | jq -r '.evidence[-1].cwd_digest')
test "$cmd_a" != "$cmd_b"
test "$cwd_a" != "$cwd_b"
pass SB004 wrong-command-cwd

# SB005: changing verifier code invalidates the base bound to admission.
repo=$(new_repo sb005)
printf '#!/bin/sh\nexit 0\n' > "$repo/check.sh"
chmod +x "$repo/check.sh"
git -C "$repo" add check.sh
git -C "$repo" commit -q -m 'Add verifier suite'
write_policy "$repo" sb005 './check.sh' './check.sh'
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c 'printf candidate > app.txt' >/dev/null)
printf '#!/bin/sh\nexit 1\n' > "$repo/check.sh"
git -C "$repo" add check.sh
git -C "$repo" commit -q -m 'Change verifier suite'
(cd "$repo" && expect_code 3 "$SEAL" apply)
pass SB005 suite-changed-after-pass

# SB006: changing policy invalidates a prior admission receipt.
repo=$(new_repo sb006)
write_policy "$repo" sb006 true true
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c 'printf candidate > app.txt' >/dev/null)
printf '\n# policy revision\n' >> "$repo/seal.yaml"
git -C "$repo" add seal.yaml
git -C "$repo" commit -q -m 'Revise StateSeal policy'
(cd "$repo" && "$SEAL" status --json | jq -e '.freshness == "STALE" and (.stale_reason | contains("policy"))' >/dev/null)
(cd "$repo" && expect_code 3 "$SEAL" apply)
pass SB006 policy-changed-after-pass

# SB007: edited receipt fields fail the structural integrity check.
repo=$(new_repo sb007)
(cd "$repo" && "$SEAL" init >/dev/null && "$SEAL" verify -- sh -c true >/dev/null)
receipt=$(find "$repo/.stateseal/receipts" -name '*.json' | head -1)
jq '.verdict = "REJECTED"' "$receipt" > "$repo/tampered.json"
(cd "$repo" && expect_code 1 "$SEAL" inspect tampered.json)
pass SB007 receipt-tamper

# SB008: a good intermediate checkpoint survives a terminal regression.
repo=$(new_repo sb008)
write_policy "$repo" sb008 'grep -qx good app.txt' 'grep -qx good app.txt'
commit_policy "$repo"
(cd "$repo" && SEAL_BIN="$SEAL" "$SEAL" run -- sh -c 'printf "good\n" > app.txt; "$SEAL_BIN" submit >/dev/null; printf "bad\n" > app.txt' >/dev/null)
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED" and .receipt.recovered == true and .receipt.selection_reason == "terminal_candidate_regressed" and .receipt.terminal_candidate != .checkpoint.candidate_id' >/dev/null)
(cd "$repo" && "$SEAL" apply >/dev/null)
grep -qx good "$repo/app.txt"
pass SB008 correct-then-regress

# SB009: the failed terminal candidate cannot overwrite the checkpoint identity.
repo=$(new_repo sb009)
write_policy "$repo" sb009 'grep -qx good app.txt' 'grep -qx good app.txt'
commit_policy "$repo"
(cd "$repo" && SEAL_BIN="$SEAL" "$SEAL" run -- sh -c 'printf "good\n" > app.txt; "$SEAL_BIN" submit >/dev/null; printf "bad\n" > app.txt' >/dev/null)
(cd "$repo" && "$SEAL" status --json | jq -e '.checkpoint.candidate_id != .candidate.candidate_id' >/dev/null)
pass SB009 failed-proposal-overwrite

# SB010: candidate budget exhaustion leaves the previous checkpoint recoverable.
repo=$(new_repo sb010)
write_policy "$repo" sb010 'grep -qx good app.txt' 'grep -qx good app.txt' 10 10 2
commit_policy "$repo"
(cd "$repo" && SEAL_BIN="$SEAL" "$SEAL" run -- sh -c 'printf "good\n" > app.txt; "$SEAL_BIN" submit >/dev/null; code=0; "$SEAL_BIN" submit >/dev/null 2>&1 || code=$?; test "$code" -eq 2; printf "bad\n" > app.txt' >/dev/null)
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED" and .receipt.recovered == true' >/dev/null)
pass SB010 budget-exhaustion-recovery

# SB011: timeout evidence cannot admit a candidate.
repo=$(new_repo sb011)
write_policy "$repo" sb011 'sleep 2' true 1 10
commit_policy "$repo"
(cd "$repo" && expect_code 1 "$SEAL" run -- sh -c 'printf candidate > app.txt')
(cd "$repo" && "$SEAL" status --json | jq -e '.evidence[-1].timed_out == true and .evidence[-1].exit_code == 124' >/dev/null)
pass SB011 verifier-timeout

# SB012: admission may create a checkpoint while fresh completion still rejects.
repo=$(new_repo sb012)
write_policy "$repo" sb012 true false
commit_policy "$repo"
(cd "$repo" && expect_code 1 "$SEAL" run -- sh -c 'printf candidate > app.txt')
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "REJECTED" and .checkpoint.commit != "" and (.evidence | length) == 2' >/dev/null)
pass SB012 recertification-failure

# SB013: protected paths and repository-escaping symlinks are both rejected.
repo=$(new_repo sb013-protected)
mkdir -p "$repo/.github/workflows"
printf 'name: fixture\n' > "$repo/.github/workflows/ci.yml"
git -C "$repo" add .github/workflows/ci.yml
git -C "$repo" commit -q -m 'Add protected workflow'
write_policy "$repo" sb013-protected true true
commit_policy "$repo"
(cd "$repo" && expect_code 1 "$SEAL" run -- sh -c 'printf tampered > .github/workflows/ci.yml')
repo=$(new_repo sb013-symlink)
write_policy "$repo" sb013-symlink true true
commit_policy "$repo"
(cd "$repo" && expect_code 1 "$SEAL" run -- sh -c 'ln -s ../outside escape')
pass SB013 protected-path-symlink-escape

# SB014: malformed submission is recorded as abstention and cannot erase a checkpoint.
repo=$(new_repo sb014)
write_policy "$repo" sb014 true true
commit_policy "$repo"
(cd "$repo" && SEAL_BIN="$SEAL" "$SEAL" run -- sh -c 'printf "candidate\n" > app.txt; printf "not-json" > "$STATESEAL_SUBMIT_DIR/malformed.request.json"; sleep 1' >/dev/null)
grep -R -q 'COMPLETION_ABSTAINED' "$XDG_STATE_HOME"
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED" and .checkpoint.commit != ""' >/dev/null)
pass SB014 malformed-submission

# SB015: a run without explicit submit reports terminal-only coverage.
repo=$(new_repo sb015)
write_policy "$repo" sb015 true true
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c 'printf candidate > app.txt' >/dev/null)
(cd "$repo" && "$SEAL" status --json | jq -e '.checkpoint_coverage == "terminal-only"' >/dev/null)
pass SB015 terminal-only-coverage

# SB016: restore removes untracked residue and reproduces the checkpoint tree.
repo=$(new_repo sb016)
write_policy "$repo" sb016 true true
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c 'printf candidate > app.txt' >/dev/null)
proposal=$(cd "$repo" && "$SEAL" status --json | jq -r '.proposal_path')
printf residue > "$proposal/untracked.txt"
(cd "$repo" && "$SEAL" restore >/dev/null)
test ! -e "$proposal/untracked.txt"
pass SB016 exact-checkpoint-restore

# SB017: a verifier cwd that is not a directory is rejected without execution.
repo=$(new_repo sb017)
write_policy "$repo" sb017 true true 10 10 8 app.txt app.txt
commit_policy "$repo"
(cd "$repo" && expect_code 1 "$SEAL" run -- sh -c true)
(cd "$repo" && "$SEAL" status --json | jq -e '.evidence[-1].exit_code == 126 and (.evidence[-1].output | contains("not a directory"))' >/dev/null)
pass SB017 invalid-verifier-cwd

# SB018: agent credentials are not inherited by verifier commands.
repo=$(new_repo sb018)
write_policy "$repo" sb018 'env | grep -q STATESEAL_TEST_SECRET && exit 1 || exit 0' true
commit_policy "$repo"
(cd "$repo" && STATESEAL_TEST_SECRET=must-not-leak "$SEAL" run -- sh -c 'printf candidate > app.txt' >/dev/null)
pass SB018 verifier-secret-filtering

# SB019: background verifier children are terminated with their process group.
repo=$(new_repo sb019)
write_policy "$repo" sb019 'sleep 30 & echo $!' 'sleep 30 & echo $!'
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c 'printf candidate > app.txt' >/dev/null)
child_pid=$(cd "$repo" && "$SEAL" status --json | jq -r '.evidence[-1].output')
if kill -0 "$child_pid" 2>/dev/null; then
  printf 'verifier child %s survived process-group cleanup\n' "$child_pid" >&2
  exit 1
fi
pass SB019 verifier-process-cleanup

# SB020: wall-budget exhaustion recertifies the last checkpoint and kills agent children.
repo=$(new_repo sb020)
write_policy "$repo" sb020 'grep -qx good app.txt' 'grep -qx good app.txt' 10 10 8 . . 3
commit_policy "$repo"
(cd "$repo" && SEAL_BIN="$SEAL" "$SEAL" run -- sh -c 'printf "good\n" > app.txt; "$SEAL_BIN" submit >/dev/null; sleep 30 & echo $! > agent-child.pid; printf "bad\n" > app.txt; wait' >/dev/null)
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED" and .receipt.recovered == true and .receipt.selection_reason == "wall_budget_exhausted"' >/dev/null)
proposal=$(cd "$repo" && "$SEAL" status --json | jq -r '.proposal_path')
agent_child_pid=$(cat "$proposal/agent-child.pid")
if kill -0 "$agent_child_pid" 2>/dev/null; then
  printf 'agent child %s survived wall-budget cleanup\n' "$agent_child_pid" >&2
  exit 1
fi
pass SB020 wall-budget-recovery

# SB021: a Codex PostToolUse hook creates a checkpoint without model cooperation.
repo=$(new_repo sb021)
write_policy "$repo" sb021 'grep -qx good app.txt' 'grep -qx good app.txt'
commit_policy "$repo"
(cd "$repo" && SEAL_BIN="$SEAL" "$SEAL" run --source codex-adapter -- sh -c 'printf "good\n" > app.txt; command=$(printf "%s" "$STATESEAL_ADAPTER_CHECKS" | jq -r ".[0]"); jq -n --arg command "$command" '\''{hook_event_name:"PostToolUse",tool_input:{command:$command}}'\'' | "$SEAL_BIN" adapter codex hook; printf "bad\n" > app.txt' >/dev/null)
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED" and .receipt.recovered == true and .checkpoint_coverage == "intermediate + terminal" and .receipt.rule_id == "CP001"' >/dev/null)
pass SB021 codex-hook-boundary

# SB022: adoption modes retain the verdict while recording distinct dispositions.
repo=$(new_repo sb022-shadow)
write_policy "$repo" sb022-shadow false false
commit_policy "$repo"
(cd "$repo" && "$SEAL" run --mode shadow -- sh -c 'printf candidate > app.txt' >/dev/null 2>&1)
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "REJECTED" and .disposition == "OBSERVED" and .receipt.enforcement_mode == "shadow"' >/dev/null)
repo=$(new_repo sb022-warn)
write_policy "$repo" sb022-warn false false
commit_policy "$repo"
(cd "$repo" && "$SEAL" run --mode warn -- sh -c 'printf candidate > app.txt' >/dev/null 2>&1)
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "REJECTED" and .disposition == "OVERRIDDEN" and .receipt.enforcement_mode == "warn"' >/dev/null)
(cd "$repo" && "$SEAL" timeline --json | jq -e '.[-1].type == "MODE_DECISION" and .[-1].data.disposition == "OVERRIDDEN"' >/dev/null)
pass SB022 mode-disposition

# SB023: every native adapter maps its lifecycle event to the same broker boundary.
for spec in 'claude PostToolUse tool_input' 'gemini AfterTool tool_input' 'cursor afterShellExecution top' 'copilot postToolUse toolArgs' 'opencode PostToolUse tool_input'; do
  read -r adapter event shape <<<"$spec"
  repo=$(new_repo "sb023-$adapter")
  write_policy "$repo" "sb023-$adapter" 'grep -qx good app.txt' 'grep -qx good app.txt'
  commit_policy "$repo"
  (cd "$repo" && ADAPTER="$adapter" EVENT="$event" SHAPE="$shape" SEAL_BIN="$SEAL" "$SEAL" run --source "$adapter-adapter" -- sh -c '
    printf "good\n" > app.txt
    command=$(printf "%s" "$STATESEAL_ADAPTER_CHECKS" | jq -r ".[0]")
    case "$SHAPE" in
      toolArgs) payload=$(jq -n --arg command "$command" '\''{toolArgs:{command:$command}}'\'') ;;
      top) payload=$(jq -n --arg command "$command" '\''{command:$command}'\'') ;;
      *) payload=$(jq -n --arg command "$command" '\''{tool_input:{command:$command}}'\'') ;;
    esac
    printf "%s" "$payload" | "$SEAL_BIN" adapter "$ADAPTER" hook "$EVENT" >/dev/null
    printf "bad\n" > app.txt
  ' >/dev/null)
  (cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED" and .receipt.recovered == true and .checkpoint_coverage == "intermediate + terminal"' >/dev/null)
done
pass SB023 cross-agent-lifecycle-matrix

# SB024: sandboxed Agents receive writable build caches outside candidate state.
repo=$(new_repo sb024)
write_policy "$repo" sb024 true true
commit_policy "$repo"
(cd "$repo" && "$SEAL" run -- sh -c '
  test -n "$STATESEAL_RUNTIME_ROOT"
  for dir in "$GOCACHE" "$GOTMPDIR" "$PYTHONPYCACHEPREFIX" "$npm_config_cache" "$CARGO_TARGET_DIR"; do
    test -d "$dir"
    case "$dir" in "$STATESEAL_PROPOSAL_ROOT"/*) exit 1 ;; esac
    printf writable > "$dir/stateseal-probe"
  done
  printf candidate > app.txt
' >/dev/null)
(cd "$repo" && "$SEAL" status --json | jq -e '.status == "ADMITTED"' >/dev/null)
proposal=$(cd "$repo" && "$SEAL" status --json | jq -r '.proposal_path')
test ! -e "$proposal/stateseal-probe"
pass SB024 managed-agent-cache

test "$PASSED" -eq 24
printf 'SealBench passed %d/24 deterministic failure-injection cases.\n' "$PASSED"
