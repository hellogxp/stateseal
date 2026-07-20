package broker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/worktree"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

func TestVerifyRejectsVerifyThenEdit(t *testing.T) {
	root := testRepo(t)
	p := config.Default("stale", []config.Check{{ID: "baseline", Command: []string{"git", "diff", "--check"}}})
	if err := config.Write(filepath.Join(root, "seal.yaml"), p); err != nil {
		t.Fatal(err)
	}
	b, err := New(root, "enforce")
	if err != nil {
		t.Fatal(err)
	}
	checks := []config.Check{{ID: "mutator", Command: []string{"sh", "-c", "printf changed > app.txt"}, TimeoutSeconds: 10}}
	r, err := b.VerifyCurrent(checks)
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != protocol.VerdictStale {
		t.Fatalf("got %s, want STALE", r.Verdict)
	}
}

func TestFailedCheckDoesNotProduceCheckpoint(t *testing.T) {
	root := testRepo(t)
	p := config.Default("reject", []config.Check{{ID: "baseline", Command: []string{"git", "diff", "--check"}}})
	if err := config.Write(filepath.Join(root, "seal.yaml"), p); err != nil {
		t.Fatal(err)
	}
	b, _ := New(root, "enforce")
	r, err := b.VerifyCurrent([]config.Check{{ID: "fail", Command: []string{"sh", "-c", "echo 'expected value mismatch'; exit 7"}, TimeoutSeconds: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != protocol.VerdictRejected || b.State.Checkpoint != nil {
		t.Fatalf("rejected state advanced checkpoint")
	}
	if r.RuleID != protocol.RuleVerifierFailed || r.Disposition != "BLOCKED" {
		t.Fatalf("rejection was not classified: %+v", r)
	}
	if !strings.Contains(r.Reason, "expected value mismatch") || !strings.Contains(r.Reason, "command/fail@v1") {
		t.Fatalf("rejection did not provide actionable verifier feedback: %q", r.Reason)
	}
}

func TestWarnModeRecordsOverride(t *testing.T) {
	root := testRepo(t)
	p := config.Default("warn", []config.Check{{ID: "baseline", Command: []string{"git", "diff", "--check"}}})
	if err := config.Write(filepath.Join(root, "seal.yaml"), p); err != nil {
		t.Fatal(err)
	}
	b, _ := New(root, "warn")
	r, err := b.VerifyCurrent([]config.Check{{ID: "fail", Command: []string{"sh", "-c", "exit 1"}, TimeoutSeconds: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Disposition != "OVERRIDDEN" || r.EnforcementMode != "warn" || b.State.Disposition != "OVERRIDDEN" {
		t.Fatalf("warn override was not recorded: %+v", r)
	}
	events, err := b.Store.ReadEvents()
	if err != nil || len(events) == 0 || events[len(events)-1].Type != "MODE_DECISION" {
		t.Fatalf("missing mode decision: events=%+v err=%v", events, err)
	}
}

func TestReceiptTamperIsDetected(t *testing.T) {
	r := protocol.CompletionReceipt{ReceiptVersion: protocol.Version, ReceiptID: "r", TaskID: "t", Verdict: protocol.VerdictAdmitted, ResidualRisks: []string{"risk"}}
	r.ReceiptDigest = ""
	r.ReceiptDigest, _ = identity.JSONDigest(r)
	path := filepath.Join(t.TempDir(), "receipt.json")
	b, _ := json.Marshal(r)
	os.WriteFile(path, b, 0o600)
	if _, err := InspectReceipt(path); err != nil {
		t.Fatal(err)
	}
	r.Verdict = protocol.VerdictRejected
	b, _ = json.Marshal(r)
	os.WriteFile(path, b, 0o600)
	if _, err := InspectReceipt(path); err == nil {
		t.Fatal("tampered receipt accepted")
	}
}

func TestManagedAdmissionRejectsConcurrentBaseChange(t *testing.T) {
	root := testRepo(t)
	checks := []config.Check{{ID: "pass", Command: []string{"sh", "-c", "true"}, TimeoutSeconds: 10}}
	p := config.Default("concurrent-base", checks)
	if err := config.Write(filepath.Join(root, "seal.yaml"), p); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", "seal.yaml")
	identity.Git(root, "commit", "-m", "policy")
	b, err := New(root, "enforce")
	if err != nil {
		t.Fatal(err)
	}
	m, err := worktree.New(root, p.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(proposal, "app.txt"), []byte("candidate"), 0o644)
	os.WriteFile(filepath.Join(root, "base.txt"), []byte("advanced"), 0o644)
	identity.Git(root, "add", "base.txt")
	identity.Git(root, "commit", "-m", "advance base")
	receipt, err := b.AdmitManaged(m, proposal, "test")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Verdict != protocol.VerdictStale || b.State.Checkpoint != nil {
		t.Fatalf("concurrent base change was not rejected: %+v", receipt)
	}
}

func TestTerminalRegressionRecertifiesPreviousCheckpoint(t *testing.T) {
	root := testRepo(t)
	checks := []config.Check{{ID: "good", Command: []string{"sh", "-c", "grep -qx good app.txt"}, TimeoutSeconds: 10}}
	p := config.Default("recover-terminal", checks)
	if err := config.Write(filepath.Join(root, "seal.yaml"), p); err != nil {
		t.Fatal(err)
	}
	identity.Git(root, "add", "seal.yaml")
	identity.Git(root, "commit", "-m", "policy")
	b, err := New(root, "enforce")
	if err != nil {
		t.Fatal(err)
	}
	m, err := worktree.New(root, p.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proposal, "app.txt"), []byte("good\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := b.AdmitIntermediate(m, proposal, "test-intermediate")
	if err != nil || first.Verdict != protocol.VerdictAdmitted || b.State.Checkpoint == nil {
		t.Fatalf("intermediate checkpoint was not admitted: receipt=%+v err=%v", first, err)
	}
	checkpointID := b.State.Checkpoint.CheckpointID
	checkpointCandidate := b.State.Checkpoint.CandidateID
	if err := os.WriteFile(filepath.Join(proposal, "app.txt"), []byte("bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recovered, err := b.AdmitManaged(m, proposal, "test-terminal")
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Verdict != protocol.VerdictAdmitted || !recovered.Recovered {
		t.Fatalf("terminal regression was not recovered: %+v", recovered)
	}
	if recovered.CheckpointID != checkpointID || b.State.Checkpoint.CandidateID != checkpointCandidate {
		t.Fatalf("recovery selected the wrong checkpoint: %+v", recovered)
	}
	if recovered.TerminalCandidate == "" || recovered.TerminalCandidate == checkpointCandidate {
		t.Fatalf("terminal candidate was not recorded: %+v", recovered)
	}
	if recovered.SelectionReason != "terminal_candidate_regressed" || len(recovered.CompletionEvidence) != 1 {
		t.Fatalf("recovery provenance is incomplete: %+v", recovered)
	}
	if recovered.RuleID != protocol.RuleTerminalRecovered {
		t.Fatalf("recovery rule is missing: %+v", recovered)
	}
}

func testRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Test")
	identity.Git(root, "config", "user.email", "test@example.com")
	os.WriteFile(filepath.Join(root, "app.txt"), []byte("original"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "initial")
	return root
}
