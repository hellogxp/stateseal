package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/store"
)

func TestCodexDesktopIntegrationInstallsOfficialBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"existing-policy"}]}],"PostToolUse":[{"hooks":[{"type":"command","command":"/old/seal adapter codex hook PostToolUse"}]}],"Stop":[{"hooks":[{"type":"command","command":"/old/seal adapter codex hook Stop"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := findIntegrationSpec("codex-desktop")
	if err != nil {
		t.Fatal(err)
	}
	if err := installUserIntegration(spec, path, "/usr/local/bin/seal", false); err != nil {
		t.Fatal(err)
	}
	if err := validateUserIntegration(spec, path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		if !strings.Contains(string(raw), `"`+event+`"`) {
			t.Fatalf("Codex Desktop integration is missing %s: %s", event, raw)
		}
	}
	if got := strings.Count(string(raw), "adapter codex desktop-hook"); got != 5 {
		t.Fatalf("Codex Desktop integration installed %d owned handlers, want 5: %s", got, raw)
	}
	if !strings.Contains(string(raw), "existing-policy") {
		t.Fatalf("Codex Desktop integration removed an existing hook: %s", raw)
	}
	if strings.Contains(string(raw), "/old/seal adapter codex hook") {
		t.Fatalf("Codex Desktop integration retained a legacy duplicate handler: %s", raw)
	}
	removed, err := removeUserIntegration(spec, path)
	if err != nil || !removed {
		t.Fatalf("remove Codex Desktop integration: removed=%v err=%v", removed, err)
	}
	raw, _ = os.ReadFile(path)
	if strings.Contains(string(raw), "adapter codex desktop-hook") || !strings.Contains(string(raw), "existing-policy") {
		t.Fatalf("Codex Desktop uninstall did not preserve unrelated hooks: %s", raw)
	}
}

func TestCodexDesktopHookBindsGoalAndEnforcesDelegation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := identity.Git(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/desktop\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", "go.mod")
	identity.Git(root, "-c", "user.name=Test User", "-c", "user.email=test@example.com", "commit", "-m", "chore: initialize fixture")

	runHook := func(event string, payload map[string]any) map[string]any {
		t.Helper()
		payload["hook_event_name"] = event
		payload["session_id"] = "desktop-session-1"
		payload["cwd"] = root
		raw, _ := json.Marshal(payload)
		cmd := newRoot()
		var out bytes.Buffer
		cmd.SetIn(bytes.NewReader(raw))
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"adapter", "codex", "desktop-hook", event})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s hook: %v\n%s", event, err, out.String())
		}
		if out.Len() == 0 {
			return nil
		}
		var response map[string]any
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatalf("decode %s hook response: %v\n%s", event, err, out.String())
		}
		return response
	}

	response := runHook("UserPromptSubmit", map[string]any{
		"turn_id": "turn-1", "prompt": "StateSeal: 新增安全的规范化函数",
	})
	context := hookAdditionalContext(response)
	if !strings.Contains(context, "first-project confirmation") || !strings.Contains(context, "go test ./...") {
		t.Fatalf("first Desktop prompt did not present the verification contract: %v", response)
	}
	session, err := store.LoadDesktopSession("desktop-session-1")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, _ := identity.GitRoot(root)
	if session.Stage != store.DesktopStageSetupPending || session.Goal != "新增安全的规范化函数" || session.RepoRoot != canonicalRoot {
		t.Fatalf("unexpected Desktop authority state: %+v", session)
	}

	response = runHook("UserPromptSubmit", map[string]any{"turn_id": "turn-2", "prompt": "确认"})
	if context = hookAdditionalContext(response); !strings.Contains(context, "desktop run --session") {
		t.Fatalf("confirmed setup did not authorize the controlled run: %v", response)
	}
	session, _ = store.LoadDesktopSession("desktop-session-1")
	if session.Stage != store.DesktopStageReady {
		t.Fatalf("confirmed Desktop session is %s, want READY", session.Stage)
	}

	response = runHook("PreToolUse", map[string]any{
		"turn_id": "turn-2", "tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch"},
	})
	hookOutput, _ := response["hookSpecificOutput"].(map[string]any)
	if hookOutput["permissionDecision"] != "deny" {
		t.Fatalf("direct edit was not denied during a managed Desktop turn: %v", response)
	}

	response = runHook("PreToolUse", map[string]any{
		"turn_id": "turn-2", "tool_name": "Bash", "tool_input": map[string]any{"command": desktopRunCommand(session)},
	})
	if response != nil {
		t.Fatalf("exact StateSeal Desktop command was not allowed: %v", response)
	}

	response = runHook("Stop", map[string]any{"turn_id": "turn-2", "stop_hook_active": false})
	if response["decision"] != "block" || !strings.Contains(response["reason"].(string), "desktop run --session") {
		t.Fatalf("Stop did not continue an unstarted controlled delivery: %v", response)
	}
}

func TestDesktopGoalRequiresExplicitMarker(t *testing.T) {
	for _, tc := range []struct {
		prompt string
		goal   string
		ok     bool
	}{
		{"StateSeal: 修复并测试", "修复并测试", true},
		{"stateseal：实现功能", "实现功能", true},
		{"/seal add validation", "add validation", true},
		{"StateSeal 是什么？", "", false},
		{"普通开发目标", "", false},
	} {
		goal, ok := desktopGoal(tc.prompt)
		if goal != tc.goal || ok != tc.ok {
			t.Fatalf("desktopGoal(%q)=(%q,%v), want (%q,%v)", tc.prompt, goal, ok, tc.goal, tc.ok)
		}
	}
}

func hookAdditionalContext(response map[string]any) string {
	output, _ := response["hookSpecificOutput"].(map[string]any)
	context, _ := output["additionalContext"].(string)
	return context
}
