package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

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
