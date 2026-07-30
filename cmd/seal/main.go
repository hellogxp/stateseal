package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/broker"
	"github.com/hellogxp/stateseal/internal/buildinfo"
	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/hellogxp/stateseal/internal/identity"
	processctl "github.com/hellogxp/stateseal/internal/process"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/internal/worktree"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/spf13/cobra"
)

func main() {
	root := newRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	info := buildinfo.Current()
	cmd := &cobra.Command{
		Use:   "seal",
		Short: "Read-only runtime intelligence for coding-agent delivery",
		Long: "StateSeal observes coding-agent sessions and delivery evidence without " +
			"starting, steering, blocking, approving, applying, or otherwise changing Agent execution.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Version = info.Version
	cmd.SetVersionTemplate("seal {{.Version}}\n")
	// Product invariant: every default command is observational. Managed-agent
	// execution, lifecycle hooks, admission, apply, restore, and MCP authority
	// are intentionally not registered.
	cmd.AddCommand(
		versionCmd(),
		workspaceCmd(),
		uiCmd(),
		inertAdapterCompatibilityCmd(),
	)
	return cmd
}

func versionCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version and build identity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := buildinfo.Current()
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "StateSeal %s\nCommit:    %s\nBuilt:     %s\nGo:        %s\nPlatform:  %s\n", info.Version, info.Commit, info.BuildDate, info.GoVersion, info.Platform)
			if info.Modified {
				fmt.Fprintln(cmd.OutOrStdout(), "Modified:  yes")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable build information")
	return cmd
}

func initCmd() *cobra.Command {
	var force bool
	var taskID, goal string
	cmd := &cobra.Command{Use: "init", Short: "Create a reviewed-by-default StateSeal policy", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := identity.GitRoot(".")
		if err != nil {
			return codedError{10, err}
		}
		path := filepath.Join(root, "seal.yaml")
		if _, err := os.Stat(path); err == nil && !force {
			return codedError{10, fmt.Errorf("%s already exists; use --force to replace it", path)}
		}
		plan := discoverVerificationPlan(root)
		checks, detected := plan.Admission, strings.Join(plan.Detected, ", ")
		if taskID == "" {
			taskID = identity.NormalizeTaskID(filepath.Base(root))
		} else if err := identity.ValidateTaskID(taskID); err != nil {
			return codedError{10, err}
		}
		p := config.Default(taskID, checks)
		p.Completion.Checks = plan.Completion
		for _, gap := range plan.Uncovered {
			p.ResidualRisks = append(p.ResidualRisks, gap+".")
		}
		if goal != "" {
			p.Task.Goal = goal
		}
		if err := config.Write(path, p); err != nil {
			return codedError{10, err}
		}
		if err := identity.EnsureLocalExclude(root, ".stateseal/"); err != nil {
			return codedError{10, err}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "StateSeal initialized\n\nPolicy:    %s\nTask:      %s\nVerifier:  %s\nMode:      enforce (default)\n\nNext:\n  1. Review task.goal, protected paths, and verifier commands in seal.yaml.\n  2. Run `seal doctor`.\n  3. Install the repository adapter with `seal adapter <agent> install`.\n  4. Start managed development with `seal run -- <agent> [args...]`.\n\nStateSeal runs the configured verifier automatically; `seal verify` remains available for standalone verification.\n", path, p.Task.ID, detected)
		return nil
	}}
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing policy")
	cmd.Flags().StringVar(&taskID, "task-id", "", "stable task identifier (defaults to the repository name)")
	cmd.Flags().StringVar(&goal, "goal", "", "intended outcome recorded in the policy")
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
	var mode, source, agentName, goal, taskID, branch, repoPath string
	var apply, noApply, verbose, quiet, jsonOut, autonomous, yes, requireChange bool
	cmd := &cobra.Command{Use: "run \"<goal>\"", Short: "Develop a goal with a coding Agent and deliver only verified changes", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		locale := i18n.Detect()
		if mode != "shadow" && mode != "warn" && mode != "enforce" {
			return codedError{10, fmt.Errorf("invalid mode %q", mode)}
		}
		if apply && noApply {
			return codedError{10, fmt.Errorf("--apply and --no-apply cannot be used together")}
		}
		if boolCount(verbose, quiet, jsonOut) > 1 {
			return codedError{10, fmt.Errorf("--verbose, --quiet, and --json are mutually exclusive")}
		}
		legacy := cmd.ArgsLenAtDash() >= 0
		if legacy && (agentName != "" || goal != "") {
			return codedError{10, fmt.Errorf("do not combine --agent or --goal with a legacy command after --")}
		}
		if !legacy {
			if goal != "" && len(args) > 0 {
				return codedError{10, fmt.Errorf("provide the goal either as an argument or with --goal, not both")}
			}
			if goal == "" {
				goal = strings.TrimSpace(strings.Join(args, " "))
			}
			if strings.TrimSpace(goal) == "" {
				return codedError{10, fmt.Errorf("%s", locale.T(i18n.GoalRequired))}
			}
		} else if len(args) == 0 {
			return codedError{10, fmt.Errorf("provide a command after --")}
		}
		routingGoal := goal
		if legacy {
			routingGoal = strings.Join(args, " ")
		}
		root, routing, err := resolveRunRepository(cmd, repoPath, routingGoal, locale, quiet || jsonOut)
		if err != nil {
			return codedError{10, err}
		}
		if !quiet && !jsonOut {
			printWorkspaceRoute(cmd.OutOrStdout(), routing, locale)
		}
		var agentArgs []string
		desktopMCPChild := os.Getenv("STATESEAL_DESKTOP_MCP_CHILD") == "1"
		if !legacy {
			settings, settingsErr := store.LoadProjectSettings(root)
			if settingsErr != nil && !os.IsNotExist(settingsErr) {
				return codedError{10, settingsErr}
			}
			if agentName == "" {
				agentName = settings.Agent
			}
			if agentName == "" {
				agentName, err = detectAgent()
				if err != nil {
					return codedError{10, err}
				}
			}
			if !isSupportedAgent(agentName) {
				return codedError{10, fmt.Errorf("unsupported agent %q; choose one of: %s", agentName, strings.Join(supportedAgents, ", "))}
			}
			if _, err := exec.LookPath(agentExecutable(agentName)); err != nil {
				return codedError{10, fmt.Errorf("%s executable %q was not found in PATH", agentDisplayName(agentName), agentExecutable(agentName))}
			}
			if err := identity.CheckpointIdentity(root); err != nil {
				return codedError{10, err}
			}
			trustedHooks := false
			if desktopMCPChild {
				if err := ensureDesktopMCPSetup(root, agentName); err != nil {
					return codedError{10, err}
				}
			} else {
				if err := ensureManagedSetup(cmd, root, agentName, locale, yes, quiet || jsonOut); err != nil {
					return codedError{10, err}
				}
				if err := adapterHandshake(root, agentName); err != nil {
					return codedError{10, fmt.Errorf("Agent integration handshake failed: %w", err)}
				}
				trustedHooks, err = authorizeTrustedHooks(cmd, root, agentName, locale, yes, jsonOut)
				if err != nil {
					return codedError{10, err}
				}
			}
			agentArgs, err = agentLaunch(agentName, goal, autonomous, trustedHooks)
			if err != nil {
				return codedError{10, err}
			}
			agentArgs = isolateManagedChild(agentName, agentArgs)
			if taskID == "" {
				taskID = newTaskID(goal, time.Now().UTC())
			}
			if err := identity.ValidateTaskID(taskID); err != nil {
				return codedError{10, err}
			}
		} else {
			agentArgs = args
		}
		if err := identity.CheckpointIdentity(root); err != nil {
			return codedError{10, err}
		}
		dirty, _ := identity.Git(root, "status", "--porcelain")
		if len(bytes.TrimSpace(dirty)) > 0 {
			return codedError{10, fmt.Errorf("trusted base is dirty; commit or stash changes before seal run")}
		}
		restoreEnvironment := augmentLocalToolPath(root)
		defer restoreEnvironment()
		var b *broker.Broker
		if agentName != "" {
			b, err = broker.NewTask(root, mode, taskID, goal)
		} else {
			b, err = broker.New(root, mode)
		}
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
		progress := newRunProgress(cmd.OutOrStdout(), locale, !quiet && !jsonOut, !verbose && isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()))
		displayAgent := agentDisplayNameFromExecutable(agentArgs[0])
		progress.Header(b.State.Goal, displayAgent, b.Policy)
		progress.WorkspaceReady()
		progress.ProposalReady()
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "  Task:     %s\n  Proposal: %s\n", b.State.TaskID, proposal)
			printVerificationPlan(cmd.OutOrStdout(), b.Policy, "")
		}
		if source == "auto" {
			source = adapterSource(agentArgs[0])
		}
		requestDir := filepath.Join(b.Store.Dir, "submissions")
		if err := os.MkdirAll(requestDir, 0o700); err != nil {
			return codedError{11, err}
		}
		adapterChecks, _ := json.Marshal(checkCommandStrings(b.Policy))
		removeHookRuntime, err := writeHookRuntime(proposal, requestDir, mode, string(adapterChecks))
		if err != nil {
			return codedError{11, err}
		}
		defer removeHookRuntime()
		stopBroker := make(chan struct{})
		brokerDone := make(chan loopOutcome)
		go serveSubmissions(stopBroker, brokerDone, requestDir, b, m, proposal, source, displayAgent, cmd.ErrOrStderr(), progress)
		maxWall := time.Duration(b.Policy.Budget.MaxWallSeconds) * time.Second
		if maxWall <= 0 {
			maxWall = 30 * time.Minute
		}
		ctx, cancel := context.WithTimeout(context.Background(), maxWall)
		defer cancel()
		agent := exec.CommandContext(ctx, agentArgs[0], agentArgs[1:]...)
		processctl.ConfigureGroup(agent)
		agent.Dir, agent.Stdin = proposal, cmd.InOrStdin()
		agentLogPath := filepath.Join(b.Store.Dir, "agent.log")
		agentLog, err := os.OpenFile(agentLogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return codedError{11, err}
		}
		defer agentLog.Close()
		var agentStdout, agentStderr io.Writer = agentLog, agentLog
		if verbose {
			agentStdout = io.MultiWriter(cmd.OutOrStdout(), agentLog)
			agentStderr = io.MultiWriter(cmd.ErrOrStderr(), agentLog)
		}
		agent.Stdout, agent.Stderr = agentStdout, agentStderr
		agentEnv, err := agentCacheEnvironment(os.Environ(), os.TempDir(), b.Store.Dir)
		if err != nil {
			return codedError{11, err}
		}
		agent.Env = mergeEnvironment(agentEnv, map[string]string{
			"STATESEAL_TASK_ID":        b.State.TaskID,
			"STATESEAL_PROPOSAL_ROOT":  proposal,
			"STATESEAL_ORIGINAL_ROOT":  root,
			"STATESEAL_SUBMIT_DIR":     requestDir,
			"STATESEAL_MODE":           mode,
			"STATESEAL_ADAPTER_CHECKS": string(adapterChecks),
		})
		if autonomous && !legacy && !quiet && !jsonOut {
			fmt.Fprintln(cmd.OutOrStdout(), locale.T(i18n.AutonomousNotice))
		}
		progress.AgentStarted(displayAgent, proposal, b.State.BaseCommit)
		agentErr := agent.Run()
		progress.StopAgentHeartbeat()
		_ = processctl.KillGroup(agent)
		close(stopBroker)
		loopResult := <-brokerDone
		if ctx.Err() != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), locale.T(i18n.AgentTimedOut))
		}
		if agentErr != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), locale.T(i18n.AgentExited, agentErr))
		}
		recoveryReason := "terminal_candidate_regressed"
		if ctx.Err() != nil {
			recoveryReason = "wall_budget_exhausted"
		} else if agentErr != nil {
			recoveryReason = "agent_exit_error"
		}
		progress.FinalStarted(b.Policy.Completion.Checks, changedFileCount(proposal, b.State.BaseCommit))
		finalStarted := time.Now()
		r, err := b.AdmitManagedWithReason(m, proposal, source, recoveryReason)
		if err != nil {
			return codedError{11, err}
		}
		if requireChange && r.Verdict == protocol.VerdictAdmitted && verifiedChangedFiles(b.State) == 0 {
			r, err = b.RecordRejection("development task produced no deliverable change; the candidate matches the trusted base")
			if err != nil {
				return codedError{11, err}
			}
		}
		progress.FinalResult(r, b.Policy.Completion.Checks, b.State.Evidence, time.Since(finalStarted))
		if loopResult.NoProgress && r.Verdict != protocol.VerdictAdmitted {
			r, err = b.RecordEscalation(loopResult.Reason)
			if err != nil {
				return codedError{11, err}
			}
		}
		snapshot := progress.Complete()
		if verbose {
			fmt.Fprintln(cmd.OutOrStdout(), "\nIndependent verification")
			printReceipt(cmd, r)
			printRunResult(cmd.OutOrStdout(), r, b.State, locale, snapshot)
		} else if !quiet && !jsonOut {
			printRunResult(cmd.OutOrStdout(), r, b.State, locale, snapshot)
		}
		shouldApply := apply
		if r.Verdict == protocol.VerdictAdmitted && !apply && !noApply && !jsonOut && isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
			shouldApply, err = confirmApply(cmd.InOrStdin(), cmd.OutOrStdout(), locale)
			if err != nil {
				return codedError{10, err}
			}
		}
		if shouldApply && r.Verdict == protocol.VerdictAdmitted {
			if branch == "" && !legacy {
				branch = defaultDeliveryBranch(root, b.State.TaskID)
			}
			if err := applyCheckpoint(root, &b.State, branch); err != nil {
				return codedError{11, err}
			}
			if err := persistAppliedState(b.Store, b.State); err != nil {
				return codedError{11, err}
			}
			if !jsonOut && !quiet {
				fmt.Fprintln(cmd.OutOrStdout(), locale.T(i18n.Applied, b.State.AppliedBranch))
			}
		} else if r.Verdict == protocol.VerdictAdmitted {
			if !jsonOut && !quiet {
				fmt.Fprintln(cmd.OutOrStdout(), locale.T(i18n.NotApplied))
			}
		}
		if quiet {
			printQuietRunResult(cmd.OutOrStdout(), r, b.State, locale)
		}
		if jsonOut {
			if err := printJSONRunResult(cmd.OutOrStdout(), r, b.State, snapshot); err != nil {
				return codedError{11, err}
			}
		}
		return handleVerdict(cmd, r, mode)
	}}
	cmd.Flags().StringVar(&mode, "mode", "enforce", "shadow, warn, or enforce")
	cmd.Flags().StringVar(&source, "source", "auto", "candidate source identity")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply an admitted checkpoint to the current branch")
	cmd.Flags().BoolVar(&noApply, "no-apply", false, "leave an admitted checkpoint in StateSeal authority state")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "stream detailed Agent output")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "show only the final delivery status")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a stable machine-readable result")
	cmd.Flags().BoolVar(&autonomous, "autonomous", false, "allow non-interactive Agent permissions; StateSeal verification remains external")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "approve first-run project setup")
	cmd.Flags().BoolVar(&requireChange, "require-change", false, "reject an otherwise valid run when it produces no deliverable code change")
	cmd.Flags().StringVar(&agentName, "agent", "", "coding Agent to launch")
	cmd.Flags().StringVar(&goal, "goal", "", "intended development outcome")
	cmd.Flags().StringVar(&taskID, "task-id", "", "task identifier (generated from the goal by default)")
	cmd.Flags().StringVar(&branch, "branch", "", "create this branch when applying the verified checkpoint")
	cmd.Flags().StringVar(&repoPath, "repo", "", "Git repository path or name when running from a non-Git workspace")
	return cmd
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func defaultDeliveryBranch(root, taskID string) string {
	current, err := identity.Git(root, "branch", "--show-current")
	if err != nil {
		return ""
	}
	switch strings.TrimSpace(string(current)) {
	case "main", "master", "trunk":
		slug := strings.TrimSpace(taskID)
		parts := strings.Split(slug, "-")
		if len(parts) > 1 && len(parts[len(parts)-1]) == 10 {
			slug = strings.Join(parts[:len(parts)-1], "-")
		}
		if len(slug) > 48 {
			slug = strings.Trim(slug[:48], "-")
		}
		if slug == "" {
			slug = "verified-change"
		}
		return "feature/" + slug
	default:
		return ""
	}
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

type loopOutcome struct {
	NoProgress bool
	Reason     string
}

func serveSubmissions(stop <-chan struct{}, done chan<- loopOutcome, dir string, b *broker.Broker, m *worktree.Manager, proposal, source, agent string, stderr interface{ Write([]byte) (int, error) }, progress *runProgress) {
	outcome := loopOutcome{}
	defer func() {
		done <- outcome
		close(done)
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	seen := map[string]bool{}
	processed := 0
	lastTree := ""
	sameRejectedTree := 0
	var treeHistory []string
	var lastReceipt protocol.CompletionReceipt
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
				if err == nil && receipt.ReceiptID == "" {
					tree, treeErr := identity.Tree(proposal, b.Policy.State.Include)
					if treeErr != nil {
						err = treeErr
					} else if tree == lastTree && lastReceipt.ReceiptID != "" {
						if lastReceipt.Verdict != protocol.VerdictAdmitted {
							sameRejectedTree++
						}
						if sameRejectedTree >= 3 {
							outcome = loopOutcome{NoProgress: true, Reason: "agent made no progress after repeating the same rejected candidate three times"}
							receipt, err = b.RecordEscalation(outcome.Reason)
						} else {
							receipt = lastReceipt
						}
					}
				}
				maxIntermediate := b.Policy.Budget.MaxCandidates - 1
				if err == nil && receipt.ReceiptID == "" && b.Policy.Budget.MaxCandidates > 0 && processed >= maxIntermediate {
					receipt, err = b.RecordAbstention("candidate budget exhausted; terminal candidate slot is reserved")
				}
				if err == nil && receipt.ReceiptID == "" {
					processed++
					b.State.Coverage = "intermediate + terminal"
					if progress != nil {
						progress.Candidate(processed, changedFileCount(proposal, b.State.BaseCommit), b.Policy.Admission.Checks, b.Policy.Completion.Checks)
					}
					started := time.Now()
					receipt, err = b.AdmitIntermediate(m, proposal, source+"-intermediate")
					if err == nil && progress != nil {
						progress.CandidateResult(receipt, b.Policy.Completion.Checks, b.State.Evidence, time.Since(started), agent)
					}
					if err == nil {
						lastTree = receipt.TreeSHA256
						if b.State.Candidate != nil {
							lastTree = b.State.Candidate.ResultTreeSHA256
						}
						lastReceipt = receipt
						sameRejectedTree = 0
						treeHistory = append(treeHistory, lastTree)
						if len(treeHistory) > 4 {
							treeHistory = treeHistory[len(treeHistory)-4:]
						}
						if len(treeHistory) == 4 && treeHistory[0] == treeHistory[2] && treeHistory[1] == treeHistory[3] && treeHistory[0] != treeHistory[1] {
							outcome = loopOutcome{NoProgress: true, Reason: "agent oscillated between two rejected candidate states without progress"}
							receipt, err = b.RecordEscalation(outcome.Reason)
							lastReceipt = receipt
						}
					}
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
		fmt.Fprintf(cmd.OutOrStdout(), "Task:       %s\n", state.TaskID)
		if state.Goal != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Goal:       %s\n", state.Goal)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Status:     %s\nFreshness:  %s\nMode:       %s\nCoverage:   %s\n", state.Status, state.Freshness, state.Mode, state.Coverage)
		if state.StaleReason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Stale:      %s\n", state.StaleReason)
		}
		if state.Checkpoint != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Checkpoint: %s\nTree:       %s\n", state.Checkpoint.CheckpointID, short(state.Checkpoint.TreeSHA256))
		}
		if state.Receipt != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Receipt:    %s\n", state.Receipt.ReceiptID)
			if coverage := state.Receipt.VerificationCoverage; coverage != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Verifiers:  %s\n", verifierCoverageSummary(coverage.Verifiers))
				fmt.Fprintf(cmd.OutOrStdout(), "Uncovered:  %d declared risk(s)\n", len(coverage.Uncovered))
			}
			if impact := state.Receipt.LivenessImpact; impact != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Candidates: %d evaluated, %d rejected, %d checkpoint(s) verified\n", impact.CandidatesEvaluated, impact.CandidatesRejected, impact.CheckpointsVerified)
			}
			if state.Receipt.Recovered {
				fmt.Fprintf(cmd.OutOrStdout(), "Recovered:  yes (%s)\n", state.Receipt.SelectionReason)
			}
		}
		if state.AppliedCommit != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Applied:    %s", short(state.AppliedCommit))
			if state.AppliedBranch != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", state.AppliedBranch)
			}
			fmt.Fprintln(cmd.OutOrStdout())
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
		state, stateStore, err := loadState()
		if err != nil {
			return codedError{10, err}
		}
		if err := applyCheckpoint(state.RepoRoot, &state, branch); err != nil {
			var stale staleStateError
			if errors.As(err, &stale) {
				return codedError{3, err}
			}
			return codedError{11, err}
		}
		if err := persistAppliedState(stateStore, state); err != nil {
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
		if state.Receipt != nil {
			printCoverageSummary(cmd.OutOrStdout(), *state.Receipt, i18n.Detect())
		}
		return nil
	}}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a machine-readable explanation")
	return cmd
}

func nextAction(status string) string {
	switch status {
	case "ADMITTED":
		return "inspect with `seal diff`, then use `seal apply`"
	case "APPLIED":
		return "the verified checkpoint is on your branch; push it or open a pull request when ready"
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
		checks = append(checks, struct {
			name string
			err  error
		}{"checkpoint Git identity", identity.CheckpointIdentity(root)})
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
	plan := discoverVerificationPlan(root)
	return plan.Admission, strings.Join(plan.Detected, ", ")
}

type verificationPlan struct {
	Admission  []config.Check
	Completion []config.Check
	Detected   []string
	Uncovered  []string
}

func discoverVerificationPlan(root string) verificationPlan {
	check := func(id string, command ...string) config.Check {
		return config.Check{ID: id, Command: command, TimeoutSeconds: 900, Layer: "L1", Origin: "auto-discovered"}
	}
	var plan verificationPlan
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(root, name))
		return err == nil
	}
	switch {
	case has("go.mod"):
		plan.Admission = []config.Check{check("tests", "go", "test", "./...")}
		plan.Completion = append(append([]config.Check{}, plan.Admission...), check("static-analysis", "go", "vet", "./..."))
		plan.Detected = []string{"Go tests", "Go static analysis"}
	case has("package.json"):
		var manifest struct {
			Scripts map[string]string `json:"scripts"`
		}
		raw, _ := os.ReadFile(filepath.Join(root, "package.json"))
		_ = json.Unmarshal(raw, &manifest)
		packageManager := "npm"
		if has("pnpm-lock.yaml") || has("pnpm-workspace.yaml") {
			packageManager = "pnpm"
		} else if has("yarn.lock") {
			packageManager = "yarn"
		}
		for _, name := range []string{"test", "build", "lint", "typecheck"} {
			script := strings.TrimSpace(manifest.Scripts[name])
			if script == "" || name == "test" && strings.Contains(script, "no test specified") {
				continue
			}
			command := []string{packageManager, "run", name}
			if name == "test" {
				command = []string{packageManager, "test"}
			}
			c := check(name, command...)
			if len(plan.Admission) == 0 && name == "test" {
				plan.Admission = append(plan.Admission, c)
			}
			plan.Completion = append(plan.Completion, c)
			if name == "test" {
				plan.Detected = append(plan.Detected, packageManager+" test")
			} else {
				plan.Detected = append(plan.Detected, "Node "+name)
			}
		}
		if len(plan.Admission) == 0 && len(plan.Completion) > 0 {
			plan.Admission = []config.Check{plan.Completion[0]}
		}
		if len(plan.Completion) == 0 {
			plan.Uncovered = append(plan.Uncovered, "package.json declares no test, build, lint, or typecheck script")
		}
	case has("pyproject.toml") || has("pytest.ini") || has("setup.cfg"):
		pythonConfig, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
		setupConfig, _ := os.ReadFile(filepath.Join(root, "setup.cfg"))
		declaresPytest := has("pytest.ini") || strings.Contains(strings.ToLower(string(pythonConfig)), "pytest") || strings.Contains(strings.ToLower(string(setupConfig)), "pytest")
		if declaresPytest {
			plan.Admission = []config.Check{check("tests", "pytest")}
			plan.Detected = []string{"Python pytest suite"}
		} else if has("tests") {
			plan.Admission = []config.Check{check("tests", "python3", "-m", "unittest", "discover", "-s", "tests")}
			plan.Detected = []string{"Python unittest suite"}
		} else {
			plan.Admission = []config.Check{check("compile-check", "python3", "-m", "compileall", "-q", ".")}
			plan.Detected = []string{"Python compile check"}
			plan.Uncovered = append(plan.Uncovered, "no executable Python test suite was detected")
		}
		plan.Completion = append([]config.Check{}, plan.Admission...)
		plan.Uncovered = append(plan.Uncovered, "Python lint and type checks were not declared as portable project commands")
	case has("Cargo.toml"):
		plan.Admission = []config.Check{check("tests", "cargo", "test")}
		plan.Completion = append(append([]config.Check{}, plan.Admission...), check("compile-check", "cargo", "check"))
		plan.Detected = []string{"Rust tests", "Rust compile check"}
	case has("pom.xml"):
		plan.Admission = []config.Check{check("tests", "mvn", "test")}
		plan.Completion = append([]config.Check{}, plan.Admission...)
		plan.Detected = []string{"Maven tests"}
	case has("gradlew"):
		plan.Admission = []config.Check{check("tests", "./gradlew", "test")}
		plan.Completion = append([]config.Check{}, plan.Admission...)
		plan.Detected = []string{"Gradle tests"}
	}
	if len(plan.Admission) == 0 {
		fallback := config.Check{ID: "patch-integrity", Command: []string{"git", "diff", "--check"}, TimeoutSeconds: 60, Layer: "L1", Origin: "auto-discovered"}
		plan.Admission, plan.Completion = []config.Check{fallback}, []config.Check{fallback}
		plan.Detected = append(plan.Detected, "Git patch integrity")
		plan.Uncovered = append(plan.Uncovered, "no executable project test command was detected")
	}
	if has(".github/workflows") {
		plan.Uncovered = append(plan.Uncovered, "remote CI workflows are not executed by the local evaluator")
	}
	return plan
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

func agentCacheEnvironment(base []string, tempRoot, storeDir string) ([]string, error) {
	runtimeRoot := filepath.Join(tempRoot, "stateseal-agent", identity.Digest([]byte(storeDir))[:20])
	dirs := map[string]string{
		"GOCACHE":             filepath.Join(runtimeRoot, "go-build"),
		"GOTMPDIR":            filepath.Join(runtimeRoot, "go-tmp"),
		"PYTHONPYCACHEPREFIX": filepath.Join(runtimeRoot, "python-cache"),
		"npm_config_cache":    filepath.Join(runtimeRoot, "npm-cache"),
		"CARGO_TARGET_DIR":    filepath.Join(runtimeRoot, "cargo-target"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create managed Agent cache: %w", err)
		}
	}
	dirs["STATESEAL_RUNTIME_ROOT"] = runtimeRoot
	return mergeEnvironment(base, dirs), nil
}

func mergeEnvironment(base []string, overrides map[string]string) []string {
	environment := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		environment = append(environment, key+"="+overrides[key])
	}
	return environment
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
	activeTask, activeErr := store.ActiveTask(root)
	if activeErr != nil {
		if !os.IsNotExist(activeErr) {
			return protocol.TaskState{}, nil, activeErr
		}
		activeTask = p.Task.ID
	}
	s, err := store.Open(root, activeTask)
	if err != nil {
		return protocol.TaskState{}, nil, err
	}
	state, err := s.Load()
	if err == nil {
		state.Freshness, state.StaleReason = stateFreshness(root, rawPolicyDigest(root), state)
		if state.Freshness == "CURRENT" && state.Checkpoint != nil && state.Receipt != nil && state.Receipt.Verdict == protocol.VerdictAdmitted {
			head, headErr := identity.Git(root, "rev-parse", "HEAD")
			if headErr == nil && strings.TrimSpace(string(head)) == state.Checkpoint.Commit {
				state.Status = "APPLIED"
				state.AppliedCommit = state.Checkpoint.Commit
				if branch, branchErr := identity.Git(root, "branch", "--show-current"); branchErr == nil {
					state.AppliedBranch = strings.TrimSpace(string(branch))
				}
			}
		}
		if state.Freshness == "STALE" {
			if strings.Contains(state.StaleReason, "policy") {
				state.RuleID = protocol.RulePolicyChanged
			} else if strings.Contains(state.StaleReason, "trusted base") {
				state.RuleID = protocol.RuleTrustedBaseChanged
			} else if strings.Contains(state.StaleReason, "working tree") {
				state.RuleID = protocol.RuleCandidateMutated
			}
		}
	}
	return state, s, err
}

func applyCheckpoint(root string, state *protocol.TaskState, branch string) error {
	if state.Receipt == nil || state.Receipt.Verdict != protocol.VerdictAdmitted || state.Checkpoint == nil {
		return fmt.Errorf("only an admitted checkpoint can be applied")
	}
	policyDigest := rawPolicyDigest(root)
	freshness, reason := stateFreshness(root, policyDigest, *state)
	if freshness != "CURRENT" {
		return staleStateError{reason: "admission is stale: " + reason}
	}
	dirty, _ := identity.Git(root, "status", "--porcelain")
	if len(bytes.TrimSpace(dirty)) > 0 {
		return fmt.Errorf("current worktree is dirty")
	}
	if state.Checkpoint.Commit == state.BaseCommit {
		return markApplied(root, state)
	}
	if _, err := identity.Git(root, "merge-base", "--is-ancestor", state.BaseCommit, state.Checkpoint.Commit); err != nil {
		return fmt.Errorf("checkpoint is not descended from the trusted base: %w", err)
	}
	if branch != "" {
		if _, err := identity.Git(root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			return fmt.Errorf("branch %q already exists", branch)
		}
		if _, err := identity.Git(root, "switch", "-c", branch, state.Checkpoint.Commit); err != nil {
			return err
		}
		return markApplied(root, state)
	}
	if _, err := identity.Git(root, "merge", "--ff-only", state.Checkpoint.Commit); err != nil {
		return err
	}
	return markApplied(root, state)
}

func markApplied(root string, state *protocol.TaskState) error {
	head, err := identity.Git(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(string(head))
	if state.Checkpoint == nil || commit != state.Checkpoint.Commit {
		return fmt.Errorf("applied HEAD does not match the verified checkpoint")
	}
	branch, err := identity.Git(root, "branch", "--show-current")
	if err != nil {
		return err
	}
	state.AppliedCommit = commit
	state.AppliedBranch = strings.TrimSpace(string(branch))
	state.AppliedAt = time.Now().UTC()
	state.Status = "APPLIED"
	state.Freshness = "CURRENT"
	state.StaleReason = ""
	state.RuleID = ""
	return nil
}

func persistAppliedState(stateStore *store.Store, state protocol.TaskState) error {
	if err := stateStore.Save(state); err != nil {
		return err
	}
	_, err := stateStore.Append(protocol.Event{Type: "CHECKPOINT_APPLIED", TaskID: state.TaskID, Data: map[string]any{
		"checkpoint_id": state.Checkpoint.CheckpointID,
		"commit":        state.AppliedCommit,
		"branch":        state.AppliedBranch,
	}})
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
	dirty, err := identity.Git(root, "status", "--porcelain")
	if err != nil {
		return "UNKNOWN", "current Git state is unavailable"
	}
	if len(bytes.TrimSpace(dirty)) > 0 {
		return "STALE", "working tree changed after admission"
	}
	head, err := identity.Git(root, "rev-parse", "HEAD")
	if err != nil {
		return "UNKNOWN", "current Git state is unavailable"
	}
	current := strings.TrimSpace(string(head))
	if state.Checkpoint != nil && current == state.Checkpoint.Commit {
		return "CURRENT", ""
	}
	if current != state.BaseCommit {
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
	if r.VerificationCoverage != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Coverage: %s; %s\n", r.VerificationCoverage.Observation, verifierCoverageSummary(r.VerificationCoverage.Verifiers))
	}
	if r.LivenessImpact != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Delivery impact: %d candidate(s) evaluated, %d rejected, %d checkpoint(s) verified\n",
			r.LivenessImpact.CandidatesEvaluated, r.LivenessImpact.CandidatesRejected, r.LivenessImpact.CheckpointsVerified)
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

func printRunResult(w io.Writer, r protocol.CompletionReceipt, state protocol.TaskState, locale i18n.Locale, snapshot progressSnapshot) {
	changed := verifiedChangedFiles(state)
	if snapshot.Changed > changed {
		changed = snapshot.Changed
	}
	attempts := snapshot.Attempts
	if attempts == 0 {
		attempts = 1
	}
	checks := len(r.CompletionEvidence)
	duration := snapshot.CompletedAt.Sub(snapshot.StartedAt)

	fmt.Fprintf(w, "\n%s\n", locale.T(i18n.ResultTitle))
	switch r.Verdict {
	case protocol.VerdictAdmitted:
		fmt.Fprintln(w, locale.T(i18n.ResultPassed))
	case protocol.VerdictRejected:
		fmt.Fprintln(w, locale.T(i18n.ResultRejected))
	case protocol.VerdictAbstained:
		fmt.Fprintln(w, locale.T(i18n.ResultAbstained))
	default:
		fmt.Fprintln(w, locale.T(i18n.ResultStatus, r.Verdict))
	}
	printResultField(w, locale, locale.T(i18n.ChangedFilesLabel), fmt.Sprint(changed))
	printResultField(w, locale, locale.T(i18n.AgentAttemptsLabel), fmt.Sprint(attempts))
	printResultField(w, locale, locale.T(i18n.ChecksLabel), fmt.Sprint(checks))
	printResultField(w, locale, locale.T(i18n.CoverageLabel), coverageLabel(state.Coverage, locale))
	printResultField(w, locale, locale.T(i18n.DurationLabel), formatDuration(duration))
	if len(verifiedChangedFileNames(state)) > 0 {
		printResultField(w, locale, integrationText(locale, "Files", "文件"), compactFileList(verifiedChangedFileNames(state)))
	}
	if r.ReceiptID != "" {
		printResultField(w, locale, integrationText(locale, "Receipt", "Receipt"), r.ReceiptID)
	}
	if r.TreeSHA256 != "" {
		printResultField(w, locale, integrationText(locale, "Code state", "代码状态"), short(r.TreeSHA256))
	}
	if r.Reason != "" && r.Verdict != protocol.VerdictAdmitted {
		printResultField(w, locale, locale.T(i18n.ReasonLabel), compactFailure(r.Reason))
	}
	if r.Verdict == protocol.VerdictAdmitted {
		fmt.Fprintf(w, "\n%s\n", locale.T(i18n.DeliveryBasis))
		for _, key := range []i18n.Key{i18n.BasisIsolated, i18n.BasisExternal, i18n.BasisExactState, i18n.BasisRecertified} {
			fmt.Fprintf(w, "  ✓ %s\n", locale.T(key))
		}
	}
	printCoverageSummary(w, r, locale)
	if len(r.ResidualRisks) > 0 {
		fmt.Fprintf(w, "\n%s\n", locale.T(i18n.ResidualRiskLabel))
		for _, risk := range localizedRisks(r.ResidualRisks, locale) {
			fmt.Fprintf(w, "  • %s\n", risk)
		}
	}
}

func printResultField(w io.Writer, locale i18n.Locale, label, value string) {
	separator := ":"
	if locale.IsChinese() {
		separator = "："
	}
	fmt.Fprintf(w, "  %s%s %s\n", label, separator, value)
}

func verifiedChangedFiles(state protocol.TaskState) int {
	if state.Checkpoint == nil {
		return 0
	}
	out, err := identity.Git(state.RepoRoot, "diff", "--name-only", state.BaseCommit, state.Checkpoint.Commit)
	if err != nil {
		return 0
	}
	return len(strings.Fields(strings.TrimSpace(string(out))))
}

func printQuietRunResult(w io.Writer, r protocol.CompletionReceipt, state protocol.TaskState, locale i18n.Locale) {
	message := locale.T(i18n.ResultStatus, r.Verdict)
	if r.Verdict == protocol.VerdictAdmitted {
		message = locale.T(i18n.ResultPassed)
	} else if r.Verdict == protocol.VerdictRejected {
		message = locale.T(i18n.ResultRejected)
	} else if r.Verdict == protocol.VerdictAbstained {
		message = locale.T(i18n.ResultAbstained)
	}
	branch := ""
	if state.AppliedBranch != "" {
		branch = " · " + state.AppliedBranch
	}
	fmt.Fprintf(w, "%s · %d %s · %d %s%s\n", message, verifiedChangedFiles(state), locale.T(i18n.ChangedFilesLabel), len(r.CompletionEvidence), locale.T(i18n.ChecksLabel), branch)
}

type jsonRunResult struct {
	Verdict              protocol.Verdict               `json:"verdict"`
	TaskID               string                         `json:"task_id"`
	ReceiptID            string                         `json:"receipt_id,omitempty"`
	ChangedFiles         int                            `json:"changed_files"`
	Attempts             int                            `json:"attempts"`
	ChecksPassed         int                            `json:"checks_passed"`
	Coverage             string                         `json:"coverage"`
	DurationMS           int64                          `json:"duration_ms"`
	Applied              bool                           `json:"applied"`
	Branch               string                         `json:"branch,omitempty"`
	Reason               string                         `json:"reason,omitempty"`
	ResidualRisk         []string                       `json:"residual_risks,omitempty"`
	VerificationCoverage *protocol.VerificationCoverage `json:"verification_coverage,omitempty"`
	LivenessImpact       *protocol.LivenessImpact       `json:"liveness_impact,omitempty"`
}

func printJSONRunResult(w io.Writer, r protocol.CompletionReceipt, state protocol.TaskState, snapshot progressSnapshot) error {
	attempts := snapshot.Attempts
	if attempts == 0 {
		attempts = 1
	}
	return json.NewEncoder(w).Encode(jsonRunResult{
		Verdict: r.Verdict, TaskID: state.TaskID, ReceiptID: r.ReceiptID,
		ChangedFiles: verifiedChangedFiles(state), Attempts: attempts,
		ChecksPassed: len(r.CompletionEvidence), Coverage: state.Coverage,
		DurationMS: snapshot.CompletedAt.Sub(snapshot.StartedAt).Milliseconds(),
		Applied:    state.AppliedCommit != "", Branch: state.AppliedBranch,
		Reason: r.Reason, ResidualRisk: r.ResidualRisks,
		VerificationCoverage: r.VerificationCoverage, LivenessImpact: r.LivenessImpact,
	})
}

func printCoverageSummary(w io.Writer, r protocol.CompletionReceipt, locale i18n.Locale) {
	coverage := r.VerificationCoverage
	if coverage == nil {
		return
	}
	fmt.Fprintf(w, "\n%s\n", integrationText(locale, "Verification coverage", "验证覆盖"))
	fmt.Fprintf(w, "  L0 · %s\n", integrationText(locale,
		"state, policy, freshness, checkpoint, and receipt integrity",
		"代码状态、策略、新鲜度、Checkpoint 与 Receipt 完整性"))
	if len(coverage.Verifiers) > 0 {
		fmt.Fprintf(w, "  %s\n", verifierCoverageSummaryLocalized(coverage.Verifiers, locale))
	}
	if impact := r.LivenessImpact; impact != nil {
		if locale.IsChinese() {
			fmt.Fprintf(w, "  交付影响：评估 %d 个候选 · 拒绝 %d 个 · 验证 %d 个 Checkpoint\n",
				impact.CandidatesEvaluated, impact.CandidatesRejected, impact.CheckpointsVerified)
		} else {
			fmt.Fprintf(w, "  Delivery impact: %d evaluated · %d rejected · %d checkpoints verified\n",
				impact.CandidatesEvaluated, impact.CandidatesRejected, impact.CheckpointsVerified)
		}
	}
}

func verifierCoverageSummaryLocalized(verifiers []protocol.VerifierCoverage, locale i18n.Locale) string {
	if !locale.IsChinese() {
		return verifierCoverageSummary(verifiers)
	}
	groups := make([]string, 0, len(verifiers))
	seen := map[string]bool{}
	for _, verifier := range verifiers {
		origin := verifier.Origin
		switch origin {
		case "auto-discovered":
			origin = "自动发现"
		case "project-policy":
			origin = "项目策略"
		case "external":
			origin = "外部权威"
		}
		key := verifier.Layer + " · " + origin
		if !seen[key] {
			seen[key] = true
			groups = append(groups, key)
		}
	}
	return strings.Join(groups, "；") + fmt.Sprintf(" · %d 项检查", len(verifiers))
}

func verifierCoverageSummary(verifiers []protocol.VerifierCoverage) string {
	if len(verifiers) == 0 {
		return "no command verifier evidence"
	}
	groups := make([]string, 0, len(verifiers))
	seen := map[string]bool{}
	for _, verifier := range verifiers {
		key := verifier.Layer + " · " + verifier.Origin
		if seen[key] {
			continue
		}
		seen[key] = true
		groups = append(groups, key)
	}
	return strings.Join(groups, "; ") + fmt.Sprintf(" · %d check(s)", len(verifiers))
}

func localizedRisks(risks []string, locale i18n.Locale) []string {
	result := make([]string, 0, len(risks))
	for _, risk := range risks {
		trimmed := strings.TrimSpace(strings.TrimSuffix(risk, "."))
		switch trimmed {
		case "Only configured checks were evaluated":
			result = append(result, locale.T(i18n.RiskConfiguredOnly))
		case "The execution host was not independently attested":
			result = append(result, locale.T(i18n.RiskHostUnattested))
		case "remote CI workflows are not executed by the local evaluator":
			result = append(result, locale.T(i18n.RiskRemoteCI))
		default:
			result = append(result, risk)
		}
	}
	return result
}

func coverageLabel(coverage string, locale i18n.Locale) string {
	if coverage == "intermediate + terminal" {
		return locale.T(i18n.CoverageFull)
	}
	if coverage == "terminal-only" || coverage == "" {
		return locale.T(i18n.CoverageTerminal)
	}
	return coverage
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
	if strings.Contains(base, "gemini") {
		return "gemini-adapter"
	}
	if strings.Contains(base, "cursor") {
		return "cursor-adapter"
	}
	if strings.Contains(base, "copilot") {
		return "copilot-adapter"
	}
	return "command-adapter"
}
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
