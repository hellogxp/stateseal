package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCodexDesktopIntegrationInstallsMCPWithoutReplacingUserConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	legacyPath := filepath.Join(dir, "hooks.json")
	binary := filepath.Join(dir, "seal")
	if err := os.WriteFile(binary, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5\"\n[projects.\"/work\"]\ntrust_level = \"trusted\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"existing-policy"}]}],"Stop":[{"hooks":[{"type":"command","command":"/old/seal adapter codex desktop-hook Stop"}]}]}}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	spec, err := findIntegrationSpec("codex-desktop")
	if err != nil {
		t.Fatal(err)
	}
	if err := installUserIntegration(spec, configPath, binary, false); err != nil {
		t.Fatal(err)
	}
	if err := validateUserIntegration(spec, configPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	for _, expected := range []string{
		`model = "gpt-5"`, `trust_level = "trusted"`, "[mcp_servers.stateseal]",
		`args = ["mcp", "serve", "--agent", "codex"]`,
		"[mcp_servers.stateseal.tools.enable_project]", "[mcp_servers.stateseal.tools.apply_verified]",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("Codex Desktop MCP config is missing %q:\n%s", expected, content)
		}
	}
	if got := strings.Count(content, stateSealMCPBlockBegin); got != 1 {
		t.Fatalf("StateSeal block count=%d, want 1", got)
	}
	if err := installUserIntegration(spec, configPath, binary, false); err != nil {
		t.Fatalf("idempotent install: %v", err)
	}
	raw, _ = os.ReadFile(configPath)
	if got := strings.Count(string(raw), stateSealMCPBlockBegin); got != 1 {
		t.Fatalf("idempotent install produced %d blocks", got)
	}

	legacyRaw, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(legacyRaw), "desktop-hook") || !strings.Contains(string(legacyRaw), "existing-policy") {
		t.Fatalf("legacy migration did not preserve unrelated hooks: %s", legacyRaw)
	}

	removed, err := removeUserIntegration(spec, configPath)
	if err != nil || !removed {
		t.Fatalf("remove integration: removed=%v err=%v", removed, err)
	}
	raw, _ = os.ReadFile(configPath)
	if strings.Contains(string(raw), "mcp_servers.stateseal") || !strings.Contains(string(raw), `model = "gpt-5"`) {
		t.Fatalf("uninstall did not preserve unrelated config: %s", raw)
	}
}

func TestStateSealMCPServerAdvertisesRoutingAndApprovalBoundaries(t *testing.T) {
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := newStateSealMCPServer("codex").Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-test", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	if instructions := clientSession.InitializeResult().Instructions; !strings.Contains(instructions, "inspect_project") || !strings.Contains(instructions, "Never claim") {
		t.Fatalf("MCP routing instructions are incomplete: %s", instructions)
	}
	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"inspect_project": false, "enable_project": false, "start_delivery": false,
		"get_delivery_status": false, "apply_verified": false, "reject_delivery": false,
	}
	for _, tool := range listed.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
		if tool.Name == "apply_verified" && (tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint) {
			t.Fatalf("apply_verified is not marked destructive: %+v", tool.Annotations)
		}
		if tool.Name == "inspect_project" && (tool.Annotations == nil || !tool.Annotations.ReadOnlyHint) {
			t.Fatalf("inspect_project is not marked read-only: %+v", tool.Annotations)
		}
	}
	for name, present := range want {
		if !present {
			t.Fatalf("MCP server is missing %s", name)
		}
	}
}

func TestMCPProjectEnablementBindsTheDisplayedPolicy(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "config", "user.name", "Test User")
	identity.Git(root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/desktop\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", "go.mod")
	identity.Git(root, "commit", "-m", "chore: initialize fixture")

	inspection, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Enabled || !inspection.ConfirmationRequired || inspection.SetupToken == "" {
		t.Fatalf("unexpected first inspection: %+v", inspection)
	}
	if _, err := enableMCPProject(root, "wrong-token", "codex"); err == nil {
		t.Fatal("enable_project accepted a contract token that was not displayed")
	}
	if _, err := os.Stat(filepath.Join(root, "seal.yaml")); !os.IsNotExist(err) {
		t.Fatalf("rejected enablement changed the project: %v", err)
	}

	enabled, err := enableMCPProject(root, inspection.SetupToken, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled.Enabled || enabled.ConfirmationRequired || enabled.ConfigCommit == "" {
		t.Fatalf("project was not enabled: %+v", enabled)
	}
	settings, err := store.LoadProjectSettings(enabled.RepoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.DesktopEnabled || settings.DesktopSurface != "mcp" || settings.DesktopPolicyDigest != enabled.PolicyDigest || !containsString(settings.DesktopAgents, "codex") {
		t.Fatalf("Desktop policy binding is incomplete: %+v", settings)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("Desktop MCP enablement unexpectedly installed project hooks: %v", err)
	}
	status, _ := identity.Git(root, "status", "--porcelain")
	if strings.TrimSpace(string(status)) != "" {
		t.Fatalf("enablement left a dirty project: %s", status)
	}
	qoderInspection, err := inspectMCPProject(root, "qoder")
	if err != nil {
		t.Fatal(err)
	}
	if qoderInspection.Enabled || qoderInspection.SetupToken == "" || qoderInspection.SetupToken == inspection.SetupToken {
		t.Fatalf("Desktop approval was not isolated by Agent: %+v", qoderInspection)
	}
	if _, err := enableMCPProject(root, qoderInspection.SetupToken, "qoder"); err != nil {
		t.Fatal(err)
	}
	codexInspection, err := inspectMCPProject(root, "codex")
	if err != nil || !codexInspection.Enabled {
		t.Fatalf("enabling a second Desktop Agent invalidated Codex: %+v err=%v", codexInspection, err)
	}

	policyPath := filepath.Join(root, "seal.yaml")
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, append(raw, []byte("\n# policy changed\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if changed.Enabled || !changed.ConfirmationRequired {
		t.Fatalf("changed policy did not require renewed approval: %+v", changed)
	}
}

func TestMCPDeliveryLeavesSourceUntouchedUntilExactReceiptIsApproved(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "config", "user.name", "Test User")
	identity.Git(root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/mcp-delivery\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "baseline.go"), []byte("package delivery\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "chore: initialize fixture")
	inspection, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enableMCPProject(root, inspection.SetupToken, "codex"); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	sealBinary := filepath.Join(binDir, "seal")
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(workingDir, "..", ".."))
	build := exec.Command("go", "build", "-o", sealBinary, "./cmd/seal")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build StateSeal: %v\n%s", err, output)
	}
	fakeCodex := filepath.Join(binDir, "codex")
	script := `#!/bin/sh
set -eu
printf 'package delivery\n\nfunc Message() string { return "verified" }\n' > message.go
printf 'package delivery\n\nimport "testing"\n\nfunc TestMessage(t *testing.T) { if Message() != "verified" { t.Fatal("unexpected message") } }\n' > message_test.go
`
	if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-e2e", Version: "test"}, nil)
	serverCommand := exec.Command(sealBinary, "mcp", "serve", "--agent", "codex")
	serverCommand.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"), "XDG_STATE_HOME="+stateHome)
	clientSession, err := client.Connect(ctx, &mcp.CommandTransport{Command: serverCommand}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	started, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "start_delivery", Arguments: map[string]any{
			"repo_path": root,
			"goal":      "Add a tested Message function that returns verified.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var delivery desktopCommandResult
	decodeMCPStructuredContent(t, started.StructuredContent, &delivery)
	if delivery.Stage != store.DesktopStagePendingApply || delivery.ReceiptID == "" || delivery.Verdict != "ADMITTED" {
		t.Fatalf("unexpected delivery result: %+v", delivery)
	}
	if _, err := os.Stat(filepath.Join(root, "message.go")); !os.IsNotExist(err) {
		t.Fatalf("start_delivery modified the source workspace before approval: %v", err)
	}
	branch, _ := identity.Git(root, "branch", "--show-current")
	if strings.TrimSpace(string(branch)) != "main" {
		t.Fatalf("source branch changed before approval: %s", branch)
	}

	wrong, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "apply_verified", Arguments: map[string]any{
			"session_id": delivery.SessionID, "receipt_id": "rcpt_wrong",
		},
	})
	if err == nil && !wrong.IsError {
		t.Fatalf("apply_verified accepted the wrong receipt: %+v", wrong)
	}
	if _, err := os.Stat(filepath.Join(root, "message.go")); !os.IsNotExist(err) {
		t.Fatalf("wrong receipt modified the source workspace: %v", err)
	}

	applied, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "apply_verified", Arguments: map[string]any{
			"session_id": delivery.SessionID, "receipt_id": delivery.ReceiptID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var result desktopCommandResult
	decodeMCPStructuredContent(t, applied.StructuredContent, &result)
	if result.Stage != store.DesktopStageApplied || result.ReceiptID != delivery.ReceiptID {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "message.go")); err != nil {
		t.Fatalf("verified code was not applied: %v", err)
	}
	branch, _ = identity.Git(root, "branch", "--show-current")
	if !strings.HasPrefix(strings.TrimSpace(string(branch)), "feature/") {
		t.Fatalf("verified checkpoint was not applied to a feature branch: %s", branch)
	}
}

func decodeMCPStructuredContent(t *testing.T, value any, target any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode MCP structured content: %v\n%s", err, raw)
	}
}
