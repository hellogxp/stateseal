package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

type Store struct{ Dir string }

// ProjectSettings are local user preferences. They live outside the repository
// so a coding Agent cannot silently change which integration StateSeal trusts.
type ProjectSettings struct {
	Agent                 string `json:"agent,omitempty"`
	TrustedHookAutomation bool   `json:"trusted_hook_automation,omitempty"`
}

func Open(repoRoot, taskID string) (*Store, error) {
	if err := identity.ValidateTaskID(taskID); err != nil {
		return nil, fmt.Errorf("unsafe authority-state task ID: %w", err)
	}
	repoDir, err := repositoryDir(repoRoot)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(repoDir, taskID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

func repositoryDir(repoRoot string) (string, error) {
	repoID := identity.Digest([]byte(repoRoot))[:20]
	base, err := stateHome()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "stateseal", repoID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// SetActiveTask records the latest task outside the repository so an Agent
// cannot redirect status, apply, or receipt inspection by editing project files.
func SetActiveTask(repoRoot, taskID string) error {
	if err := identity.ValidateTaskID(taskID); err != nil {
		return fmt.Errorf("unsafe active task ID: %w", err)
	}
	dir, err := repositoryDir(repoRoot)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]string{"task_id": taskID})
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "active-task.json.tmp")
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "active-task.json"))
}

func ActiveTask(repoRoot string) (string, error) {
	dir, err := repositoryDir(repoRoot)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "active-task.json"))
	if err != nil {
		return "", err
	}
	var active struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(raw, &active); err != nil {
		return "", fmt.Errorf("parse active task: %w", err)
	}
	if err := identity.ValidateTaskID(active.TaskID); err != nil {
		return "", fmt.Errorf("active task: %w", err)
	}
	return active.TaskID, nil
}

func SaveProjectSettings(repoRoot string, settings ProjectSettings) error {
	dir, err := repositoryDir(repoRoot)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "project-settings.json.tmp")
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "project-settings.json"))
}

func LoadProjectSettings(repoRoot string) (ProjectSettings, error) {
	dir, err := repositoryDir(repoRoot)
	if err != nil {
		return ProjectSettings{}, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "project-settings.json"))
	if err != nil {
		return ProjectSettings{}, err
	}
	var settings ProjectSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return ProjectSettings{}, fmt.Errorf("parse project settings: %w", err)
	}
	return settings, nil
}

func stateHome() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return x, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(h, "Library", "Application Support"), nil
	}
	return filepath.Join(h, ".local", "state"), nil
}

func (s *Store) Save(state protocol.TaskState) error {
	state.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.Dir, "state.json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.Dir, "state.json"))
}

func (s *Store) Load() (protocol.TaskState, error) {
	b, err := os.ReadFile(filepath.Join(s.Dir, "state.json"))
	if err != nil {
		return protocol.TaskState{}, err
	}
	var state protocol.TaskState
	if err := json.Unmarshal(b, &state); err != nil {
		return state, err
	}
	return state, nil
}

func (s *Store) Append(event protocol.Event) (protocol.Event, error) {
	path := filepath.Join(s.Dir, "ledger.jsonl")
	seq, prev, err := ledgerTail(path)
	if err != nil {
		return event, err
	}
	event.Sequence, event.PrevHash, event.Timestamp = seq+1, prev, time.Now().UTC()
	event.Hash = ""
	h, err := identity.JSONDigest(event)
	if err != nil {
		return event, err
	}
	event.Hash = h
	b, err := json.Marshal(event)
	if err != nil {
		return event, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return event, err
	}
	defer f.Close()
	if _, err = f.Write(append(b, '\n')); err != nil {
		return event, err
	}
	return event, f.Sync()
}

func ledgerTail(path string) (uint64, string, error) {
	events, err := readEvents(path)
	if err != nil {
		return 0, "", err
	}
	if len(events) == 0 {
		return 0, "", nil
	}
	last := events[len(events)-1]
	return last.Sequence, last.Hash, nil
}

// ReadEvents returns the complete ledger after validating sequence and hash
// continuity. Consumers never receive a partially trusted timeline.
func (s *Store) ReadEvents() ([]protocol.Event, error) {
	return readEvents(filepath.Join(s.Dir, "ledger.jsonl"))
}

func readEvents(path string) ([]protocol.Event, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var last protocol.Event
	var events []protocol.Event
	s := bufio.NewScanner(f)
	for s.Scan() {
		var e protocol.Event
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("malformed ledger: %w", err)
		}
		copyE := e
		copyE.Hash = ""
		h, _ := identity.JSONDigest(copyE)
		if h != e.Hash || e.Sequence != uint64(len(events)+1) || (last.Hash == "" && e.PrevHash != "") || (last.Hash != "" && e.PrevHash != last.Hash) {
			return nil, fmt.Errorf("ledger integrity check failed at sequence %d", e.Sequence)
		}
		last = e
		events = append(events, e)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) ExportReceipt(receipt protocol.CompletionReceipt) (string, error) {
	dir := filepath.Join(s.Dir, "receipts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, receipt.ReceiptID+".json")
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o600)
}

func (s *Store) Lock() (func(), error) {
	path := filepath.Join(s.Dir, "task.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("task is already active: %w", err)
	}
	f.WriteString(fmt.Sprintf("pid=%d\n", os.Getpid()))
	f.Close()
	return func() { _ = os.Remove(path) }, nil
}
