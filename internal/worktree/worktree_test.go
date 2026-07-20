package worktree

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCommitCandidatePreservesRepositoryIdentity(t *testing.T) {
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Repository User")
	identity.Git(root, "config", "user.email", "repository@example.com")
	os.WriteFile(filepath.Join(root, "state.txt"), []byte("trusted\n"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "initial")

	m, err := New(root, "identity")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(proposal, "state.txt"), []byte("candidate\n"), 0o644)

	commit, err := m.CommitCandidate(proposal)
	if err != nil {
		t.Fatal(err)
	}
	out, err := identity.Git(proposal, "show", "-s", "--format=%an|%ae|%cn|%ce", commit)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(out)), "Repository User|repository@example.com|Repository User|repository@example.com"; got != want {
		t.Fatalf("checkpoint identity = %q, want %q", got, want)
	}
	assertGitConfig(t, root, "user.name", "Repository User")
	assertGitConfig(t, root, "user.email", "repository@example.com")
}

func TestCommitCandidateHonorsEnvironmentIdentityWithoutPersistingIt(t *testing.T) {
	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	identity.Git(root, "config", "user.name", "Repository User")
	identity.Git(root, "config", "user.email", "repository@example.com")
	os.WriteFile(filepath.Join(root, "state.txt"), []byte("trusted\n"), 0o644)
	identity.Git(root, "add", ".")
	identity.Git(root, "commit", "-m", "initial")

	m, err := New(root, "environment-identity")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(proposal, "state.txt"), []byte("candidate\n"), 0o644)
	t.Setenv("GIT_AUTHOR_NAME", "Automation Author")
	t.Setenv("GIT_AUTHOR_EMAIL", "author@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Automation Committer")
	t.Setenv("GIT_COMMITTER_EMAIL", "committer@example.com")

	commit, err := m.CommitCandidate(proposal)
	if err != nil {
		t.Fatal(err)
	}
	out, err := identity.Git(proposal, "show", "-s", "--format=%an|%ae|%cn|%ce", commit)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(out)), "Automation Author|author@example.com|Automation Committer|committer@example.com"; got != want {
		t.Fatalf("checkpoint identity = %q, want %q", got, want)
	}
	assertGitConfig(t, root, "user.name", "Repository User")
	assertGitConfig(t, root, "user.email", "repository@example.com")
}

func TestCommitCandidateRejectsMissingExplicitIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "missing-gitconfig"))

	root := t.TempDir()
	identity.Git(root, "init", "-b", "main")
	os.WriteFile(filepath.Join(root, "state.txt"), []byte("trusted\n"), 0o644)
	identity.Git(root, "add", ".")
	if _, err := identity.Git(root,
		"-c", "user.name=Bootstrap User",
		"-c", "user.email=bootstrap@example.com",
		"-c", "user.useConfigOnly=true",
		"commit", "-m", "initial",
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")

	m, err := New(root, "missing-identity")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := m.Proposal()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(proposal, "state.txt"), []byte("candidate\n"), 0o644)

	if _, err := m.CommitCandidate(proposal); err == nil || !strings.Contains(err.Error(), "checkpoint commit requires a configured Git identity") {
		t.Fatalf("CommitCandidate() error = %v, want configured identity guidance", err)
	}
	out, err := identity.Git(root, "config", "--local", "--list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "user.name=") || strings.Contains(string(out), "user.email=") {
		t.Fatalf("checkpoint commit persisted a Git identity: %s", out)
	}
}

func assertGitConfig(t *testing.T, root, key, want string) {
	t.Helper()
	out, err := identity.Git(root, "config", "--local", "--get", key)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
