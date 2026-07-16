package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Check struct {
	ID             string   `yaml:"id" json:"id"`
	Command        []string `yaml:"command" json:"command"`
	CWD            string   `yaml:"cwd,omitempty" json:"cwd,omitempty"`
	TimeoutSeconds int      `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type CheckSet struct {
	Checks         []Check `yaml:"checks" json:"checks"`
	TimeoutSeconds int     `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
}

type Completion struct {
	Checks                    []Check `yaml:"checks" json:"checks"`
	TimeoutSeconds            int     `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
	RecertifyLatestCheckpoint bool    `yaml:"recertify_latest_checkpoint" json:"recertify_latest_checkpoint"`
	OnMissingEvidence         string  `yaml:"on_missing_evidence" json:"on_missing_evidence"`
	OnStaleEvidence           string  `yaml:"on_stale_evidence" json:"on_stale_evidence"`
}

type Exception struct {
	Rule      string `yaml:"rule" json:"rule"`
	Scope     string `yaml:"scope" json:"scope"`
	Owner     string `yaml:"owner" json:"owner"`
	Reason    string `yaml:"reason" json:"reason"`
	ExpiresAt string `yaml:"expires_at" json:"expires_at"`
}

type Policy struct {
	Version string `yaml:"version" json:"version"`
	Task    struct {
		ID   string `yaml:"id" json:"id"`
		Goal string `yaml:"goal" json:"goal"`
	} `yaml:"task" json:"task"`
	State struct {
		Include   []string `yaml:"include" json:"include"`
		Protected []string `yaml:"protected" json:"protected"`
	} `yaml:"state" json:"state"`
	Admission  CheckSet   `yaml:"admission" json:"admission"`
	Completion Completion `yaml:"completion" json:"completion"`
	Execution  struct {
		CleanWorktree bool   `yaml:"clean_worktree" json:"clean_worktree"`
		Network       string `yaml:"network" json:"network"`
	} `yaml:"execution" json:"execution"`
	Budget struct {
		MaxCandidates  int `yaml:"max_candidates" json:"max_candidates"`
		MaxWallSeconds int `yaml:"max_wall_seconds" json:"max_wall_seconds"`
	} `yaml:"budget" json:"budget"`
	ResidualRisks []string    `yaml:"residual_risks" json:"residual_risks"`
	Exceptions    []Exception `yaml:"exceptions,omitempty" json:"exceptions,omitempty"`
}

func Default(taskID string, checks []Check) Policy {
	var p Policy
	p.Version = "v0alpha1"
	p.Task.ID = taskID
	p.Task.Goal = "Define the intended outcome."
	p.State.Include = []string{"**"}
	p.State.Protected = []string{"seal.yaml", ".git/**", ".stateseal/**", ".codex/**", ".github/workflows/**"}
	p.Admission.Checks = checks
	p.Admission.TimeoutSeconds = 900
	p.Completion.Checks = checks
	p.Completion.TimeoutSeconds = 900
	p.Completion.RecertifyLatestCheckpoint = true
	p.Completion.OnMissingEvidence = "abstain"
	p.Completion.OnStaleEvidence = "reject"
	p.Execution.CleanWorktree = true
	p.Execution.Network = "inherit"
	p.Budget.MaxCandidates = 8
	p.Budget.MaxWallSeconds = 1800
	p.ResidualRisks = []string{"Only configured checks were evaluated.", "The execution host was not independently attested."}
	return p
}

func Load(root string) (Policy, []byte, error) {
	path := filepath.Join(root, "seal.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, nil, fmt.Errorf("read %s: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return Policy{}, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := p.Validate(); err != nil {
		return Policy{}, nil, err
	}
	return p, raw, nil
}

func (p Policy) Validate() error {
	if p.Version != "v0alpha1" {
		return fmt.Errorf("unsupported policy version %q", p.Version)
	}
	if p.Task.ID == "" {
		return errors.New("task.id is required")
	}
	for group, checks := range map[string][]Check{"admission": p.Admission.Checks, "completion": p.Completion.Checks} {
		if len(checks) == 0 {
			return fmt.Errorf("%s.checks must contain at least one verifier", group)
		}
		ids := map[string]bool{}
		for i, c := range checks {
			if c.ID == "" || len(c.Command) == 0 || c.Command[0] == "" {
				return fmt.Errorf("%s.checks[%d] requires id and command argv", group, i)
			}
			if c.TimeoutSeconds < 0 {
				return fmt.Errorf("%s check %q has negative timeout", group, c.ID)
			}
			if ids[c.ID] {
				return fmt.Errorf("%s check id %q is duplicated", group, c.ID)
			}
			ids[c.ID] = true
		}
	}
	if p.Admission.TimeoutSeconds < 0 || p.Completion.TimeoutSeconds < 0 {
		return errors.New("check-set timeout_seconds cannot be negative")
	}
	if !p.Execution.CleanWorktree {
		return fmt.Errorf("execution.clean_worktree must be true for the v0alpha1 backend")
	}
	if !p.Completion.RecertifyLatestCheckpoint {
		return fmt.Errorf("completion.recertify_latest_checkpoint must be true")
	}
	if p.Completion.OnMissingEvidence != "abstain" {
		return fmt.Errorf("completion.on_missing_evidence must be abstain")
	}
	if p.Completion.OnStaleEvidence != "reject" {
		return fmt.Errorf("completion.on_stale_evidence must be reject")
	}
	if p.Execution.Network != "" && p.Execution.Network != "inherit" {
		return fmt.Errorf("execution.network=%q is not supported by the local worktree backend", p.Execution.Network)
	}
	for i, exception := range p.Exceptions {
		if exception.Rule != "PV001" || exception.Scope == "" || exception.Owner == "" || exception.Reason == "" || exception.ExpiresAt == "" {
			return fmt.Errorf("exceptions[%d] requires rule PV001, scope, owner, reason, and expires_at", i)
		}
		if _, err := time.Parse("2006-01-02", exception.ExpiresAt); err != nil {
			return fmt.Errorf("exceptions[%d].expires_at: %w", i, err)
		}
		if exception.Scope == "seal.yaml" || exception.Scope == ".stateseal" || len(exception.Scope) >= len(".stateseal/") && exception.Scope[:len(".stateseal/")] == ".stateseal/" {
			return fmt.Errorf("exceptions[%d] cannot waive policy or authority-state protection", i)
		}
	}
	return nil
}

func (p Policy) AllowsProtected(path string, now time.Time) bool {
	for _, exception := range p.Exceptions {
		expires, _ := time.Parse("2006-01-02", exception.ExpiresAt)
		if exception.Rule == "PV001" && exception.Scope == path && now.Before(expires.Add(24*time.Hour)) {
			return true
		}
	}
	return false
}

func (c Check) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func Write(path string, p Policy) error {
	raw, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
