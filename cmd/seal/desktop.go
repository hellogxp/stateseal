package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/spf13/cobra"
)

type desktopCommandResult struct {
	Stage         string           `json:"stage"`
	Verdict       protocol.Verdict `json:"verdict,omitempty"`
	TaskID        string           `json:"task_id,omitempty"`
	ReceiptID     string           `json:"receipt_id,omitempty"`
	ChangedFiles  int              `json:"changed_files,omitempty"`
	ChecksPassed  int              `json:"checks_passed,omitempty"`
	Coverage      string           `json:"coverage,omitempty"`
	Branch        string           `json:"branch,omitempty"`
	Reason        string           `json:"reason,omitempty"`
	ResidualRisks []string         `json:"residual_risks,omitempty"`
	NextAction    string           `json:"next_action"`
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
			session, err := store.LoadDesktopSession(sessionID)
			if err != nil {
				return codedError{10, err}
			}
			if session.Stage != store.DesktopStageReady && session.Stage != store.DesktopStageFailed {
				return codedError{10, fmt.Errorf("Desktop session is %s, not ready to run", session.Stage)}
			}
			unlock, err := store.LockDesktopRepository(session.RepoRoot, session.SessionID)
			if err != nil {
				return codedError{10, err}
			}
			defer unlock()

			session.Stage, session.LastError = store.DesktopStageRunning, ""
			if err := store.SaveDesktopSession(session); err != nil {
				return codedError{11, err}
			}
			binary, err := sealExecutable()
			if err != nil {
				return codedError{11, err}
			}
			args := []string{"run", "--agent", session.Agent, "--yes", "--no-apply", "--json", "--task-id", session.TaskID, session.Goal}
			child := exec.Command(binary, args...)
			child.Dir = session.RepoRoot
			child.Env = append(os.Environ(), "STATESEAL_DESKTOP_CHILD=1")
			var stdout, stderr bytes.Buffer
			child.Stdout, child.Stderr = &stdout, &stderr
			runErr := child.Run()

			var result jsonRunResult
			decodeErr := json.Unmarshal(stdout.Bytes(), &result)
			validResult := result.TaskID == session.TaskID && (result.Verdict == protocol.VerdictAdmitted || result.Verdict == protocol.VerdictRejected || result.Verdict == protocol.VerdictAbstained || result.Verdict == protocol.VerdictStale || result.Verdict == protocol.VerdictEscalated)
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
				return json.NewEncoder(cmd.OutOrStdout()).Encode(desktopCommandResult{
					Stage: session.Stage, TaskID: session.TaskID, Reason: reason,
					NextAction: "report the failure to the user; do not edit or apply unverified code",
				})
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
				return codedError{11, err}
			}
			next := "explain why the candidate was not admitted; do not apply it"
			if session.Stage == store.DesktopStagePendingApply {
				next = "present the verification summary and ask the user whether to apply the exact verified checkpoint"
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(desktopCommandResult{
				Stage: session.Stage, Verdict: result.Verdict, TaskID: result.TaskID, ReceiptID: result.ReceiptID,
				ChangedFiles: result.ChangedFiles, ChecksPassed: result.ChecksPassed, Coverage: result.Coverage,
				Reason: result.Reason, ResidualRisks: result.ResidualRisk, NextAction: next,
			})
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func desktopApplyCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the exact verified checkpoint after explicit user acceptance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			session, err := store.LoadDesktopSession(sessionID)
			if err != nil {
				return codedError{10, err}
			}
			if session.Stage != store.DesktopStagePendingApply || session.Verdict != string(protocol.VerdictAdmitted) {
				return codedError{10, fmt.Errorf("Desktop session has no admitted checkpoint awaiting acceptance")}
			}
			unlock, err := store.LockDesktopRepository(session.RepoRoot, session.SessionID)
			if err != nil {
				return codedError{10, err}
			}
			defer unlock()
			stateStore, err := store.Open(session.RepoRoot, session.TaskID)
			if err != nil {
				return codedError{11, err}
			}
			state, err := stateStore.Load()
			if err != nil {
				return codedError{11, err}
			}
			if state.TaskID != session.TaskID || state.RepoRoot != session.RepoRoot || state.Receipt == nil || state.Receipt.ReceiptID != session.ReceiptID {
				return codedError{11, fmt.Errorf("Desktop session does not match the exact admitted task state")}
			}
			branch := defaultDeliveryBranch(session.RepoRoot, session.TaskID)
			if err := applyCheckpoint(session.RepoRoot, &state, branch); err != nil {
				session.Stage, session.LastError = store.DesktopStageFailed, err.Error()
				_ = store.SaveDesktopSession(session)
				return codedError{11, fmt.Errorf("apply verified checkpoint: %s", session.LastError)}
			}
			if err := persistAppliedState(stateStore, state); err != nil {
				return codedError{11, err}
			}
			session.Stage, session.LastError = store.DesktopStageApplied, ""
			if err := store.SaveDesktopSession(session); err != nil {
				return codedError{11, err}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(desktopCommandResult{
				Stage: session.Stage, Verdict: protocol.VerdictAdmitted, TaskID: session.TaskID,
				ReceiptID: session.ReceiptID, Branch: branch,
				NextAction: "tell the user that the verified checkpoint is now applied on the feature branch",
			})
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func desktopRejectCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{Use: "reject", Short: "Reject a pending Desktop delivery without changing the user branch", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		session, err := store.LoadDesktopSession(sessionID)
		if err != nil {
			return codedError{10, err}
		}
		session.Stage = store.DesktopStageRejected
		if err := store.SaveDesktopSession(session); err != nil {
			return codedError{11, err}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(desktopCommandResult{Stage: session.Stage, TaskID: session.TaskID, NextAction: "confirm that no code was applied"})
	}}
	cmd.Flags().StringVar(&sessionID, "session", "", "Desktop Agent session identifier")
	_ = cmd.MarkFlagRequired("session")
	return cmd
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

func runCodexDesktopHook(cmd *cobra.Command, eventName string, event map[string]any) (bool, error) {
	if os.Getenv("STATESEAL_DESKTOP_CHILD") == "1" || os.Getenv("STATESEAL_TASK_ID") != "" {
		return false, nil
	}
	sessionID, _ := event["session_id"].(string)
	if sessionID == "" {
		return false, nil
	}
	cwd, _ := event["cwd"].(string)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	root, err := identity.GitRoot(cwd)
	if err != nil {
		return false, nil
	}
	session, loadErr := store.LoadDesktopSession(sessionID)
	if loadErr == nil && session.RepoRoot != root {
		return false, nil
	}

	switch eventName {
	case "UserPromptSubmit":
		prompt, _ := event["prompt"].(string)
		goal, starts := desktopGoal(prompt)
		if starts {
			if loadErr == nil && !desktopTerminalStage(session.Stage) {
				return true, codexDesktopContext(cmd, eventName, desktopExistingTaskContext(session))
			}
			turnID, _ := event["turn_id"].(string)
			session = store.DesktopSession{
				SessionID: sessionID, TurnID: turnID, Agent: "codex", RepoRoot: root,
				Goal: goal, TaskID: newTaskID(goal, time.Now().UTC()), Stage: store.DesktopStageReady,
			}
			if !desktopProjectReady(root, "codex") {
				session.Stage = store.DesktopStageSetupPending
			}
			if err := store.SaveDesktopSession(session); err != nil {
				return true, agentHookMessage("codex", cmd, "StateSeal could not create Desktop authority state: "+err.Error(), false, eventName)
			}
			if session.Stage == store.DesktopStageSetupPending {
				return true, codexDesktopContext(cmd, eventName, desktopSetupContext(session))
			}
			return true, codexDesktopContext(cmd, eventName, desktopRunContext(session))
		}
		if loadErr != nil || desktopTerminalStage(session.Stage) {
			return false, nil
		}
		if session.Stage == store.DesktopStageSetupPending {
			if desktopAffirmative(prompt) {
				session.Stage = store.DesktopStageReady
				if err := store.SaveDesktopSession(session); err != nil {
					return true, err
				}
				return true, codexDesktopContext(cmd, eventName, desktopRunContext(session))
			}
			if desktopNegative(prompt) {
				session.Stage = store.DesktopStageRejected
				_ = store.SaveDesktopSession(session)
				return true, codexDesktopContext(cmd, eventName, "The user declined StateSeal project setup. Do not modify the repository. Confirm that nothing was changed.")
			}
			return true, codexDesktopContext(cmd, eventName, desktopSetupContext(session))
		}
		if session.Stage == store.DesktopStagePendingApply {
			if desktopAffirmative(prompt) {
				return true, codexDesktopContext(cmd, eventName, desktopApplyContext(session))
			}
			if desktopNegative(prompt) {
				session.Stage = store.DesktopStageRejected
				_ = store.SaveDesktopSession(session)
				return true, codexDesktopContext(cmd, eventName, "The user rejected the verified candidate. Do not apply it. Confirm that the user branch is unchanged.")
			}
			return true, codexDesktopContext(cmd, eventName, desktopPendingContext(session))
		}
		return true, codexDesktopContext(cmd, eventName, desktopExistingTaskContext(session))

	case "SessionStart":
		if loadErr == nil && !desktopTerminalStage(session.Stage) {
			return true, codexDesktopContext(cmd, eventName, "StateSeal recovered an active Desktop delivery. "+desktopExistingTaskContext(session))
		}
	case "PreToolUse":
		if loadErr == nil && !desktopTerminalStage(session.Stage) {
			if desktopToolAllowed(event, session) {
				return true, nil
			}
			return true, codexDesktopToolDenied(cmd, desktopExpectedCommand(session))
		}
	case "PostToolUse":
		if loadErr == nil && !desktopTerminalStage(session.Stage) {
			refreshed, refreshErr := store.LoadDesktopSession(sessionID)
			if refreshErr == nil {
				session = refreshed
			}
			if session.Stage == store.DesktopStagePendingApply {
				return true, codexDesktopContext(cmd, eventName, desktopPendingContext(session))
			}
			if session.Stage == store.DesktopStageFailed {
				return true, codexDesktopContext(cmd, eventName, "StateSeal did not admit the candidate: "+session.LastError+". Report this result; do not edit the source worktree.")
			}
		}
	case "Stop":
		if loadErr == nil && session.Stage == store.DesktopStageReady {
			message := "StateSeal has not started the controlled delivery. Execute exactly: " + desktopExpectedCommand(session)
			return true, agentHookMessage("codex", cmd, message, true, eventName)
		}
	}
	return false, nil
}

func runCodexDesktopCommandHook(cmd *cobra.Command, args []string) error {
	eventName := ""
	if len(args) == 1 {
		eventName = args[0]
	}
	raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
	if err != nil {
		return agentHookMessage("codex", cmd, "StateSeal could not read the Desktop lifecycle event: "+err.Error(), false, eventName)
	}
	var event map[string]any
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &event); err != nil {
			return agentHookMessage("codex", cmd, "StateSeal received a malformed Desktop lifecycle event.", false, eventName)
		}
	}
	if eventName == "" {
		eventName, _ = event["hook_event_name"].(string)
	}
	handled, err := runCodexDesktopHook(cmd, eventName, event)
	if handled {
		return err
	}
	return neutralHookOutput("codex", cmd)
}

func desktopGoal(prompt string) (string, bool) {
	trimmed := strings.TrimSpace(prompt)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"stateseal:", "stateseal：", "/seal ", "/seal\n"} {
		if strings.HasPrefix(lower, prefix) {
			goal := strings.TrimSpace(trimmed[len(prefix):])
			return goal, goal != ""
		}
	}
	return "", false
}

func desktopAffirmative(prompt string) bool {
	value := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(prompt, "。")))
	switch value {
	case "y", "yes", "confirm", "confirmed", "确认", "同意", "接受", "应用", "可以", "继续":
		return true
	default:
		return false
	}
}

func desktopNegative(prompt string) bool {
	value := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(prompt, "。")))
	switch value {
	case "n", "no", "reject", "cancel", "拒绝", "取消", "不接受", "不要应用":
		return true
	default:
		return false
	}
}

func desktopTerminalStage(stage string) bool {
	return stage == store.DesktopStageApplied || stage == store.DesktopStageRejected || stage == store.DesktopStageFailed
}

func desktopProjectReady(root, agent string) bool {
	if _, err := os.Stat(filepath.Join(root, "seal.yaml")); err != nil {
		return false
	}
	return adapterConfigured(root, agent)
}

func desktopSetupContext(session store.DesktopSession) string {
	policy, err := projectPolicyPreview(session.RepoRoot, !fileExists(filepath.Join(session.RepoRoot, "seal.yaml")))
	if err != nil {
		return "StateSeal could not prepare the first-project verification preview: " + err.Error() + ". Do not modify the repository."
	}
	return fmt.Sprintf("StateSeal first-project confirmation is required before development. Do not edit or run implementation tools. Present this concise plan to the user and ask for an explicit yes/no response:\nProject: %s\nAgent: Codex Desktop\nAdmission: %s\nCompletion: %s\nProtected: seal.yaml and Agent lifecycle configuration\nExecution: isolated Git worktree plus fresh evaluator.\nNo repository change has been made yet.", filepath.Base(session.RepoRoot), checkSetSummary(policy.Admission.Checks), checkSetSummary(policy.Completion.Checks))
}

func desktopRunContext(session store.DesktopSession) string {
	return "This turn is a StateSeal-managed delivery. Do not analyze, edit, test, or modify the source workspace directly. Execute exactly one tool command, then follow its JSON next_action:\n" + desktopRunCommand(session)
}

func desktopApplyContext(session store.DesktopSession) string {
	return "The user explicitly accepted the exact verified checkpoint. Do not edit code. Execute exactly:\n" + desktopApplyCommand(session)
}

func desktopPendingContext(session store.DesktopSession) string {
	return fmt.Sprintf("StateSeal admitted task %s with receipt %s. The checkpoint has NOT been applied. Present the verified result and residual risks, then ask the user for an explicit yes/no decision. Do not apply without that decision.", session.TaskID, session.ReceiptID)
}

func desktopExistingTaskContext(session store.DesktopSession) string {
	switch session.Stage {
	case store.DesktopStageReady:
		return desktopRunContext(session)
	case store.DesktopStagePendingApply:
		return desktopPendingContext(session)
	case store.DesktopStageSetupPending:
		return desktopSetupContext(session)
	case store.DesktopStageRunning:
		return "A StateSeal controlled child run is already active. Wait for its tool result; do not start another run or edit the source workspace."
	case store.DesktopStageFailed:
		return "The StateSeal run failed: " + session.LastError + ". Report the failure and do not edit or apply unverified code."
	default:
		return "StateSeal Desktop session state is " + session.Stage + "."
	}
}

func desktopRunCommand(session store.DesktopSession) string {
	binary, _ := sealExecutable()
	return shellJoin([]string{binary, "desktop", "run", "--session", session.SessionID})
}

func desktopApplyCommand(session store.DesktopSession) string {
	binary, _ := sealExecutable()
	return shellJoin([]string{binary, "desktop", "apply", "--session", session.SessionID})
}

func desktopExpectedCommand(session store.DesktopSession) string {
	if session.Stage == store.DesktopStagePendingApply {
		return desktopApplyCommand(session)
	}
	return desktopRunCommand(session)
}

func desktopToolAllowed(event map[string]any, session store.DesktopSession) bool {
	toolName, _ := event["tool_name"].(string)
	if toolName != "Bash" {
		return false
	}
	command := normalizeShell(hookCommand(event))
	allowed := []string{desktopExpectedCommand(session)}
	binary, _ := sealExecutable()
	allowed = append(allowed,
		shellJoin([]string{binary, "desktop", "status", "--session", session.SessionID}),
		shellJoin([]string{binary, "desktop", "recover", "--session", session.SessionID}),
		shellJoin([]string{binary, "desktop", "reject", "--session", session.SessionID}),
	)
	for _, expected := range allowed {
		if command == normalizeShell(expected) {
			return true
		}
	}
	return false
}

func codexDesktopToolDenied(cmd *cobra.Command, expected string) error {
	response := map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": "PreToolUse", "permissionDecision": "deny",
		"permissionDecisionReason": "This Desktop turn is bound to StateSeal. Direct tools would bypass the isolated candidate and independent evaluator. Execute only: " + expected,
	}}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(response)
}

func codexDesktopContext(cmd *cobra.Command, event, message string) error {
	response := map[string]any{
		"systemMessage":      "StateSeal managed delivery is active.",
		"hookSpecificOutput": map[string]any{"hookEventName": event, "additionalContext": message},
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(response)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
