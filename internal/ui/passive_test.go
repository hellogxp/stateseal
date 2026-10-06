package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPassiveProjectionMarksChecksStaleAfterLaterEdit(t *testing.T) {
	snapshot := projectFixture(t, []map[string]any{
		record("2026-07-31T01:00:00Z", "session_meta", map[string]any{"id": "session-1", "cwd": t.TempDir(), "originator": "Codex Desktop"}),
		record("2026-07-31T01:00:01Z", "event_msg", map[string]any{"type": "user_message", "message": "Fix duplicate callbacks"}),
		record("2026-07-31T01:00:02Z", "response_item", map[string]any{"type": "function_call", "name": "exec_command", "call_id": "check-1", "arguments": `{"cmd":"go test ./..."}`}),
		record("2026-07-31T01:00:03Z", "response_item", map[string]any{"type": "function_call_output", "call_id": "check-1", "output": `{"exit_code":0,"output":"ok"}`}),
		record("2026-07-31T01:00:04Z", "response_item", map[string]any{"type": "custom_tool_call", "name": "apply_patch", "call_id": "edit-1", "arguments": `{"patch":"*** Begin Patch\n*** Update File: internal/payments.go\n@@\n-old\n+new\n*** End Patch"}`}),
		record("2026-07-31T01:00:05Z", "response_item", map[string]any{"type": "custom_tool_call_output", "call_id": "edit-1", "output": `{"exit_code":0}`}),
		record("2026-07-31T01:00:06Z", "event_msg", map[string]any{"type": "task_complete"}),
	})

	if snapshot.Summary.EvidencePosture != "STALE" || snapshot.Summary.Status != "NEEDS_ATTENTION" {
		t.Fatalf("expected stale delivery evidence, got %+v", snapshot.Summary)
	}
	if len(snapshot.Checks) != 1 || snapshot.Checks[0].Freshness != "STALE" {
		t.Fatalf("expected one stale check, got %+v", snapshot.Checks)
	}
	if len(snapshot.Changes) != 1 || snapshot.Changes[0].Path != "internal/payments.go" {
		t.Fatalf("patch provenance was not extracted: %+v", snapshot.Changes)
	}
	for _, node := range snapshot.Nodes {
		if node.Kind == "skill" {
			t.Fatalf("delivery graph must not contain Skill attribution: %+v", node)
		}
	}
}

func TestPassiveProjectionRecognizesCurrentEvidence(t *testing.T) {
	snapshot := projectFixture(t, []map[string]any{
		record("2026-07-31T02:00:00Z", "session_meta", map[string]any{"id": "session-2", "cwd": t.TempDir(), "originator": "Codex Desktop"}),
		record("2026-07-31T02:00:01Z", "event_msg", map[string]any{"type": "user_message", "message": "Improve the delivery view"}),
		record("2026-07-31T02:00:02Z", "response_item", map[string]any{"type": "custom_tool_call", "name": "apply_patch", "call_id": "edit-1", "arguments": "*** Begin Patch\n*** Add File: report.md\n+done\n*** End Patch"}),
		record("2026-07-31T02:00:03Z", "response_item", map[string]any{"type": "custom_tool_call_output", "call_id": "edit-1", "output": `{"exit_code":0}`}),
		record("2026-07-31T02:00:04Z", "response_item", map[string]any{"type": "function_call", "name": "exec_command", "call_id": "check-1", "arguments": `{"cmd":"go test ./...","workdir":"/repo"}`}),
		record("2026-07-31T02:00:05Z", "response_item", map[string]any{"type": "function_call_output", "call_id": "check-1", "output": `{"exit_code":0,"output":"ok"}`}),
		record("2026-07-31T02:00:06Z", "event_msg", map[string]any{"type": "task_complete"}),
	})

	if snapshot.Summary.EvidencePosture != "CURRENT" || snapshot.Summary.Status != "AGENT_ENDED" {
		t.Fatalf("expected current evidence, got %+v", snapshot.Summary)
	}
	if len(snapshot.Artifacts) != 1 || snapshot.Artifacts[0].Path != "report.md" {
		t.Fatalf("expected an observed document artifact, got %+v", snapshot.Artifacts)
	}
}

func TestPassiveProjectionSupportsModernCodexToolEnvelope(t *testing.T) {
	patchInput := `const patch = "*** Begin Patch\n*** Add File: output.json\n+{}\n*** End Patch"; text(await tools.apply_patch(patch));`
	checkInput := `const r = await tools.exec_command({cmd:"go test ./...",workdir:"/repo"}); text(r.output)`
	modernOutput := []map[string]any{{"type": "input_text", "text": "Script completed\nWall time 1.2 seconds\nOutput:\nok"}}
	snapshot := projectFixture(t, []map[string]any{
		record("2026-07-31T04:00:00Z", "session_meta", map[string]any{"id": "session-modern", "cwd": t.TempDir()}),
		record("2026-07-31T04:00:01Z", "event_msg", map[string]any{"type": "user_message", "message": "Produce a verified report"}),
		record("2026-07-31T04:00:02Z", "response_item", map[string]any{"type": "custom_tool_call", "name": "exec", "call_id": "edit", "input": patchInput}),
		record("2026-07-31T04:00:03Z", "response_item", map[string]any{"type": "custom_tool_call_output", "call_id": "edit", "output": modernOutput}),
		record("2026-07-31T04:00:04Z", "response_item", map[string]any{"type": "custom_tool_call", "name": "exec", "call_id": "check", "input": checkInput}),
		record("2026-07-31T04:00:05Z", "response_item", map[string]any{"type": "custom_tool_call_output", "call_id": "check", "output": modernOutput}),
		record("2026-07-31T04:00:06Z", "event_msg", map[string]any{"type": "task_complete"}),
	})

	if len(snapshot.Changes) != 1 || snapshot.Changes[0].Path != "output.json" {
		t.Fatalf("modern patch envelope not projected: %+v", snapshot.Changes)
	}
	if len(snapshot.Checks) != 1 || snapshot.Checks[0].Status != "PASSED" || snapshot.Checks[0].WorkingDirectory != "/repo" {
		t.Fatalf("modern command envelope not projected: %+v", snapshot.Checks)
	}
	if snapshot.Summary.EvidencePosture != "CURRENT" {
		t.Fatalf("expected current evidence, got %+v", snapshot.Summary)
	}
}

func TestPassiveProjectionRedactsSecrets(t *testing.T) {
	secret := "github_pat_abcdefghijklmnopqrstuvwxyz123456"
	snapshot := projectFixture(t, []map[string]any{
		record("2026-07-31T03:00:00Z", "session_meta", map[string]any{"id": "session-3", "cwd": t.TempDir()}),
		record("2026-07-31T03:00:01Z", "event_msg", map[string]any{"type": "user_message", "message": "Use " + secret}),
		record("2026-07-31T03:00:02Z", "event_msg", map[string]any{"type": "task_complete"}),
	})
	if snapshot.Summary.Goal == "Use "+secret || snapshot.Summary.Goal != "Use [REDACTED]" {
		t.Fatalf("secret was not redacted: %q", snapshot.Summary.Goal)
	}
}

func projectFixture(t *testing.T, records []map[string]any) RunSnapshot {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	handle, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(handle)
	for _, item := range records {
		if err := encoder.Encode(item); err != nil {
			t.Fatal(err)
		}
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := projectCodexTranscript(passiveSessionFile{Path: path, ModTime: info.ModTime(), Size: info.Size()})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func record(timestamp, kind string, payload map[string]any) map[string]any {
	at, _ := time.Parse(time.RFC3339, timestamp)
	return map[string]any{"timestamp": at, "type": kind, "payload": payload}
}
