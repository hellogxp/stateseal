package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hellogxp/stateseal/internal/broker"
	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/worktree"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/spf13/cobra"
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
	for _, expected := range []string{"StateSeal initialized", "seal doctor", "seal adapter <agent> install", "seal run -- <agent>", "runs the configured verifier automatically"} {
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
	if err := installCodexHooks(path, "/usr/local/bin/seal", false); err != nil {
		t.Fatalf("idempotent install failed: %v", err)
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
		if _, err := installAgentAdapter(root, agent, "/usr/local/bin/seal", false); err != nil {
			t.Fatalf("idempotent %s install: %v", agent, err)
		}
		if _, err := installAgentAdapter(root, agent, "/opt/stateseal/seal", true); err != nil {
			t.Fatalf("replace %s adapter: %v", agent, err)
		}
	}
}

func TestUserIntegrationPreservesAndRemovesOnlyOwnedHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	existing := `{"theme":"dark","hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"policy-check"}]}],"Stop":[{"hooks":[{"type":"command","command":"notify"}]}]}}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := findIntegrationSpec("qoder-desktop")
	if err != nil {
		t.Fatal(err)
	}
	if err := installUserIntegration(spec, path, "/usr/local/bin/seal", false); err != nil {
		t.Fatal(err)
	}
	if err := validateUserIntegration(spec, path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "policy-check") || !strings.Contains(string(raw), "notify") || !strings.Contains(string(raw), `"theme": "dark"`) {
		t.Fatalf("integration removed unrelated configuration: %s", raw)
	}
	if _, err := os.Stat(path + ".stateseal.bak"); err != nil {
		t.Fatalf("integration did not create a safety backup: %v", err)
	}
	if err := installUserIntegration(spec, path, "/usr/local/bin/seal", false); err != nil {
		t.Fatalf("idempotent install: %v", err)
	}
	raw, _ = os.ReadFile(path)
	if got := strings.Count(string(raw), "adapter qoder hook"); got != 2 {
		t.Fatalf("idempotent integration produced %d hook entries, want 2: %s", got, raw)
	}
	removed, err := removeUserIntegration(spec, path)
	if err != nil || !removed {
		t.Fatalf("remove integration: removed=%v err=%v", removed, err)
	}
	raw, _ = os.ReadFile(path)
	if strings.Contains(string(raw), "adapter qoder hook") || !strings.Contains(string(raw), "policy-check") || !strings.Contains(string(raw), "notify") {
		t.Fatalf("uninstall did not preserve unrelated hooks: %s", raw)
	}
}

func TestIntegrationAliasesAndStaticDoctor(t *testing.T) {
	for alias, want := range map[string]string{
		"codex-desktop": "codex-desktop",
		"claude-code":   "claude",
		"qoder-ide":     "qoder",
	} {
		spec, err := findIntegrationSpec(alias)
		if err != nil || spec.ID != want {
			t.Fatalf("alias %s: spec=%+v err=%v", alias, spec, err)
		}
	}
	if _, err := findIntegrationSpec("unknown"); err == nil {
		t.Fatal("unknown integration was accepted")
	}
}

func TestProjectSetupPreviewShowsExactVerificationContract(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/preview\n"), 0o644)
	policy, err := projectPolicyPreview(root, true)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	printProjectSetupPreview(&out, root, "codex", policy, i18n.English)
	for _, expected := range []string{"First project setup", "go test ./...", "go vet ./...", "seal.yaml", ".codex/hooks.json", "isolated evaluators"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("preview missing %q: %s", expected, out.String())
		}
	}
}

func TestSetupCreatesProjectPolicyAndPreservesExistingHooks(t *testing.T) {
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "config", "user.name", "Test User")
	identity.Git(root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/setup\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(root, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hooksPath, []byte(`{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"custom-policy"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	fakeCodex := filepath.Join(binDir, "codex")
	if err := os.WriteFile(fakeCodex, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(root)

	for i := 0; i < 2; i++ {
		cmd := newRoot()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"setup", "--agent", "codex", "--binary", "/usr/local/bin/seal"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("setup %d: %v", i+1, err)
		}
		for _, expected := range []string{"StateSeal project setup", "Verification plan", "seal run \"Describe the intended outcome\""} {
			if !strings.Contains(out.String(), expected) {
				t.Fatalf("setup output missing %q: %s", expected, out.String())
			}
		}
	}
	raw, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "custom-policy") {
		t.Fatalf("setup removed an existing hook: %s", raw)
	}
	if got := strings.Count(string(raw), "adapter codex hook"); got != 2 {
		t.Fatalf("idempotent setup produced %d StateSeal hook entries, want 2: %s", got, raw)
	}
	policy, _, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Task.Goal != "Runtime goals are supplied by seal run." {
		t.Fatalf("setup embedded a runtime goal in project policy: %q", policy.Task.Goal)
	}
}

func TestAgentLaunchMatrixUsesManagedPrompt(t *testing.T) {
	for _, agent := range supportedAgents {
		args, err := agentLaunch(agent, "修复重复扣款", true, true)
		if err != nil {
			t.Fatalf("%s launch: %v", agent, err)
		}
		joined := strings.Join(args, " ")
		if args[0] != agentExecutable(agent) || !strings.Contains(joined, "修复重复扣款") || !strings.Contains(joined, "StateSeal-managed") {
			t.Fatalf("unexpected %s launch: %v", agent, args)
		}
	}
}

func TestAgentLaunchUsesSafeDefaults(t *testing.T) {
	codex, err := agentLaunch("codex", "implement validation", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(codex, " "); strings.Contains(joined, "bypass-hook-trust") {
		t.Fatalf("safe Codex launch bypassed hook trust: %v", codex)
	}
	copilot, err := agentLaunch("copilot", "implement validation", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(copilot, " "); strings.Contains(joined, "allow-all") || strings.Contains(joined, "no-ask-user") {
		t.Fatalf("safe Copilot launch enabled autonomous permissions: %v", copilot)
	}
	if _, err := agentLaunch("cursor", "implement validation", false, false); err == nil {
		t.Fatal("Cursor safe headless launch should require explicit autonomous permission")
	}
}

func TestManagedCodexChildIgnoresOuterDesktopRouting(t *testing.T) {
	args, err := agentLaunch("codex", "implement validation", false, false)
	if err != nil {
		t.Fatal(err)
	}
	args = isolateManagedChild("codex", args)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--ignore-user-config") {
		t.Fatalf("managed Codex child still loads outer MCP configuration: %v", args)
	}
	if !strings.Contains(args[len(args)-1], "do not call StateSeal MCP tools") {
		t.Fatalf("managed prompt omitted the direct-edit child boundary: %q", args[len(args)-1])
	}
}

func TestSingleGoalRunBootstrapsVerifiesAndApplies(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "config", "user.name", "Test User")
	identity.Git(root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", "app.txt")
	if _, err := identity.Git(root, "commit", "-m", "chore: initialize fixture"); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	fakeCodex := filepath.Join(binDir, "codex")
	argsPath := filepath.Join(t.TempDir(), "codex-args")
	fakeCodexScript := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\nprintf 'changed\\n' > app.txt\n", argsPath)
	if err := os.WriteFile(fakeCodex, []byte(fakeCodexScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(root)

	cmd := newRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "--agent", "codex", "--yes", "--apply", "实现安全的输入规范化"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("single-goal run failed: %v\n%s", err, out.String())
	}
	content, err := os.ReadFile(filepath.Join(root, "app.txt"))
	if err != nil || string(content) != "changed\n" {
		t.Fatalf("verified change was not applied: %q %v", content, err)
	}
	launchedArgs, err := os.ReadFile(argsPath)
	if err != nil || !strings.Contains(string(launchedArgs), "--ignore-user-config") {
		t.Fatalf("managed Codex child inherited outer MCP configuration: %q %v", launchedArgs, err)
	}
	for _, want := range []string{"StateSeal · 受控开发", "Codex 已启动", "结束前独立复验", "验证通过，可以交付", "交付依据", "已将验证通过的代码应用"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("compact Chinese experience missing %q:\n%s", want, out.String())
		}
	}
	for _, hidden := range []string{"Proposal:", "Checkpoint candidate state"} {
		if strings.Contains(out.String(), hidden) {
			t.Fatalf("ordinary experience exposed internal detail %q:\n%s", hidden, out.String())
		}
	}
	log, err := identity.Git(root, "log", "-1", "--format=%s")
	if err != nil || strings.TrimSpace(string(log)) != "feat: implement verified change" {
		t.Fatalf("unexpected delivery commit: %q %v", log, err)
	}
	branch, err := identity.Git(root, "branch", "--show-current")
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(branch)), "feature/") {
		t.Fatalf("mainline delivery did not create a feature branch: %q %v", branch, err)
	}
}

func TestDevelopmentRunRejectsEmptyDeliverable(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "config", "user.name", "Test User")
	identity.Git(root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", "app.txt")
	identity.Git(root, "commit", "-m", "chore: initialize fixture")
	binDir := t.TempDir()
	fakeCodex := filepath.Join(binDir, "codex")
	if err := os.WriteFile(fakeCodex, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(root)

	cmd := newRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "--agent", "codex", "--yes", "--no-apply", "--require-change", "Add a deliverable code change"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("empty development candidate was admitted:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "no deliverable change") || !strings.Contains(out.String(), "REJECTED") && !strings.Contains(out.String(), "Delivery requirements were not met") {
		t.Fatalf("empty candidate rejection was not explained professionally:\n%s", out.String())
	}
	state, _, err := loadState()
	if err != nil || state.Status != string(protocol.VerdictRejected) || state.RuleID != protocol.RuleNoDeliverableChange || state.Checkpoint != nil || state.Receipt == nil || state.Receipt.Verdict != protocol.VerdictRejected {
		t.Fatalf("empty candidate did not produce the stable LC004 authority decision: state=%+v err=%v", state, err)
	}
	if status, _ := identity.Git(root, "status", "--porcelain"); strings.TrimSpace(string(status)) != "" {
		t.Fatalf("empty rejected delivery changed the source workspace: %s", status)
	}
}

func TestGeneratedTaskIDIsSafeAndGoalSpecific(t *testing.T) {
	now := time.Unix(100, 200)
	a := newTaskID("实现中文目标并保持兼容", now)
	b := newTaskID("实现另一个目标", now)
	if a == b {
		t.Fatal("different goals produced the same task ID")
	}
	if err := identity.ValidateTaskID(a); err != nil {
		t.Fatalf("unsafe generated task ID %q: %v", a, err)
	}
}

func TestApplyConfirmationDefaultsToNo(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{{"y\n", true}, {"YES\n", true}, {"\n", false}, {"no\n", false}} {
		var out bytes.Buffer
		got, err := confirmApply(strings.NewReader(tc.input), &out, i18n.English)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want || !strings.Contains(out.String(), "[y/N]") {
			t.Fatalf("confirmApply(%q) = %v, want %v; output=%q", tc.input, got, tc.want, out.String())
		}
	}
}

func TestIntermediateSubmissionsDeduplicateUnchangedTrees(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Test User")
	identity.Git(root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := config.Default("deduplicate-hooks", []config.Check{{ID: "clean-diff", Command: []string{"git", "diff", "--check"}, TimeoutSeconds: 30}})
	if err := config.Write(filepath.Join(root, "seal.yaml"), policy); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", ".")
	if _, err := identity.Git(root, "commit", "-m", "Initial state"); err != nil {
		t.Fatal(err)
	}
	b, err := broker.New(root, "enforce")
	if err != nil {
		t.Fatal(err)
	}
	m, err := worktree.New(root, b.State.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = identity.Git(root, "worktree", "remove", "--force", proposal) })
	dir := filepath.Join(b.Store.Dir, "submissions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stop, done := make(chan struct{}), make(chan loopOutcome)
	go serveSubmissions(stop, done, dir, b, m, proposal, "test", "Test Agent", &bytes.Buffer{}, nil)
	first := requestAndWait(t, dir, "first")
	second := requestAndWait(t, dir, "second")
	close(stop)
	<-done
	if first.ReceiptID == "" || first.ReceiptID != second.ReceiptID {
		t.Fatalf("unchanged tree was reverified: first=%s second=%s", first.ReceiptID, second.ReceiptID)
	}
	events, err := b.Store.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	candidates := 0
	for _, event := range events {
		if event.Type == "CANDIDATE_SUBMITTED" {
			candidates++
		}
	}
	if candidates != 1 {
		t.Fatalf("got %d candidate evaluations, want 1", candidates)
	}
}

func requestAndWait(t *testing.T, dir, id string) protocol.CompletionReceipt {
	t.Helper()
	raw, _ := json.Marshal(submissionRequest{ID: id, CreatedAt: time.Now().UTC()})
	if err := os.WriteFile(filepath.Join(dir, id+".request.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		responseRaw, err := os.ReadFile(filepath.Join(dir, id+".response.json"))
		if err == nil {
			var response struct {
				Receipt protocol.CompletionReceipt `json:"receipt"`
				Error   string                     `json:"error"`
			}
			if err := json.Unmarshal(responseRaw, &response); err != nil {
				t.Fatal(err)
			}
			if response.Error != "" {
				t.Fatal(response.Error)
			}
			return response.Receipt
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("broker response timed out")
	return protocol.CompletionReceipt{}
}

func TestLifecycleEventMatrix(t *testing.T) {
	cases := []struct {
		agent, tool, stop string
	}{
		{"codex", "PostToolUse", "Stop"},
		{"claude", "PostToolUse", "Stop"},
		{"qoder", "PostToolUse", "Stop"},
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

func TestQoderStopBlockUsesOfficialExitCode(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := agentHookMessage("qoder", cmd, "tests failed", true, "Stop")
	var exitErr interface{ ExitCode() int }
	if err == nil || !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("Qoder Stop block did not use exit 2: %v", err)
	}
	if !strings.Contains(out.String(), `"decision":"block"`) {
		t.Fatalf("Qoder Stop block did not emit a reason: %s", out.String())
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

func TestDiscoverVerificationPlanUsesLayeredChecks(t *testing.T) {
	goRoot := t.TempDir()
	os.WriteFile(filepath.Join(goRoot, "go.mod"), []byte("module example.com/test\n"), 0o644)
	goPlan := discoverVerificationPlan(goRoot)
	if len(goPlan.Admission) != 1 || len(goPlan.Completion) != 2 || goPlan.Completion[1].ID != "static-analysis" {
		t.Fatalf("unexpected Go plan: %+v", goPlan)
	}
	nodeRoot := t.TempDir()
	os.WriteFile(filepath.Join(nodeRoot, "package.json"), []byte(`{"scripts":{"test":"vitest","build":"tsc","lint":"eslint .","typecheck":"tsc --noEmit"}}`), 0o644)
	nodePlan := discoverVerificationPlan(nodeRoot)
	if len(nodePlan.Admission) != 1 || len(nodePlan.Completion) != 4 {
		t.Fatalf("unexpected Node plan: %+v", nodePlan)
	}
	pythonRoot := t.TempDir()
	os.Mkdir(filepath.Join(pythonRoot, "tests"), 0o755)
	os.WriteFile(filepath.Join(pythonRoot, "pyproject.toml"), []byte("[project]\nname='fixture'\n"), 0o644)
	pythonPlan := discoverVerificationPlan(pythonRoot)
	if !reflect.DeepEqual(pythonPlan.Admission[0].Command, []string{"python3", "-m", "unittest", "discover", "-s", "tests"}) {
		t.Fatalf("unexpected Python plan: %+v", pythonPlan)
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
	if err := applyCheckpoint(root, &state, ""); err != nil {
		t.Fatal(err)
	}
	headRaw, _ := identity.Git(root, "rev-parse", "HEAD")
	if strings.TrimSpace(string(headRaw)) != checkpoint {
		t.Fatalf("apply did not select the exact checkpoint: %s", headRaw)
	}
	if state.Status != "APPLIED" || state.Freshness != "CURRENT" || state.AppliedCommit != checkpoint || state.AppliedBranch != "main" {
		t.Fatalf("apply did not record a current applied state: %+v", state)
	}
	freshness, reason := stateFreshness(root, rawPolicyDigest(root), state)
	if freshness != "CURRENT" || reason != "" {
		t.Fatalf("applied checkpoint is not current: freshness=%s reason=%s", freshness, reason)
	}
	if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("regressed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	freshness, reason = stateFreshness(root, rawPolicyDigest(root), state)
	if freshness != "STALE" || reason != "working tree changed after admission" {
		t.Fatalf("post-apply mutation was not detected: freshness=%s reason=%s", freshness, reason)
	}
}

func TestHookRuntimeRecoversContextWithoutInheritedEnvironment(t *testing.T) {
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	submitDir := filepath.Join(t.TempDir(), "submissions")
	if err := os.MkdirAll(submitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cleanup, err := writeHookRuntime(root, submitDir, "enforce", `["go test ./..."]`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	t.Setenv("STATESEAL_SUBMIT_DIR", "")
	t.Setenv("STATESEAL_MODE", "")
	t.Setenv("STATESEAL_ADAPTER_CHECKS", "")

	if !hydrateHookRuntime(map[string]any{"cwd": root}) {
		t.Fatal("hook runtime was not recovered from the managed proposal")
	}
	if got := os.Getenv("STATESEAL_SUBMIT_DIR"); got != submitDir {
		t.Fatalf("unexpected submission directory: %s", got)
	}
	if got := os.Getenv("STATESEAL_MODE"); got != "enforce" {
		t.Fatalf("unexpected enforcement mode: %s", got)
	}
	if got := os.Getenv("STATESEAL_ADAPTER_CHECKS"); got != `["go test ./..."]` {
		t.Fatalf("unexpected adapter checks: %s", got)
	}
}

func TestCodexStopHookSubmitsThroughRecoveredRuntime(t *testing.T) {
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	submitDir := filepath.Join(t.TempDir(), "submissions")
	if err := os.MkdirAll(submitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cleanup, err := writeHookRuntime(root, submitDir, "enforce", `[]`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	t.Setenv("STATESEAL_SUBMIT_DIR", "")
	t.Setenv("STATESEAL_MODE", "")
	t.Setenv("STATESEAL_ADAPTER_CHECKS", "")

	brokerResult := make(chan error, 1)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			entries, readErr := os.ReadDir(submitDir)
			if readErr != nil {
				brokerResult <- readErr
				return
			}
			for _, entry := range entries {
				if !strings.HasSuffix(entry.Name(), ".request.json") {
					continue
				}
				raw, readErr := os.ReadFile(filepath.Join(submitDir, entry.Name()))
				if readErr != nil {
					brokerResult <- readErr
					return
				}
				var request submissionRequest
				if json.Unmarshal(raw, &request) != nil || request.ID == "" {
					brokerResult <- fmt.Errorf("malformed submission request: %s", raw)
					return
				}
				response, _ := json.Marshal(map[string]any{"receipt": protocol.CompletionReceipt{Verdict: protocol.VerdictAdmitted}})
				brokerResult <- os.WriteFile(filepath.Join(submitDir, request.ID+".response.json"), response, 0o600)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		brokerResult <- fmt.Errorf("submission request was not received")
	}()

	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(`{"cwd":` + strconv.Quote(root) + `}`))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runAgentHook("codex", cmd, []string{"Stop"}); err != nil {
		t.Fatal(err)
	}
	if err := <-brokerResult; err != nil {
		t.Fatal(err)
	}
}

func TestCodexRejectedStopRequestsAnotherAgentIteration(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := agentHookMessage("codex", cmd, "StateSeal REJECTED: tests failed", true, "Stop"); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["decision"] != "block" || response["reason"] == "" {
		t.Fatalf("rejected Stop did not request Codex continuation: %v", response)
	}
	if _, exists := response["continue"]; exists {
		t.Fatalf("rejected Stop used continue, which can override continuation: %v", response)
	}
}

func TestCodexPostToolFeedbackUsesOfficialContextShape(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := agentHookMessage("codex", cmd, "StateSeal REJECTED: tests failed", false, "PostToolUse"); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	hookOutput, ok := response["hookSpecificOutput"].(map[string]any)
	if !ok || hookOutput["hookEventName"] != "PostToolUse" || hookOutput["additionalContext"] == "" {
		t.Fatalf("PostToolUse feedback did not use Codex context output: %v", response)
	}
}

func TestRunFailsBeforeStartingAgentWhenGitIdentityIsMissing(t *testing.T) {
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/preflight\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := config.Default("identity-preflight", []config.Check{{ID: "test", Command: []string{"go", "test", "./..."}}})
	if err := config.Write(filepath.Join(root, "seal.yaml"), policy); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root,
		"-c", "user.name=Bootstrap User",
		"-c", "user.email=bootstrap@example.com",
		"-c", "user.useConfigOnly=true",
		"commit", "-m", "initial",
	); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(key, "")
	}
	t.Chdir(root)
	marker := filepath.Join(root, "agent-started")
	cmd := newRoot()
	cmd.SetArgs([]string{"run", "--", "sh", "-c", "touch " + shellQuote(marker)})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "checkpoint commit requires a configured Git identity") {
		t.Fatalf("run error = %v, want actionable identity preflight", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("agent started before identity preflight completed: %v", statErr)
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

func TestAgentCacheEnvironmentUsesManagedWritablePaths(t *testing.T) {
	tempRoot := t.TempDir()
	environment, err := agentCacheEnvironment([]string{"PATH=/bin", "GOCACHE=/unwritable"}, tempRoot, "/state/task")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	if values["PATH"] != "/bin" {
		t.Fatalf("unrelated environment was not preserved: %v", values)
	}
	for _, key := range []string{"GOCACHE", "GOTMPDIR", "PYTHONPYCACHEPREFIX", "npm_config_cache", "CARGO_TARGET_DIR"} {
		value := values[key]
		if !strings.HasPrefix(value, tempRoot+string(os.PathSeparator)) {
			t.Fatalf("%s escaped the managed cache root: %s", key, value)
		}
		if info, err := os.Stat(value); err != nil || !info.IsDir() {
			t.Fatalf("%s cache was not created: %v", key, err)
		}
	}
}

func TestMergeEnvironmentReplacesDuplicateKeys(t *testing.T) {
	got := mergeEnvironment([]string{"A=old", "B=keep", "A=older"}, map[string]string{"A": "new"})
	if !reflect.DeepEqual(got, []string{"B=keep", "A=new"}) {
		t.Fatalf("unexpected environment: %v", got)
	}
}
