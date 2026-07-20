package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hellogxp/stateseal/internal/identity"
)

const DesktopSessionVersion = "v0alpha1"

const (
	DesktopStageSetupPending = "SETUP_PENDING"
	DesktopStageReady        = "READY"
	DesktopStageRunning      = "RUNNING"
	DesktopStagePendingApply = "PENDING_APPLY"
	DesktopStageApplied      = "APPLIED"
	DesktopStageRejected     = "REJECTED"
	DesktopStageFailed       = "FAILED"
)

// DesktopSession binds an Agent surface session to a repository, goal, and
// StateSeal task. It is authority state and therefore never lives in the
// Agent-writable repository.
type DesktopSession struct {
	Version   string    `json:"version"`
	SessionID string    `json:"session_id"`
	TurnID    string    `json:"turn_id,omitempty"`
	Agent     string    `json:"agent"`
	RepoRoot  string    `json:"repo_root"`
	Goal      string    `json:"goal"`
	TaskID    string    `json:"task_id,omitempty"`
	Stage     string    `json:"stage"`
	Verdict   string    `json:"verdict,omitempty"`
	ReceiptID string    `json:"receipt_id,omitempty"`
	LastError string    `json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func desktopSessionDir() (string, error) {
	home, err := stateHome()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "stateseal", "desktop", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func desktopSessionPath(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("desktop session id is required")
	}
	dir, err := desktopSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, identity.Digest([]byte(sessionID))+".json"), nil
}

func SaveDesktopSession(session DesktopSession) error {
	path, err := desktopSessionPath(session.SessionID)
	if err != nil {
		return err
	}
	if session.Version == "" {
		session.Version = DesktopSessionVersion
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	session.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadDesktopSession(sessionID string) (DesktopSession, error) {
	path, err := desktopSessionPath(sessionID)
	if err != nil {
		return DesktopSession{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return DesktopSession{}, err
	}
	var session DesktopSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return DesktopSession{}, fmt.Errorf("parse desktop session: %w", err)
	}
	if session.Version != DesktopSessionVersion || session.SessionID != sessionID || session.RepoRoot == "" {
		return DesktopSession{}, fmt.Errorf("desktop session authority state is invalid")
	}
	return session, nil
}

// LockDesktopRepository prevents two Desktop sessions from concurrently
// changing the same trusted base. Old crash residue is recoverable after the
// maximum supported managed-run window.
func LockDesktopRepository(repoRoot, sessionID string) (func(), error) {
	dir, err := repositoryDir(repoRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "desktop.lock")
	for attempt := 0; attempt < 2; attempt++ {
		f, openErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if openErr == nil {
			_, _ = fmt.Fprintf(f, "session=%s\npid=%d\nstarted=%s\n", sessionID, os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(openErr) {
			return nil, openErr
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil, statErr
		}
		if time.Since(info.ModTime()) <= 2*time.Hour {
			return nil, fmt.Errorf("another StateSeal Desktop session is active for this repository")
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale Desktop session lock: %w", err)
		}
	}
	return nil, fmt.Errorf("could not acquire StateSeal Desktop repository lock")
}
