package broker

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	r, err := b.VerifyCurrent([]config.Check{{ID: "fail", Command: []string{"sh", "-c", "exit 7"}, TimeoutSeconds: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != protocol.VerdictRejected || b.State.Checkpoint != nil {
		t.Fatalf("rejected state advanced checkpoint")
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
