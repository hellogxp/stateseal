package store

import (
	"path/filepath"
	"testing"
)

func TestDesktopSessionAuthorityStateRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	want := DesktopSession{
		SessionID: "session/a", Agent: "codex", RepoRoot: filepath.Join(t.TempDir(), "repo"),
		Goal: "implement validation", TaskID: "validation-1", Stage: DesktopStageReady,
	}
	if err := SaveDesktopSession(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDesktopSession(want.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != want.SessionID || got.RepoRoot != want.RepoRoot || got.Stage != want.Stage || got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("unexpected Desktop session round trip: %+v", got)
	}
}

func TestDesktopRepositoryLockRejectsConcurrentSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	unlock, err := LockDesktopRepository(root, "one")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := LockDesktopRepository(root, "two"); err == nil {
		t.Fatal("concurrent Desktop repository session was accepted")
	}
}
