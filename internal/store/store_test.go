package store

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestProjectSettingsRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	want := ProjectSettings{
		Agent: "codex", TrustedHookAutomation: true, DesktopEnabled: true,
		DesktopPolicyDigest: "policy-digest", DesktopSurface: "mcp", DesktopAgents: []string{"codex", "qoder"},
	}
	if err := SaveProjectSettings("/repo/project", want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProjectSettings("/repo/project")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("project settings mismatch: got %+v want %+v", got, want)
	}
}

func TestOpenRejectsTaskIDPathTraversal(t *testing.T) {
	if _, err := Open(t.TempDir(), "../../escape"); err == nil {
		t.Fatal("unsafe task ID was accepted")
	}
}

func TestActiveTaskPointerIsExternalAndValidated(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	if err := SetActiveTask(repo, "fix-payment-race"); err != nil {
		t.Fatal(err)
	}
	got, err := ActiveTask(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got != "fix-payment-race" {
		t.Fatalf("active task = %q", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "active-task.json")); !os.IsNotExist(err) {
		t.Fatalf("authority pointer leaked into repository: %v", err)
	}
	if err := SetActiveTask(repo, "../../escape"); err == nil {
		t.Fatal("unsafe active task was accepted")
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

func TestListTasksAcrossRepositoriesAndReportsLedgerIntegrity(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	first, err := Open("/repo/alpha", "task-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Save(protocol.TaskState{TaskID: "task-a", RepoRoot: "/repo/alpha", Status: "WORKING"}); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Append(protocol.Event{Type: "TASK_CREATED", TaskID: "task-a"}); err != nil {
		t.Fatal(err)
	}
	second, err := Open("/repo/beta", "task-b")
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Save(protocol.TaskState{TaskID: "task-b", RepoRoot: "/repo/beta", Status: "ADMITTED"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second.Dir, "ledger.jsonl"), []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tasks, err := ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2: %+v", len(tasks), tasks)
	}
	byID := map[string]StoredTask{}
	for _, task := range tasks {
		byID[task.State.TaskID] = task
	}
	if len(byID["task-a"].Events) != 1 || byID["task-a"].IntegrityError != "" {
		t.Fatalf("valid task was not loaded: %+v", byID["task-a"])
	}
	if byID["task-b"].IntegrityError == "" {
		t.Fatalf("tampered task did not report integrity failure: %+v", byID["task-b"])
	}
}
