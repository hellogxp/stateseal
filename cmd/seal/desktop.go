package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/spf13/cobra"
)

type desktopCommandResult struct {
	Workspace            string                         `json:"workspace,omitempty"`
	Repository           string                         `json:"repository,omitempty"`
	Agent                string                         `json:"agent,omitempty"`
	ExecutionMode        string                         `json:"execution_mode,omitempty"`
	DeliveryStages       []string                       `json:"delivery_stages,omitempty"`
	SessionID            string                         `json:"session_id,omitempty"`
	Stage                string                         `json:"stage"`
	Verdict              protocol.Verdict               `json:"verdict,omitempty"`
	TaskID               string                         `json:"task_id,omitempty"`
	ReceiptID            string                         `json:"receipt_id,omitempty"`
	ChangedFiles         int                            `json:"changed_files,omitempty"`
	Files                []string                       `json:"files,omitempty"`
	Attempts             int                            `json:"attempts,omitempty"`
	ChecksPassed         int                            `json:"checks_passed,omitempty"`
	Checks               []desktopCheckResult           `json:"checks,omitempty"`
	Coverage             string                         `json:"coverage,omitempty"`
	CodeState            string                         `json:"code_state,omitempty"`
	DurationMS           int64                          `json:"duration_ms,omitempty"`
	Timeline             []machineProgressEvent         `json:"timeline,omitempty"`
	RuleID               string                         `json:"rule_id,omitempty"`
	Branch               string                         `json:"branch,omitempty"`
	Reason               string                         `json:"reason,omitempty"`
	ResidualRisks        []string                       `json:"residual_risks,omitempty"`
	VerificationCoverage *protocol.VerificationCoverage `json:"verification_coverage,omitempty"`
	LivenessImpact       *protocol.LivenessImpact       `json:"liveness_impact,omitempty"`
	AuthorityStatus      string                         `json:"authority_status,omitempty"`
	ReasonCode           string                         `json:"reason_code,omitempty"`
	Retryable            bool                           `json:"retryable,omitempty"`
	SafeState            *mcpSafeState                  `json:"safe_state,omitempty"`
	AllowedActions       []string                       `json:"allowed_actions,omitempty"`
	Unverified           bool                           `json:"unverified,omitempty"`
	NextAction           string                         `json:"next_action"`
}

type desktopCheckResult struct {
	ID         string `json:"id"`
	Command    string `json:"command,omitempty"`
	Phase      string `json:"phase"`
	Layer      string `json:"layer,omitempty"`
	Origin     string `json:"origin,omitempty"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	EvidenceID string `json:"evidence_id,omitempty"`
}

func desktopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Coordinate verified delivery from an existing Agent desktop session",
	}
	cmd.AddCommand(desktopRunCmd(), desktopApplyCmd(), desktopRejectCmd(), desktopStatusCmd(), desktopRecoverCmd())
	return cmd
}

func desktopRunCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the goal bound to a Desktop session in an isolated Agent process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := runDesktopSession(cmd.Context(), sessionID, nil)
			if err != nil {
				return codedError{11, err}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func runDesktopSession(ctx context.Context, sessionID string, onProgress func(machineProgressEvent)) (desktopCommandResult, error) {
	session, err := store.LoadDesktopSession(sessionID)
	if err != nil {
		return desktopCommandResult{}, err
	}
	if session.Stage != store.DesktopStageReady && session.Stage != store.DesktopStageFailed {
		return desktopCommandResult{}, fmt.Errorf("Desktop session is %s, not ready to run", session.Stage)
	}
	unlock, err := store.LockDesktopRepository(session.RepoRoot, session.SessionID)
	if err != nil {
		return desktopCommandResult{}, err
	}
	defer unlock()

	session.Stage, session.LastError = store.DesktopStageRunning, ""
	if err := store.SaveDesktopSession(session); err != nil {
		return desktopCommandResult{}, err
	}
	binary, err := sealExecutable()
	if err != nil {
		return desktopCommandResult{}, err
	}
	progressFile, err := os.CreateTemp("", "stateseal-desktop-progress-*.jsonl")
	if err != nil {
		return desktopCommandResult{}, err
	}
	progressPath := progressFile.Name()
	_ = progressFile.Close()
	defer os.Remove(progressPath)

	args := []string{
		"run", "--agent", session.Agent, "--yes", "--no-apply", "--json", "--require-change",
		"--source", session.Agent + "-desktop-mcp", "--task-id", session.TaskID, session.Goal,
	}
	child := exec.CommandContext(ctx, binary, args...)
	child.Dir = session.RepoRoot
	child.Env = append(os.Environ(),
		"STATESEAL_DESKTOP_MCP_CHILD=1",
		"STATESEAL_PROGRESS_FILE="+progressPath,
	)
	var stdout, stderr bytes.Buffer
	timeline := make([]machineProgressEvent, 0, 16)
	publish := func(event machineProgressEvent) {
		timeline = append(timeline, event)
		if onProgress != nil {
			onProgress(event)
		}
	}
	child.Stdout, child.Stderr = &stdout, &stderr
	runErr := child.Start()
	if runErr == nil {
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()
		lastSequence := 0
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case runErr = <-done:
				publishProgressFile(progressPath, &lastSequence, publish)
				goto completed
			case <-ticker.C:
				publishProgressFile(progressPath, &lastSequence, publish)
			}
		}
	}

completed:
	var result jsonRunResult
	decodeErr := json.Unmarshal(stdout.Bytes(), &result)
	validResult := result.TaskID == session.TaskID && validDesktopVerdict(result.Verdict)
	if decodeErr != nil || !validResult {
		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = strings.TrimSpace(stdout.String())
		}
		if reason == "" && runErr != nil {
			reason = runErr.Error()
		}
		if reason == "" {
			reason = "managed child returned an invalid or mismatched result"
		}
		session.Stage, session.LastError = store.DesktopStageFailed, reason
		_ = store.SaveDesktopSession(session)
		return desktopCommandResult{
			SessionID: session.SessionID, Stage: session.Stage, TaskID: session.TaskID, Reason: reason,
			NextAction: "report the failure to the user; do not edit or apply unverified code",
		}, nil
	}
	session.Verdict = string(result.Verdict)
	session.ReceiptID = result.ReceiptID
	session.LastError = result.Reason
	if result.Verdict == protocol.VerdictAdmitted {
		session.Stage = store.DesktopStagePendingApply
	} else {
		session.Stage = store.DesktopStageFailed
	}
	if err := store.SaveDesktopSession(session); err != nil {
		return desktopCommandResult{}, err
	}
	state, _ := loadDesktopTaskState(session)
	files := verifiedChangedFileNames(state)
	checks := desktopCheckResults(session.RepoRoot, state)
	next := "explain why the candidate was not admitted; do not apply it"
	if session.Stage == store.DesktopStagePendingApply {
		next = "present the verification summary and request native approval for apply_verified"
	}
	return desktopCommandResult{
		SessionID: session.SessionID, Stage: session.Stage, Verdict: result.Verdict,
		TaskID: result.TaskID, ReceiptID: result.ReceiptID, ChangedFiles: result.ChangedFiles,
		Files: files, Attempts: result.Attempts, Checks: checks, CodeState: resultCodeState(state),
		DurationMS: result.DurationMS, Timeline: timeline, RuleID: resultRuleID(state),
		ChecksPassed: result.ChecksPassed, Coverage: result.Coverage, Reason: result.Reason,
		ResidualRisks: result.ResidualRisk, VerificationCoverage: result.VerificationCoverage,
		LivenessImpact: result.LivenessImpact, NextAction: next,
	}, nil
}

func loadDesktopTaskState(session store.DesktopSession) (protocol.TaskState, error) {
	stateStore, err := store.Open(session.RepoRoot, session.TaskID)
	if err != nil {
		return protocol.TaskState{}, err
	}
	return stateStore.Load()
}

func verifiedChangedFileNames(state protocol.TaskState) []string {
	if state.Checkpoint == nil {
		return nil
	}
	out, err := identity.Git(state.RepoRoot, "diff", "--name-only", state.BaseCommit, state.Checkpoint.Commit)
	if err != nil {
		return nil
	}
	return strings.Fields(strings.TrimSpace(string(out)))
}

func desktopCheckResults(root string, state protocol.TaskState) []desktopCheckResult {
	if state.Receipt == nil || state.Receipt.VerificationCoverage == nil {
		return nil
	}
	commands := map[string]string{}
	if policy, _, err := config.Load(root); err == nil {
		for _, check := range append(append([]config.Check(nil), policy.Admission.Checks...), policy.Completion.Checks...) {
			commands[check.ID] = shellJoin(check.Command)
		}
	}
	evidence := map[string]protocol.EvidenceEnvelope{}
	for _, item := range state.Evidence {
		evidence[item.EvidenceID] = item
	}
	results := make([]desktopCheckResult, 0, len(state.Receipt.VerificationCoverage.Verifiers))
	for _, item := range state.Receipt.VerificationCoverage.Verifiers {
		result := desktopCheckResult{ID: item.CheckID, Command: commands[item.CheckID], Phase: item.Phase, Layer: item.Layer, Origin: item.Origin, Status: item.Status, EvidenceID: item.EvidenceID}
		if envelope, ok := evidence[item.EvidenceID]; ok {
			result.DurationMS = envelope.FinishedAt.Sub(envelope.StartedAt).Milliseconds()
		}
		results = append(results, result)
	}
	return results
}

func resultCodeState(state protocol.TaskState) string {
	if state.Receipt != nil {
		return state.Receipt.TreeSHA256
	}
	return ""
}

func resultRuleID(state protocol.TaskState) string {
	if state.Receipt != nil {
		return state.Receipt.RuleID
	}
	return state.RuleID
}

func validDesktopVerdict(verdict protocol.Verdict) bool {
	return verdict == protocol.VerdictAdmitted || verdict == protocol.VerdictRejected ||
		verdict == protocol.VerdictAbstained || verdict == protocol.VerdictStale ||
		verdict == protocol.VerdictEscalated
}

func publishProgressFile(path string, lastSequence *int, publish func(machineProgressEvent)) {
	if publish == nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event machineProgressEvent
		if json.Unmarshal(line, &event) == nil && event.Sequence > *lastSequence {
			*lastSequence = event.Sequence
			publish(event)
		}
	}
}

func desktopApplyCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the exact verified checkpoint after explicit user acceptance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := applyDesktopSession(sessionID, "")
			if err != nil {
				return codedError{11, err}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func applyDesktopSession(sessionID, expectedReceiptID string) (desktopCommandResult, error) {
	session, err := store.LoadDesktopSession(sessionID)
	if err != nil {
		return desktopCommandResult{}, err
	}
	if session.Stage != store.DesktopStagePendingApply || session.Verdict != string(protocol.VerdictAdmitted) {
		return desktopCommandResult{}, fmt.Errorf("Desktop session has no admitted checkpoint awaiting acceptance")
	}
	if expectedReceiptID != "" && expectedReceiptID != session.ReceiptID {
		return desktopCommandResult{}, fmt.Errorf("receipt %q does not match the admitted Desktop session", expectedReceiptID)
	}
	unlock, err := store.LockDesktopRepository(session.RepoRoot, session.SessionID)
	if err != nil {
		return desktopCommandResult{}, err
	}
	defer unlock()
	stateStore, err := store.Open(session.RepoRoot, session.TaskID)
	if err != nil {
		return desktopCommandResult{}, err
	}
	state, err := stateStore.Load()
	if err != nil {
		return desktopCommandResult{}, err
	}
	if state.TaskID != session.TaskID || state.RepoRoot != session.RepoRoot || state.Receipt == nil || state.Receipt.ReceiptID != session.ReceiptID {
		return desktopCommandResult{}, fmt.Errorf("Desktop session does not match the exact admitted task state")
	}
	branch := defaultDeliveryBranch(session.RepoRoot, session.TaskID)
	if err := applyCheckpoint(session.RepoRoot, &state, branch); err != nil {
		// Keep the admitted checkpoint recoverable. Apply is a separate
		// transaction and a runtime failure must not erase delivery authority.
		session.Stage, session.LastError = store.DesktopStagePendingApply, err.Error()
		_ = store.SaveDesktopSession(session)
		return desktopCommandResult{}, fmt.Errorf("apply verified checkpoint: %s", session.LastError)
	}
	if err := persistAppliedState(stateStore, state); err != nil {
		return desktopCommandResult{}, err
	}
	session.Stage, session.LastError = store.DesktopStageApplied, ""
	if err := store.SaveDesktopSession(session); err != nil {
		return desktopCommandResult{}, err
	}
	return desktopCommandResult{
		SessionID: session.SessionID, Stage: session.Stage, Verdict: protocol.VerdictAdmitted,
		TaskID: session.TaskID, ReceiptID: session.ReceiptID, Branch: state.AppliedBranch,
		AuthorityStatus: "healthy",
		NextAction:      "tell the user that the exact verified checkpoint is now applied on the feature branch",
	}, nil
}

func desktopRejectCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{Use: "reject", Short: "Reject a pending Desktop delivery without changing the user branch", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := rejectDesktopSession(sessionID)
		if err != nil {
			return codedError{11, err}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
	}}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func rejectDesktopSession(sessionID string) (desktopCommandResult, error) {
	session, err := store.LoadDesktopSession(sessionID)
	if err != nil {
		return desktopCommandResult{}, err
	}
	if session.Stage == store.DesktopStageApplied {
		return desktopCommandResult{}, fmt.Errorf("an applied Desktop delivery cannot be rejected")
	}
	session.Stage = store.DesktopStageRejected
	if err := store.SaveDesktopSession(session); err != nil {
		return desktopCommandResult{}, err
	}
	return desktopCommandResult{
		SessionID: session.SessionID, Stage: session.Stage, TaskID: session.TaskID,
		NextAction: "confirm that no code was applied",
	}, nil
}

func desktopStatusCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{Use: "status", Short: "Show the durable StateSeal state for a Desktop session", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		session, err := store.LoadDesktopSession(sessionID)
		if err != nil {
			return codedError{10, err}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(session)
	}}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func desktopRecoverCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{Use: "recover", Short: "Recover a Desktop session after an interrupted parent Agent turn", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		session, err := store.LoadDesktopSession(sessionID)
		if err != nil {
			return codedError{10, err}
		}
		if session.TaskID != "" {
			stateStore, openErr := store.Open(session.RepoRoot, session.TaskID)
			if openErr == nil {
				state, loadErr := stateStore.Load()
				if loadErr == nil && state.Receipt != nil {
					session.Verdict, session.ReceiptID = string(state.Receipt.Verdict), state.Receipt.ReceiptID
					if state.Receipt.Verdict == protocol.VerdictAdmitted && state.AppliedCommit == "" {
						session.Stage = store.DesktopStagePendingApply
					} else if state.AppliedCommit != "" {
						session.Stage = store.DesktopStageApplied
					} else {
						session.Stage = store.DesktopStageFailed
					}
				}
			}
		}
		if session.Stage == store.DesktopStageRunning {
			session.Stage = store.DesktopStageFailed
			session.LastError = "the managed child run ended without durable completion evidence"
		}
		if err := store.SaveDesktopSession(session); err != nil {
			return codedError{11, err}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(session)
	}}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
