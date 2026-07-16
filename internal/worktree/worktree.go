package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/identity"
)

type Manager struct {
	Root    string
	TaskID  string
	Base    string
	WorkDir string
}

func New(root, taskID string) (*Manager, error) {
	baseRaw, err := identity.Git(root, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("StateSeal requires at least one commit: %w", err)
	}
	base := strings.TrimSpace(string(baseRaw))
	home, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "stateseal", identity.Digest([]byte(root))[:16], taskID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Manager{Root: root, TaskID: taskID, Base: base, WorkDir: dir}, nil
}

func (m *Manager) Proposal() (string, error) {
	baseID := m.Base
	if len(baseID) > 12 {
		baseID = baseID[:12]
	}
	path := filepath.Join(m.WorkDir, "proposal-"+baseID)
	if _, err := os.Stat(path); err == nil {
		if err := m.linkIgnoredDependencies(path); err != nil {
			return "", err
		}
		return path, nil
	}
	branch := "stateseal/" + sanitize(m.TaskID) + "-" + baseID + "-" + time.Now().UTC().Format("20060102-150405")
	if _, err := identity.Git(m.Root, "worktree", "add", "-b", branch, path, m.Base); err != nil {
		return "", err
	}
	if err := m.linkIgnoredDependencies(path); err != nil {
		return "", err
	}
	return path, nil
}

func (m *Manager) CommitCandidate(proposal string) (string, error) {
	if _, err := identity.Git(proposal, "add", "-A"); err != nil {
		return "", err
	}
	quiet := identity.Git
	if _, err := quiet(proposal, "diff", "--cached", "--quiet"); err == nil {
		out, e := identity.Git(proposal, "rev-parse", "HEAD")
		return strings.TrimSpace(string(out)), e
	}
	envName, envEmail := os.Getenv("GIT_AUTHOR_NAME"), os.Getenv("GIT_AUTHOR_EMAIL")
	if envName == "" {
		identity.Git(proposal, "config", "user.name", "StateSeal Broker")
	}
	if envEmail == "" {
		identity.Git(proposal, "config", "user.email", "broker@stateseal.local")
	}
	if _, err := identity.Git(proposal, "commit", "-m", "Checkpoint candidate state"); err != nil {
		return "", err
	}
	out, err := identity.Git(proposal, "rev-parse", "HEAD")
	return strings.TrimSpace(string(out)), err
}

func (m *Manager) Evaluator(commit string) (string, func(), error) {
	path := filepath.Join(m.WorkDir, "eval-"+identity.ID("run"))
	if _, err := identity.Git(m.Root, "worktree", "add", "--detach", path, commit); err != nil {
		return "", nil, err
	}
	if err := m.linkIgnoredDependencies(path); err != nil {
		_, _ = identity.Git(m.Root, "worktree", "remove", "--force", path)
		return "", nil, err
	}
	cleanup := func() { _, _ = identity.Git(m.Root, "worktree", "remove", "--force", path) }
	return path, cleanup, nil
}

func (m *Manager) linkIgnoredDependencies(targetRoot string) error {
	for _, name := range []string{"node_modules"} {
		source := filepath.Join(m.Root, name)
		info, err := os.Stat(source)
		if os.IsNotExist(err) || err == nil && !info.IsDir() {
			continue
		}
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, name)
		if _, err := os.Lstat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if _, err := identity.Git(targetRoot, "check-ignore", "-q", "--no-index", name+"/"); err != nil {
			continue
		}
		if err := identity.EnsureLocalExclude(targetRoot, name); err != nil {
			return err
		}
		if err := os.Symlink(source, target); err != nil {
			return fmt.Errorf("link local dependency directory %s: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) ChangedFiles(base, commit string) ([]string, error) {
	out, err := identity.Git(m.Root, "diff", "--name-only", base, commit)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// Restore resets a managed proposal to an exact checkpoint. Ignored local
// dependencies are preserved, while every tracked and untracked candidate
// artifact is removed before the checkpoint is used again.
func (m *Manager) Restore(proposal, commit string) error {
	if _, err := identity.Git(proposal, "reset", "--hard", commit); err != nil {
		return err
	}
	if _, err := identity.Git(proposal, "clean", "-fd"); err != nil {
		return err
	}
	return m.linkIgnoredDependencies(proposal)
}

func sanitize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
