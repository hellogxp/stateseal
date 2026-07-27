package main

import (
	"bytes"
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

func TestProjectExclusionDisablesAutomaticMCPUntilExplicitInclude(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	root, err := identity.GitRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/excluded\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"integrate", "exclude", root, "--reason", "StateSeal self-development"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Explicit `seal run` commands remain available") {
		t.Fatalf("exclude output did not explain its boundary: %s", out.String())
	}
	settings, err := store.LoadProjectSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.IntegrationExcluded || settings.IntegrationExclusionReason != "StateSeal self-development" ||
		settings.IntegrationExcludedAt == "" || settings.DesktopEnabled || settings.TrustedHookAutomation {
		t.Fatalf("project exclusion was incomplete: %+v", settings)
	}
	inspection, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Excluded || inspection.Enabled || inspection.ConfirmationRequired ||
		inspection.SetupToken != "" || !strings.Contains(inspection.NextAction, "do not call enable_project") {
		t.Fatalf("MCP inspection ignored project exclusion: %+v", inspection)
	}
	if _, err := enableMCPProject(root, "", "codex"); err == nil || !strings.Contains(err.Error(), "project is excluded") {
		t.Fatalf("excluded project could be enabled without include: %v", err)
	}

	list := newRoot()
	out.Reset()
	list.SetOut(&out)
	list.SetArgs([]string{"integrate", "exclusions"})
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), root) || !strings.Contains(out.String(), "StateSeal self-development") {
		t.Fatalf("excluded project was not listed: %s", out.String())
	}

	include := newRoot()
	out.Reset()
	include.SetOut(&out)
	include.SetArgs([]string{"integrate", "include", root})
	if err := include.Execute(); err != nil {
		t.Fatal(err)
	}
	included, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if included.Excluded || included.Enabled || !included.ConfirmationRequired || included.SetupToken == "" {
		t.Fatalf("include did not restore fresh approval flow: %+v", included)
	}
}

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
		`args = ["mcp", "serve", "--agent", "codex", "--confirmation", "host-tool"]`,
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

	if instructions := clientSession.InitializeResult().Instructions; !strings.Contains(instructions, "inspect_project") || !strings.Contains(instructions, "UNVERIFIED") || !strings.Contains(instructions, "read-only") || !strings.Contains(instructions, "must not be bypassed") {
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

func TestManagedDesktopChildCannotRecursivelyInvokeStateSeal(t *testing.T) {
	t.Setenv("STATESEAL_DESKTOP_MCP_CHILD", "1")
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := newStateSealMCPServer("codex").Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-child-test", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 0 {
		t.Fatalf("managed child inherited recursive StateSeal tools: %+v", listed.Tools)
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
	hooks, err := os.ReadFile(filepath.Join(root, ".codex", "hooks.json"))
	if err != nil || !strings.Contains(string(hooks), adapterMarker("codex")) {
		t.Fatalf("Desktop MCP enablement did not install protected project hooks: err=%v hooks=%s", err, hooks)
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

func TestMCPProjectEnablementRequiresNativeUserConfirmation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "config", "user.name", "Test User")
	identity.Git(root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/native-confirmation\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", "go.mod")
	identity.Git(root, "commit", "-m", "chore: initialize fixture")

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := newStateSealMCPServer("codex").Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	accept := false
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-confirm-test", Version: "test"}, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			if accept {
				return &mcp.ElicitResult{Action: "accept"}, nil
			}
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	})
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	inspection, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}
	declined, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "enable_project", Arguments: map[string]any{
		"repo_path": root, "setup_token": inspection.SetupToken,
	}})
	if err != nil || declined.IsError {
		t.Fatalf("enable_project did not return a recoverable decline: err=%v result=%+v", err, declined)
	}
	var declinedInspection mcpProjectInspection
	decodeMCPStructuredContent(t, declined.StructuredContent, &declinedInspection)
	if declinedInspection.ReasonCode != "USER_DECLINED_CONFIRMATION" || !declinedInspection.Unverified ||
		declinedInspection.AuthorityStatus != "degraded" {
		t.Fatalf("unexpected recoverable decline: %+v", declinedInspection)
	}
	if _, err := os.Stat(filepath.Join(root, "seal.yaml")); !os.IsNotExist(err) {
		t.Fatalf("declined enablement changed the project: %v", err)
	}
	accept = true
	enabled, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "enable_project", Arguments: map[string]any{
		"repo_path": root, "setup_token": inspection.SetupToken,
	}})
	if err != nil || enabled.IsError {
		t.Fatalf("confirmed project enablement failed: err=%v result=%+v", err, enabled)
	}
	if _, err := os.Stat(filepath.Join(root, "seal.yaml")); err != nil {
		t.Fatalf("confirmed enablement did not create policy: %v", err)
	}
}

func TestMCPProjectEnablementWithoutElicitationFailsOpen(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := initializeMCPTestRepo(t, "no-elicitation")
	inspection, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := newStateSealMCPServer("codex").Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-no-elicitation", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	missing, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "inspect_project", Arguments: map[string]any{
		"repo_path": filepath.Join(root, "missing-workspace"),
	}})
	if err != nil || missing.IsError {
		t.Fatalf("workspace inspection failure blocked the Agent: err=%v result=%+v", err, missing)
	}
	var missingInspection mcpProjectInspection
	decodeMCPStructuredContent(t, missing.StructuredContent, &missingInspection)
	if missingInspection.ReasonCode != "PROJECT_INSPECTION_FAILED" || !missingInspection.Unverified {
		t.Fatalf("unexpected inspection degradation: %+v", missingInspection)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "enable_project", Arguments: map[string]any{
		"repo_path": root, "setup_token": inspection.SetupToken,
	}})
	if err != nil || result.IsError {
		t.Fatalf("missing elicitation became a tool failure: err=%v result=%+v", err, result)
	}
	var degraded mcpProjectInspection
	decodeMCPStructuredContent(t, result.StructuredContent, &degraded)
	if degraded.ReasonCode != "CLIENT_CONFIRMATION_UNAVAILABLE" || degraded.AuthorityStatus != "degraded" ||
		!degraded.Unverified || !strings.Contains(degraded.NextAction, "normal development workflow") {
		t.Fatalf("unexpected fail-open result: %+v", degraded)
	}
	if _, err := os.Stat(filepath.Join(root, "seal.yaml")); !os.IsNotExist(err) {
		t.Fatalf("fail-open enablement changed project state: %v", err)
	}
}

func TestMCPHostToolApprovalDoesNotRequireSecondElicitation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := initializeMCPTestRepo(t, "host-tool-approval")
	inspection, err := inspectMCPProject(root, "codex")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := newStateSealMCPServerWithConfirmation("codex", mcpConfirmationHostTool).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-host-tool", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "enable_project", Arguments: map[string]any{
		"repo_path": root, "setup_token": inspection.SetupToken,
	}})
	if err != nil || result.IsError {
		t.Fatalf("host-approved tool requested a second confirmation: err=%v result=%+v", err, result)
	}
	var enabled mcpProjectInspection
	decodeMCPStructuredContent(t, result.StructuredContent, &enabled)
	if !enabled.Enabled || enabled.AuthorityStatus != "healthy" {
		t.Fatalf("host-approved enablement failed: %+v", enabled)
	}
}

func TestMCPWorkspaceRequiresExplicitRepositorySelection(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"api", "web"} {
		initializeMCPTestRepoAt(t, filepath.Join(root, name), name)
	}
	inspection, err := inspectMCPWorkspace(root, "", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if inspection.ReasonCode != "REPOSITORY_SELECTION_REQUIRED" || len(inspection.Repositories) != 2 ||
		inspection.AuthorityStatus != "needs_user_action" {
		t.Fatalf("workspace did not require explicit selection: %+v", inspection)
	}
	selected, err := inspectMCPWorkspace(root, "web", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Project != "web" || selected.RepoRoot == "" || selected.AuthorityStatus != "needs_enablement" {
		t.Fatalf("workspace selection failed: %+v", selected)
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
	acceptApply := false
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-e2e", Version: "test"}, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			if acceptApply {
				return &mcp.ElicitResult{Action: "accept"}, nil
			}
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	})
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
	if delivery.ChangedFiles != 2 || len(delivery.Files) != 2 || len(delivery.Checks) == 0 || delivery.CodeState == "" || delivery.DurationMS <= 0 || len(delivery.Timeline) == 0 {
		t.Fatalf("delivery omitted professional progress or evidence fields: %+v", delivery)
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

	declined, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "apply_verified", Arguments: map[string]any{
			"session_id": delivery.SessionID, "receipt_id": delivery.ReceiptID,
		},
	})
	if err != nil || declined.IsError {
		t.Fatalf("apply_verified did not preserve a recoverable pending state: err=%v result=%+v", err, declined)
	}
	var declinedApply desktopCommandResult
	decodeMCPStructuredContent(t, declined.StructuredContent, &declinedApply)
	if declinedApply.Stage != store.DesktopStagePendingApply || declinedApply.ReasonCode != "USER_DECLINED_CONFIRMATION" ||
		declinedApply.SafeState == nil || !declinedApply.SafeState.CandidatePreserved {
		t.Fatalf("unexpected declined apply state: %+v", declinedApply)
	}
	if _, err := os.Stat(filepath.Join(root, "message.go")); !os.IsNotExist(err) {
		t.Fatalf("declined apply modified the source workspace: %v", err)
	}

	acceptApply = true
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

func initializeMCPTestRepo(t *testing.T, module string) string {
	t.Helper()
	return initializeMCPTestRepoAt(t, t.TempDir(), module)
}

func initializeMCPTestRepoAt(t *testing.T, root, module string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root, "config", "user.name", "Test User"); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root, "config", "user.email", "test@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/"+module+"\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root, "add", "go.mod"); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Git(root, "commit", "-m", "chore: initialize fixture"); err != nil {
		t.Fatal(err)
	}
	resolved, err := identity.GitRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
