package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

func TestInitCreatesActionablePolicy(t *testing.T) {
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/service\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	cmd := newRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--task-id", "payment-idempotency", "--goal", "Repeated callbacks create one charge."})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	policy, _, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Task.ID != "payment-idempotency" || policy.Task.Goal != "Repeated callbacks create one charge." {
		t.Fatalf("unexpected task identity: %+v", policy.Task)
	}
	if got := policy.Completion.Checks[0].Command; !reflect.DeepEqual(got, []string{"go", "test", "./..."}) {
		t.Fatalf("unexpected verifier: %v", got)
	}
	for _, expected := range []string{"StateSeal initialized", "seal doctor", "seal verify -- go test ./...", "seal run -- <agent>"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("init output is missing %q: %s", expected, out.String())
		}
	}
}

func TestVersionCommandSupportsMachineReadableOutput(t *testing.T) {
	cmd := newRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var info map[string]any
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "commit", "build_date", "go_version", "platform"} {
		if info[key] == nil || info[key] == "" {
			t.Fatalf("version output is missing %s: %s", key, out.String())
		}
	}
}

func TestInstallCodexHooksPreservesExistingHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"policy-check"}]}]}}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installCodexHooks(path, "/usr/local/bin/seal", false); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	hooks := root["hooks"].(map[string]any)
	if hooks["PreToolUse"] == nil || hooks["PostToolUse"] == nil || hooks["Stop"] == nil {
		t.Fatalf("hook merge lost data: %s", raw)
	}
	if err := installCodexHooks(path, "/usr/local/bin/seal", false); err == nil {
		t.Fatal("duplicate StateSeal hook was accepted")
	}
	if err := installCodexHooks(path, "/opt/stateseal/seal", true); err != nil {
		t.Fatal(err)
	}
}

func TestInstallAgentAdapterMatrix(t *testing.T) {
	root := t.TempDir()
	for _, agent := range supportedAgents {
		path, err := installAgentAdapter(root, agent, "/usr/local/bin/seal", false)
		if err != nil {
			t.Fatalf("install %s: %v", agent, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s adapter: %v", agent, err)
		}
		marker := "adapter " + agent + " hook"
		if agent == "opencode" {
			marker = `"opencode", "hook"`
		}
		if !strings.Contains(string(raw), marker) {
			t.Fatalf("%s adapter does not invoke StateSeal: %s", agent, raw)
		}
		if _, err := installAgentAdapter(root, agent, "/usr/local/bin/seal", false); err == nil {
			t.Fatalf("%s adapter accepted duplicate installation", agent)
		}
		if _, err := installAgentAdapter(root, agent, "/opt/stateseal/seal", true); err != nil {
			t.Fatalf("replace %s adapter: %v", agent, err)
		}
	}
}

func TestLifecycleEventMatrix(t *testing.T) {
	cases := []struct {
		agent, tool, stop string
	}{
		{"codex", "PostToolUse", "Stop"},
		{"claude", "PostToolUse", "Stop"},
		{"gemini", "AfterTool", "AfterAgent"},
		{"cursor", "afterShellExecution", "stop"},
		{"copilot", "postToolUse", "agentStop"},
		{"opencode", "PostToolUse", "Stop"},
	}
	for _, tc := range cases {
		tool, stop := lifecycleEventKind(tc.agent, tc.tool)
		if !tool || stop {
			t.Fatalf("%s tool event was not recognized", tc.agent)
		}
		tool, stop = lifecycleEventKind(tc.agent, tc.stop)
		if tool || !stop {
			t.Fatalf("%s stop event was not recognized", tc.agent)
		}
	}
}

func TestHookCommandSupportsAgentPayloadShapes(t *testing.T) {
	for _, event := range []map[string]any{
		{"tool_input": map[string]any{"command": "go test ./..."}},
		{"toolArgs": map[string]any{"command": "go test ./..."}},
		{"command": "go test ./..."},
	} {
		if got := hookCommand(event); got != "go test ./..." {
			t.Fatalf("command not extracted from %+v: %q", event, got)
		}
	}
}

func TestCodexVerifierCommandMatching(t *testing.T) {
	configured := `["go test ./...","npm test"]`
	if !matchesVerifierCommand("  go   test ./... ", configured) {
		t.Fatal("configured verifier was not matched")
	}
	if matchesVerifierCommand("go test ./... && rm -rf build", configured) {
		t.Fatal("compound command was incorrectly matched")
	}
}

func TestShellJoinQuotesArguments(t *testing.T) {
	got := shellJoin([]string{"sh", "-c", "printf 'ok' > file"})
	if got != `sh -c 'printf '"'"'ok'"'"' > file'` {
		t.Fatalf("unexpected shell command: %s", got)
	}
}

func TestDetectChecksUsesPortableNPMTest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"test":"ava"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	checks, detected := detectChecks(root)
	if detected != "npm test" || len(checks) != 1 || !reflect.DeepEqual(checks[0].Command, []string{"npm", "test"}) {
		t.Fatalf("unexpected Node detection: %s %+v", detected, checks)
	}
}

func TestApplyCheckpointFastForwardsExactVerifiedCommit(t *testing.T) {
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Test")
	identity.Git(root, "config", "user.email", "test@example.com")
	os.WriteFile(filepath.Join(root, "app.txt"), []byte("base\n"), 0o644)
	os.WriteFile(filepath.Join(root, "seal.yaml"), []byte("policy\n"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "base")
	baseRaw, _ := identity.Git(root, "rev-parse", "HEAD")
	base := strings.TrimSpace(string(baseRaw))
	os.WriteFile(filepath.Join(root, "app.txt"), []byte("verified\n"), 0o644)
	identity.Git(root, "add", "app.txt")
	identity.Git(root, "commit", "-m", "verified")
	checkpointRaw, _ := identity.Git(root, "rev-parse", "HEAD")
	checkpoint := strings.TrimSpace(string(checkpointRaw))
	identity.Git(root, "reset", "--hard", base)
	state := protocol.TaskState{
		BaseCommit: base,
		Checkpoint: &protocol.VerifiedCheckpoint{Commit: checkpoint},
		Receipt:    &protocol.CompletionReceipt{Verdict: protocol.VerdictAdmitted, PolicyDigest: rawPolicyDigest(root)},
	}
	if err := applyCheckpoint(root, state, ""); err != nil {
		t.Fatal(err)
	}
	headRaw, _ := identity.Git(root, "rev-parse", "HEAD")
	if strings.TrimSpace(string(headRaw)) != checkpoint {
		t.Fatalf("apply did not select the exact checkpoint: %s", headRaw)
	}
}

func TestAugmentLocalToolPath(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	restore := augmentLocalToolPath(root)
	if !strings.HasPrefix(os.Getenv("PATH"), bin+string(os.PathListSeparator)) {
		t.Fatalf("tool path was not prepended: %s", os.Getenv("PATH"))
	}
	restore()
	if os.Getenv("PATH") != original {
		t.Fatal("PATH was not restored")
	}
}
