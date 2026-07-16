package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hellogxp/stateseal/pkg/protocol"
)

func TestLedgerRejectsTampering(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := Open("/repo", "task")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(protocol.Event{Type: "TASK_CREATED", TaskID: "task"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir, "ledger.jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b[10] ^= 1
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(protocol.Event{Type: "CANDIDATE_SUBMITTED", TaskID: "task"}); err == nil {
		t.Fatal("tampered ledger was accepted")
	}
}

func TestReadEventsValidatesAndReturnsTimeline(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := Open("/repo", "timeline")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"TASK_CREATED", "CANDIDATE_SUBMITTED", "COMPLETION_ADMITTED"} {
		if _, err := s.Append(protocol.Event{Type: kind, TaskID: "timeline"}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := s.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[2].Sequence != 3 || events[2].PrevHash != events[1].Hash {
		t.Fatalf("unexpected timeline: %+v", events)
	}
}

func TestLockIsExclusive(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, _ := Open("/repo", "task")
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := s.Lock(); err == nil {
		t.Fatal("second lock unexpectedly succeeded")
	}
}
