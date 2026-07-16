package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/broker"
	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/identity"
	processctl "github.com/hellogxp/stateseal/internal/process"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/internal/worktree"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := newRoot()
	if err := root.Execute(); err != nil {
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(11)
	}
}

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string { return e.err.Error() }
func (e codedError) ExitCode() int { return e.code }

type staleStateError struct{ reason string }

func (e staleStateError) Error() string { return e.reason }

func newRoot() *cobra.Command {
	cmd := &cobra.Command{Use: "seal", Short: "Transactional admission for coding-agent changes", SilenceUsage: true, SilenceErrors: true}
	cmd.Version = version
	cmd.AddCommand(initCmd(), verifyCmd(), runCmd(), submitCmd(), statusCmd(), timelineCmd(), diffCmd(), applyCmd(), explainCmd(), inspectCmd(), restoreCmd(), adapterCmd(), doctorCmd())
	return cmd
}

func initCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{Use: "init", Short: "Create a reviewed-by-default StateSeal policy", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := identity.GitRoot(".")
		if err != nil {
			return codedError{10, err}
		}
		path := filepath.Join(root, "seal.yaml")
		if _, err := os.Stat(path); err == nil && !force {
			return codedError{10, fmt.Errorf("%s already exists; use --force to replace it", path)}
		}
		checks, detected := detectChecks(root)
		p := config.Default(filepath.Base(root), checks)
		if err := config.Write(path, p); err != nil {
			return codedError{10, err}
		}
		if err := identity.EnsureLocalExclude(root, ".stateseal/"); err != nil {
			return codedError{10, err}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "StateSeal policy written to %s\nDetected verifier: %s\nReview goal, protected paths, and commands before using enforce mode.\n", path, detected)
		return nil
	}}
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing policy")
	return cmd
}

func verifyCmd() *cobra.Command {
	var mode string
	cmd := &cobra.Command{Use: "verify -- <command> [args...]", Short: "Bind fresh verifier evidence to the current code state", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := identity.GitRoot(".")
		if err != nil {
			return codedError{10, err}
		}
		b, err := broker.New(root, mode)
		if err != nil {
			return codedError{10, err}
		}
		unlock, err := b.Store.Lock()
		if err != nil {
			return codedError{11, err}
		}
		defer unlock()
		checks := []config.Check{{ID: "cli", Command: args, TimeoutSeconds: 900}}
		r, err := b.VerifyCurrent(checks)
		if err != nil {
			return codedError{11, err}
		}
		printReceipt(cmd, r)
		return handleVerdict(cmd, r, mode)
	}}
	cmd.Flags().StringVar(&mode, "mode", "enforce", "shadow, warn, or enforce")
	return cmd
}

func runCmd() *cobra.Command {
	var mode, source string
	var apply bool
	cmd := &cobra.Command{Use: "run -- <agent> [args...]", Short: "Run an existing coding agent in a managed proposal worktree", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if mode != "shadow" && mode != "warn" && mode != "enforce" {
			return codedError{10, fmt.Errorf("invalid mode %q", mode)}
		}
		root, err := identity.GitRoot(".")
		if err != nil {
			return codedError{10, err}
		}
		dirty, _ := identity.Git(root, "status", "--porcelain")
		if len(bytes.TrimSpace(dirty)) > 0 {
			return codedError{10, fmt.Errorf("trusted base is dirty; commit or stash changes before seal run")}
		}
		restoreEnvironment := augmentLocalToolPath(root)
		defer restoreEnvironment()
		b, err := broker.New(root, mode)
		if err != nil {
			return codedError{10, err}
		}
		unlock, err := b.Store.Lock()
		if err != nil {
			return codedError{11, err}
		}
		defer unlock()
		m, err := worktree.New(root, b.State.TaskID)
		if err != nil {
			return codedError{11, err}
		}
		proposal, err := m.Proposal()
		if err != nil {
			return codedError{11, err}
		}
		b.State.ProposalPath = proposal
		_ = b.Store.Save(b.State)
		fmt.Fprintf(cmd.OutOrStdout(), "Proposal worktree: %s\n", proposal)
		if source == "auto" {
			source = adapterSource(args[0])
		}
		requestDir := filepath.Join(b.Store.Dir, "submissions")
		if err := os.MkdirAll(requestDir, 0o700); err != nil {
			return codedError{11, err}
		}
		stopBroker := make(chan struct{})
		brokerDone := make(chan struct{})
		go serveSubmissions(stopBroker, brokerDone, requestDir, b, m, proposal, source, cmd.ErrOrStderr())
		maxWall := time.Duration(b.Policy.Budget.MaxWallSeconds) * time.Second
		if maxWall <= 0 {
			maxWall = 30 * time.Minute
		}
		ctx, cancel := context.WithTimeout(context.Background(), maxWall)
		defer cancel()
		agent := exec.CommandContext(ctx, args[0], args[1:]...)
		processctl.ConfigureGroup(agent)
		agent.Dir, agent.Stdin, agent.Stdout, agent.Stderr = proposal, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
		adapterChecks, _ := json.Marshal(checkCommandStrings(b.Policy))
		agent.Env = append(os.Environ(), "STATESEAL_TASK_ID="+b.State.TaskID, "STATESEAL_PROPOSAL_ROOT="+proposal, "STATESEAL_ORIGINAL_ROOT="+root, "STATESEAL_SUBMIT_DIR="+requestDir, "STATESEAL_MODE="+mode, "STATESEAL_ADAPTER_CHECKS="+string(adapterChecks))
		agentErr := agent.Run()
		_ = processctl.KillGroup(agent)
		close(stopBroker)
		<-brokerDone
		if ctx.Err() != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "Agent wall-time budget was exhausted; evaluating the latest candidate.")
		}
		if agentErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Agent exited with an error; terminal candidate will still be evaluated: %v\n", agentErr)
		}
		recoveryReason := "terminal_candidate_regressed"
		if ctx.Err() != nil {
			recoveryReason = "wall_budget_exhausted"
		} else if agentErr != nil {
			recoveryReason = "agent_exit_error"
		}
		r, err := b.AdmitManagedWithReason(m, proposal, source, recoveryReason)
		if err != nil {
			return codedError{11, err}
		}
		printReceipt(cmd, r)
		if apply && r.Verdict == protocol.VerdictAdmitted {
			if err := applyCheckpoint(root, b.State, ""); err != nil {
				return codedError{11, err}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Verified checkpoint applied to the current branch.")
		}
		return handleVerdict(cmd, r, mode)
	}}
	cmd.Flags().StringVar(&mode, "mode", "enforce", "shadow, warn, or enforce")
	cmd.Flags().StringVar(&source, "source", "auto", "candidate source identity")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply an admitted checkpoint to the current branch")
	return cmd
}

type submissionRequest struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func submitCmd() *cobra.Command {
	return &cobra.Command{Use: "submit", Short: "Submit an intermediate candidate to the active broker", RunE: func(cmd *cobra.Command, _ []string) error {
		receipt, err := requestSubmission()
		if err != nil {
			return err
		}
		printReceipt(cmd, receipt)
		mode := os.Getenv("STATESEAL_MODE")
		if mode == "" {
			mode = "enforce"
		}
		return handleVerdict(cmd, receipt, mode)
	}}
}

func requestSubmission() (protocol.CompletionReceipt, error) {
	dir := os.Getenv("STATESEAL_SUBMIT_DIR")
	if dir == "" {
		return protocol.CompletionReceipt{}, codedError{10, fmt.Errorf("seal submit must run inside an active seal run")}
	}
	request := submissionRequest{ID: identity.ID("submit"), CreatedAt: time.Now().UTC()}
	raw, _ := json.Marshal(request)
	if err := os.WriteFile(filepath.Join(dir, request.ID+".request.json"), raw, 0o600); err != nil {
		return protocol.CompletionReceipt{}, codedError{11, err}
	}
	responsePath := filepath.Join(dir, request.ID+".response.json")
	deadline := time.Now().Add(30 * time.Minute)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(responsePath)
		if err == nil {
			var response struct {
				Receipt protocol.CompletionReceipt `json:"receipt"`
				Error   string                     `json:"error,omitempty"`
			}
			if err := json.Unmarshal(raw, &response); err != nil {
				return protocol.CompletionReceipt{}, codedError{11, err}
			}
			if response.Error != "" {
				return protocol.CompletionReceipt{}, codedError{11, errors.New(response.Error)}
			}
			return response.Receipt, nil
		}
		if !os.IsNotExist(err) {
			return protocol.CompletionReceipt{}, codedError{11, err}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return protocol.CompletionReceipt{}, codedError{11, fmt.Errorf("broker did not answer the submission")}
}

func serveSubmissions(stop <-chan struct{}, done chan<- struct{}, dir string, b *broker.Broker, m *worktree.Manager, proposal, source string, stderr interface{ Write([]byte) (int, error) }) {
	defer close(done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	seen := map[string]bool{}
	processed := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			matches, _ := filepath.Glob(filepath.Join(dir, "*.request.json"))
			for _, path := range matches {
				if seen[path] {
					continue
				}
				seen[path] = true
				var req submissionRequest
				raw, err := os.ReadFile(path)
				if err == nil {
					err = json.Unmarshal(raw, &req)
				}
				var receipt protocol.CompletionReceipt
				if req.ID == "" {
					req.ID = strings.TrimSuffix(filepath.Base(path), ".request.json")
				}
				if err != nil {
					receipt, _ = b.RecordAbstention("malformed intermediate submission: " + err.Error())
				}
				maxIntermediate := b.Policy.Budget.MaxCandidates - 1
				if b.Policy.Budget.MaxCandidates > 0 && processed >= maxIntermediate {
					receipt, err = b.RecordAbstention("candidate budget exhausted; terminal candidate slot is reserved")
				}
				if err == nil && receipt.ReceiptID == "" {
					processed++
					b.State.Coverage = "intermediate + terminal"
					receipt, err = b.AdmitIntermediate(m, proposal, source+"-intermediate")
				}
				response := struct {
					Receipt protocol.CompletionReceipt `json:"receipt"`
					Error   string                     `json:"error,omitempty"`
				}{Receipt: receipt}
				if err != nil {
					response.Error = err.Error()
					fmt.Fprintf(stderr, "Intermediate submission failed: %v\n", err)
				}
				out, _ := json.Marshal(response)
				tmp := filepath.Join(dir, req.ID+".response.json.tmp")
				if writeErr := os.WriteFile(tmp, out, 0o600); writeErr == nil {
					_ = os.Rename(tmp, filepath.Join(dir, req.ID+".response.json"))
				}
			}
		}
	}
}

func statusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{Use: "status", Short: "Show task, checkpoint, and evidence freshness", RunE: func(cmd *cobra.Command, _ []string) error {
		state, _, err := loadState()
		if err != nil {
			return codedError{10, err}
		}
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(state)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Task:       %s\nStatus:     %s\nFreshness:  %s\nMode:       %s\nCoverage:   %s\n", state.TaskID, state.Status, state.Freshness, state.Mode, state.Coverage)
		if state.StaleReason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Stale:      %s\n", state.StaleReason)
		}
		if state.Checkpoint != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Checkpoint: %s\nTree:       %s\n", state.Checkpoint.CheckpointID, short(state.Checkpoint.TreeSHA256))
		}
		if state.Receipt != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Receipt:    %s\n", state.Receipt.ReceiptID)
			if state.Receipt.Recovered {
				fmt.Fprintf(cmd.OutOrStdout(), "Recovered:  yes (%s)\n", state.Receipt.SelectionReason)
			}
		}
		return nil
	}}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable state")
	return cmd
}

func timelineCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{Use: "timeline", Short: "Show the integrity-verified reliability timeline", RunE: func(cmd *cobra.Command, _ []string) error {
		_, s, err := loadState()
		if err != nil {
			return codedError{10, err}
		}
		events, err := s.ReadEvents()
		if err != nil {
			return codedError{1, err}
		}
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(events)
		}
		for _, event := range events {
			rule, _ := event.Data["rule_id"].(string)
			reason, _ := event.Data["reason"].(string)
			fmt.Fprintf(cmd.OutOrStdout(), "%03d  %s  %-25s", event.Sequence, event.Timestamp.Format(time.RFC3339), event.Type)
			if rule != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s", rule)
			}
			if reason != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s", reason)
			}
			fmt.Fprintln(cmd.OutOrStdout())
		}
		return nil
	}}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable ledger events")
	return cmd
}

func diffCmd() *cobra.Command {
	return &cobra.Command{Use: "diff", Short: "Show the verified checkpoint diff from its trusted base", RunE: func(cmd *cobra.Command, _ []string) error {
		state, _, err := loadState()
		if err != nil {
			return codedError{10, err}
		}
		if state.Checkpoint == nil || state.Checkpoint.Commit == "" {
			return codedError{2, fmt.Errorf("no managed verified checkpoint is available")}
		}
		out, err := identity.Git(state.RepoRoot, "diff", "--binary", state.BaseCommit, state.Checkpoint.Commit)
		if err != nil {
			return codedError{11, err}
		}
		_, err = cmd.OutOrStdout().Write(out)
		return err
	}}
}

func applyCmd() *cobra.Command {
	var branch string
	cmd := &cobra.Command{Use: "apply", Short: "Apply the verified checkpoint to the user's branch", RunE: func(cmd *cobra.Command, _ []string) error {
		state, _, err := loadState()
		if err != nil {
			return codedError{10, err}
		}
		if err := applyCheckpoint(state.RepoRoot, state, branch); err != nil {
			var stale staleStateError
			if errors.As(err, &stale) {
				return codedError{3, err}
			}
			return codedError{11, err}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Verified checkpoint applied.")
		return nil
	}}
	cmd.Flags().StringVar(&branch, "branch", "", "create and switch to this branch before applying")
	return cmd
}

func explainCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{Use: "explain", Short: "Explain the latest verdict and next action", RunE: func(cmd *cobra.Command, _ []string) error {
		state, _, err := loadState()
		if err != nil {
			return codedError{10, err}
		}
		next := nextAction(state.Status)
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"verdict": state.Status, "rule_id": state.RuleID, "rule_summary": protocol.RuleSummary(state.RuleID),
				"reason": state.LastError, "mode": state.Mode, "disposition": state.Disposition, "next_action": next,
				"receipt": state.Receipt, "checkpoint": state.Checkpoint,
			})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Verdict: %s\n", state.Status)
		if state.RuleID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Rule:    %s — %s\n", state.RuleID, protocol.RuleSummary(state.RuleID))
		}
		if state.LastError != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Reason:  %s\n", state.LastError)
		}
		switch state.Status {
		case "ADMITTED":
			if state.Receipt != nil && state.Receipt.Recovered {
				fmt.Fprintf(cmd.OutOrStdout(), "Selected checkpoint %s after %s and freshly recertified it.\n", state.Receipt.CheckpointID, state.Receipt.SelectionReason)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Next: %s\n", next)
		return nil
	}}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a machine-readable explanation")
	return cmd
}

func nextAction(status string) string {
	switch status {
	case "ADMITTED":
		return "inspect with `seal diff`, then use `seal apply`"
	case "REJECTED":
		return "fix the failing check in the proposal worktree and run again"
	case "STALE":
		return "rerun verification against the current state"
	default:
		return "inspect configuration, evidence output, and residual risks"
	}
}

func inspectCmd() *cobra.Command {
	return &cobra.Command{Use: "inspect <receipt.json>", Short: "Validate and display a receipt", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		r, err := broker.InspectReceipt(args[0])
		if err != nil {
			return codedError{1, err}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Receipt %s is structurally intact.\nVerdict: %s\nTree:    %s\n", r.ReceiptID, r.Verdict, short(r.TreeSHA256))
		fmt.Fprintln(cmd.OutOrStdout(), "Note: local receipt integrity does not establish CI authority.")
		return nil
	}}
}

func restoreCmd() *cobra.Command {
	return &cobra.Command{Use: "restore [checkpoint]", Short: "Restore the managed proposal worktree to a verified checkpoint", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		state, _, err := loadState()
		if err != nil {
			return codedError{10, err}
		}
		if state.Checkpoint == nil || state.ProposalPath == "" {
			return codedError{2, fmt.Errorf("no restorable checkpoint")}
		}
		if len(args) == 1 && args[0] != state.Checkpoint.CheckpointID {
			return codedError{10, fmt.Errorf("unknown checkpoint %q", args[0])}
		}
		m, err := worktree.New(state.RepoRoot, state.TaskID)
		if err != nil {
			return codedError{11, err}
		}
		if err := m.Restore(state.ProposalPath, state.Checkpoint.Commit); err != nil {
			return codedError{11, err}
		}
		policy, _, err := config.Load(state.RepoRoot)
		if err != nil {
			return codedError{10, err}
		}
		tree, err := identity.Tree(state.ProposalPath, policy.State.Include)
		if err != nil {
			return codedError{11, err}
		}
		if tree != state.Checkpoint.TreeSHA256 {
			return codedError{3, fmt.Errorf("restored checkpoint tree does not match the ledger")}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Proposal restored to %s.\n", state.Checkpoint.CheckpointID)
		return nil
	}}
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check local prerequisites and policy", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := identity.GitRoot(".")
		if err != nil {
			return codedError{10, err}
		}
		checks := []struct {
			name string
			err  error
		}{{"Git repository", nil}}
		_, _, policyErr := config.Load(root)
		checks = append(checks, struct {
			name string
			err  error
		}{"seal.yaml", policyErr})
		_, gitErr := exec.LookPath("git")
		checks = append(checks, struct {
			name string
			err  error
		}{"git executable", gitErr})
		failed := false
		for _, c := range checks {
			if c.err != nil {
				failed = true
				fmt.Fprintf(cmd.OutOrStdout(), "✗ %s: %v\n", c.name, c.err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ %s\n", c.name)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		if failed {
			return codedError{10, fmt.Errorf("doctor found configuration errors")}
		}
		return nil
	}}
}

func detectChecks(root string) ([]config.Check, string) {
	type candidate struct {
		file, name string
		command    []string
	}
	options := []candidate{{"go.mod", "go test", []string{"go", "test", "./..."}}, {"pyproject.toml", "pytest", []string{"pytest"}}, {"pytest.ini", "pytest", []string{"pytest"}}, {"package.json", "npm test", []string{"npm", "test"}}, {"Cargo.toml", "cargo test", []string{"cargo", "test"}}}
	for _, c := range options {
		if _, err := os.Stat(filepath.Join(root, c.file)); err == nil {
			return []config.Check{{ID: "tests", Command: c.command, TimeoutSeconds: 900}}, c.name
		}
	}
	return []config.Check{{ID: "review-required", Command: []string{"git", "diff", "--check"}, TimeoutSeconds: 60}}, "git diff --check (replace with project tests)"
}

func augmentLocalToolPath(root string) func() {
	original := os.Getenv("PATH")
	var additions []string
	for _, rel := range []string{"node_modules/.bin", ".venv/bin", "venv/bin"} {
		candidate := filepath.Join(root, filepath.FromSlash(rel))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			additions = append(additions, candidate)
		}
	}
	if len(additions) > 0 {
		_ = os.Setenv("PATH", strings.Join(append(additions, original), string(os.PathListSeparator)))
	}
	return func() { _ = os.Setenv("PATH", original) }
}

func loadState() (protocol.TaskState, *store.Store, error) {
	root, err := identity.GitRoot(".")
	if err != nil {
		return protocol.TaskState{}, nil, err
	}
	p, _, err := config.Load(root)
	if err != nil {
		return protocol.TaskState{}, nil, err
	}
	s, err := store.Open(root, p.Task.ID)
	if err != nil {
		return protocol.TaskState{}, nil, err
	}
	state, err := s.Load()
	if err == nil {
		state.Freshness, state.StaleReason = stateFreshness(root, rawPolicyDigest(root), state)
		if state.Freshness == "STALE" {
			if strings.Contains(state.StaleReason, "policy") {
				state.RuleID = protocol.RulePolicyChanged
			} else if strings.Contains(state.StaleReason, "trusted base") {
				state.RuleID = protocol.RuleTrustedBaseChanged
			}
		}
	}
	return state, s, err
}

func applyCheckpoint(root string, state protocol.TaskState, branch string) error {
	if state.Receipt == nil || state.Receipt.Verdict != protocol.VerdictAdmitted || state.Checkpoint == nil {
		return fmt.Errorf("only an admitted checkpoint can be applied")
	}
	policyDigest := rawPolicyDigest(root)
	freshness, reason := stateFreshness(root, policyDigest, state)
	if freshness != "CURRENT" {
		return staleStateError{reason: "admission is stale: " + reason}
	}
	dirty, _ := identity.Git(root, "status", "--porcelain")
	if len(bytes.TrimSpace(dirty)) > 0 {
		return fmt.Errorf("current worktree is dirty")
	}
	if state.Checkpoint.Commit == state.BaseCommit {
		return nil
	}
	if _, err := identity.Git(root, "merge-base", "--is-ancestor", state.BaseCommit, state.Checkpoint.Commit); err != nil {
		return fmt.Errorf("checkpoint is not descended from the trusted base: %w", err)
	}
	if branch != "" {
		if _, err := identity.Git(root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			return fmt.Errorf("branch %q already exists", branch)
		}
		_, err := identity.Git(root, "switch", "-c", branch, state.Checkpoint.Commit)
		return err
	}
	_, err := identity.Git(root, "merge", "--ff-only", state.Checkpoint.Commit)
	return err
}

func rawPolicyDigest(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "seal.yaml"))
	if err != nil {
		return ""
	}
	return identity.Digest(raw)
}

func stateFreshness(root, policyDigest string, state protocol.TaskState) (string, string) {
	if state.Receipt == nil {
		return "MISSING", "no completion receipt"
	}
	if policyDigest == "" || policyDigest != state.Receipt.PolicyDigest {
		return "STALE", "policy changed after admission"
	}
	head, err := identity.Git(root, "rev-parse", "HEAD")
	if err != nil {
		return "UNKNOWN", "current Git state is unavailable"
	}
	if strings.TrimSpace(string(head)) != state.BaseCommit {
		return "STALE", "trusted base changed after admission"
	}
	return "CURRENT", ""
}

func printReceipt(cmd *cobra.Command, r protocol.CompletionReceipt) {
	fmt.Fprintf(cmd.OutOrStdout(), "\nStateSeal — %s\nReceipt: %s\n", r.Verdict, r.ReceiptID)
	if r.TreeSHA256 != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Tree:    %s\n", short(r.TreeSHA256))
	}
	if r.Reason != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Reason:  %s\n", r.Reason)
	}
	if r.RuleID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Rule:    %s — %s\n", r.RuleID, protocol.RuleSummary(r.RuleID))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Mode:    %s (%s)\n", r.EnforcementMode, r.Disposition)
	if r.Recovered {
		fmt.Fprintf(cmd.OutOrStdout(), "Recovered checkpoint: %s\nTerminal candidate:  %s\nSelection reason:    %s\n", r.CheckpointID, r.TerminalCandidate, r.SelectionReason)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Residual risk:")
	for _, risk := range r.ResidualRisks {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", risk)
	}
	if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
		if f, err := os.OpenFile(summary, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			fmt.Fprintf(f, "## StateSeal — %s\n\n| Field | Value |\n| --- | --- |\n| Receipt | `%s` |\n| Tree | `%s` |\n| Policy | `%s` |\n", r.Verdict, r.ReceiptID, short(r.TreeSHA256), short(r.PolicyDigest))
			if r.Recovered {
				fmt.Fprintf(f, "| Recovered | yes |\n| Selection reason | `%s` |\n", r.SelectionReason)
			}
			fmt.Fprint(f, "\n### Residual risk\n\n")
			for _, risk := range r.ResidualRisks {
				fmt.Fprintf(f, "- %s\n", risk)
			}
			fmt.Fprintln(f)
		}
	}
}

func handleVerdict(cmd *cobra.Command, r protocol.CompletionReceipt, mode string) error {
	if r.Verdict != protocol.VerdictAdmitted {
		switch mode {
		case "shadow":
			fmt.Fprintf(cmd.ErrOrStderr(), "Shadow: enforce mode would block %s; execution continues.\n", r.Verdict)
		case "warn":
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s was overridden by warn mode; the override is recorded in the ledger.\n", r.Verdict)
		}
	}
	return verdictError(r.Verdict, mode)
}

func checkCommandStrings(p config.Policy) []string {
	seen := map[string]bool{}
	var commands []string
	for _, check := range append(append([]config.Check(nil), p.Admission.Checks...), p.Completion.Checks...) {
		bare := shellJoin(check.Command)
		variants := []string{bare}
		if check.CWD != "" && check.CWD != "." {
			variants = append(variants, "cd "+shellQuote(check.CWD)+" && "+bare)
		}
		for _, command := range variants {
			if command != "" && !seen[command] {
				seen[command] = true
				commands = append(commands, command)
			}
		}
	}
	return commands
}

func verdictError(v protocol.Verdict, mode string) error {
	if mode == "shadow" || mode == "warn" || v == protocol.VerdictAdmitted {
		return nil
	}
	codes := map[protocol.Verdict]int{protocol.VerdictRejected: 1, protocol.VerdictAbstained: 2, protocol.VerdictStale: 3, protocol.VerdictEscalated: 4}
	return codedError{codes[v], fmt.Errorf("completion verdict: %s", v)}
}

func adapterSource(exe string) string {
	base := strings.ToLower(filepath.Base(exe))
	if strings.Contains(base, "codex") {
		return "codex-adapter"
	}
	if strings.Contains(base, "claude") {
		return "claude-adapter"
	}
	if strings.Contains(base, "opencode") {
		return "opencode-adapter"
	}
	return "command-adapter"
}
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
