package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hellogxp/stateseal/internal/identity"
)

func TestProposalIsScopedToTrustedBase(t *testing.T) {
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Test")
	identity.Git(root, "config", "user.email", "test@example.com")
	os.WriteFile(filepath.Join(root, "state.txt"), []byte("one"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "first")
	first, err := New(root, "task")
	if err != nil {
		t.Fatal(err)
	}
	firstPath, err := first.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "state.txt"), []byte("two"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "second")
	second, err := New(root, "task")
	if err != nil {
		t.Fatal(err)
	}
	secondPath, err := second.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	if firstPath == secondPath {
		t.Fatal("proposal path was reused across trusted bases")
	}
}

func TestProposalLinksIgnoredNodeModules(t *testing.T) {
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Test")
	identity.Git(root, "config", "user.email", "test@example.com")
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "node_modules", ".bin"), 0o755)
	os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "initial")
	m, err := New(root, "dependencies")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(proposal, "node_modules"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("node_modules was not linked")
	}
	tree, err := identity.Tree(proposal, []string{"**"})
	if err != nil || tree == "" {
		t.Fatalf("managed dependency changed tree enumeration: %v", err)
	}
	unsafe, err := identity.UnsafeSymlinks(proposal)
	if err != nil || len(unsafe) != 0 {
		t.Fatalf("managed dependency entered symlink policy: %v %v", unsafe, err)
	}
}

func TestRestoreRemovesTrackedAndUntrackedCandidateArtifacts(t *testing.T) {
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Test")
	identity.Git(root, "config", "user.email", "test@example.com")
	os.WriteFile(filepath.Join(root, "state.txt"), []byte("trusted\n"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "initial")
	m, err := New(root, "restore")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(proposal, "state.txt"), []byte("regressed\n"), 0o644)
	os.WriteFile(filepath.Join(proposal, "untracked.txt"), []byte("residue\n"), 0o644)
	if err := m.Restore(proposal, m.Base); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(proposal, "state.txt"))
	if err != nil || string(contents) != "trusted\n" {
		t.Fatalf("tracked state was not restored: %q %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(proposal, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked candidate artifact survived restore: %v", err)
	}
}
