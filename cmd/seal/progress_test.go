package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

func TestRunProgressShowsPlanAndHeartbeat(t *testing.T) {
	original := progressHeartbeatInterval
	progressHeartbeatInterval = 5 * time.Millisecond
	t.Cleanup(func() { progressHeartbeatInterval = original })

	var out bytes.Buffer
	progress := newRunProgress(&out, i18n.SimplifiedChinese, true, true)
	policy := config.Default("progress", []config.Check{{ID: "test", Command: []string{"go", "test", "./..."}}})
	progress.Header("修复输入校验", "Codex", policy)
	progress.AgentStarted("Codex", t.TempDir(), "HEAD")
	time.Sleep(15 * time.Millisecond)
	progress.StopAgentHeartbeat()

	for _, want := range []string{"执行计划", "go test ./...", "Codex 已启动", "Codex 正在分析项目", "尚未产生代码变更"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("progress output missing %q:\n%s", want, out.String())
		}
	}
}

func TestMachineAndQuietResultsStayCompact(t *testing.T) {
	receipt := protocol.CompletionReceipt{
		Verdict: protocol.VerdictAdmitted, ReceiptID: "rcpt_test",
		CompletionEvidence: []string{"ev_1", "ev_2"},
		VerificationCoverage: &protocol.VerificationCoverage{
			Observation: "terminal-only",
			Verifiers:   []protocol.VerifierCoverage{{CheckID: "tests", Phase: "completion", Layer: "L1", Origin: "auto-discovered", Status: "passed"}},
		},
		LivenessImpact: &protocol.LivenessImpact{CandidatesEvaluated: 2, CheckpointsVerified: 1},
	}
	state := protocol.TaskState{TaskID: "task", Coverage: "terminal-only", AppliedBranch: "feature/task", AppliedCommit: "abc"}
	snapshot := progressSnapshot{Attempts: 2, StartedAt: time.Unix(0, 0), CompletedAt: time.Unix(2, 0)}

	var machine bytes.Buffer
	if err := printJSONRunResult(&machine, receipt, state, snapshot); err != nil {
		t.Fatal(err)
	}
	var result jsonRunResult
	if err := json.Unmarshal(machine.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON result: %v\n%s", err, machine.String())
	}
	if result.Verdict != protocol.VerdictAdmitted || result.Attempts != 2 || !result.Applied || result.Branch != "feature/task" {
		t.Fatalf("unexpected JSON result: %+v", result)
	}
	if result.VerificationCoverage == nil || result.LivenessImpact == nil {
		t.Fatalf("machine result omitted coverage or delivery impact: %+v", result)
	}

	var quiet bytes.Buffer
	printQuietRunResult(&quiet, receipt, state, i18n.English)
	if lines := strings.Count(strings.TrimSpace(quiet.String()), "\n") + 1; lines != 1 {
		t.Fatalf("quiet result used %d lines: %q", lines, quiet.String())
	}
	if !strings.Contains(quiet.String(), "Ready to deliver") || !strings.Contains(quiet.String(), "feature/task") {
		t.Fatalf("quiet result omitted delivery state: %q", quiet.String())
	}
}
