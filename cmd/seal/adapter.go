package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/spf13/cobra"
)

var supportedAgents = []string{"codex", "claude", "gemini", "cursor", "copilot", "opencode"}

func adapterCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "adapter", Short: "Configure thin coding-agent integrations"}
	for _, agent := range supportedAgents {
		cmd.AddCommand(agentAdapterCmd(agent))
	}
	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List supported lifecycle adapters", Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "AGENT\tBOUNDARIES\tCONFIGURATION")
		for _, row := range adapterRows() {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", row[0], row[1], row[2])
		}
	}})
	return cmd
}

func agentAdapterCmd(agent string) *cobra.Command {
	cmd := &cobra.Command{Use: agent, Short: "Integrate StateSeal with " + agentDisplayName(agent)}
	var force bool
	var binary string
	install := &cobra.Command{Use: "install", Short: "Install repository-local lifecycle integration", RunE: func(cmd *cobra.Command, _ []string) error {
		root, err := identity.GitRoot(".")
		if err != nil {
			return codedError{10, err}
		}
		if binary == "" {
			binary, err = sealExecutable()
			if err != nil {
				return codedError{10, err}
			}
		}
		path, err := installAgentAdapter(root, agent, binary, force)
		if err != nil {
			return codedError{10, err}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s adapter installed at %s\nCommit the file as protected infrastructure before the first managed run.\n", agentDisplayName(agent), path)
		return nil
	}}
	install.Flags().BoolVar(&force, "force", false, "replace an existing StateSeal integration")
	install.Flags().StringVar(&binary, "binary", "", "absolute StateSeal binary path embedded in the integration")
	hook := &cobra.Command{Use: "hook [event]", Args: cobra.MaximumNArgs(1), Short: "Process an agent lifecycle event", Hidden: true, RunE: func(c *cobra.Command, args []string) error {
		return runAgentHook(agent, c, args)
	}}
	cmd.AddCommand(install, hook)
	return cmd
}

func adapterRows() [][3]string {
	return [][3]string{
		{"codex", "PostToolUse, Stop", ".codex/hooks.json"},
		{"claude", "PostToolUse, Stop", ".claude/settings.json"},
		{"gemini", "AfterTool, AfterAgent", ".gemini/settings.json"},
		{"cursor", "afterShellExecution, stop", ".cursor/hooks.json"},
		{"copilot", "postToolUse, agentStop", ".github/hooks/stateseal.json"},
		{"opencode", "tool.execute.after, session.idle", ".opencode/plugins/stateseal.js"},
	}
}

func agentDisplayName(agent string) string {
	names := map[string]string{"codex": "Codex", "claude": "Claude Code", "gemini": "Gemini CLI", "cursor": "Cursor Agent", "copilot": "GitHub Copilot CLI", "opencode": "OpenCode"}
	return names[agent]
}

func sealExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate StateSeal executable: %w", err)
	}
	return filepath.Abs(path)
}

func runAgentHook(agent string, cmd *cobra.Command, args []string) error {
	eventName := ""
	if len(args) == 1 {
		eventName = args[0]
	}
	if os.Getenv("STATESEAL_SUBMIT_DIR") == "" {
		return neutralHookOutput(agent, cmd)
	}
	raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
	if err != nil {
		return agentHookMessage(agent, cmd, "StateSeal could not read the lifecycle event: "+err.Error(), false, eventName)
	}
	var event map[string]any
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &event); err != nil {
			return agentHookMessage(agent, cmd, "StateSeal received a malformed lifecycle event.", false, eventName)
		}
	}
	if eventName == "" {
		eventName, _ = event["hook_event_name"].(string)
	}
	if eventName == "" {
		eventName, _ = event["event_name"].(string)
	}
	isTool, isStop := lifecycleEventKind(agent, eventName)
	if isTool && !matchesVerifierCommand(hookCommand(event), os.Getenv("STATESEAL_ADAPTER_CHECKS")) {
		return neutralHookOutput(agent, cmd)
	}
	if !isTool && !isStop {
		return neutralHookOutput(agent, cmd)
	}
	receipt, err := requestSubmission()
	if err != nil {
		return agentHookMessage(agent, cmd, "StateSeal could not seal this candidate: "+err.Error(), false, eventName)
	}
	if receipt.Verdict == protocol.VerdictAdmitted {
		return neutralHookOutput(agent, cmd)
	}
	message := fmt.Sprintf("StateSeal %s", receipt.Verdict)
	if receipt.RuleID != "" {
		message += " (" + receipt.RuleID + ")"
	}
	if receipt.Reason != "" {
		message += ": " + receipt.Reason
	}
	blockStop := isStop && os.Getenv("STATESEAL_MODE") == "enforce" && receipt.RuleID != protocol.RuleCandidateBudgetExhausted
	return agentHookMessage(agent, cmd, message, blockStop, eventName)
}

func lifecycleEventKind(agent, event string) (tool, stop bool) {
	switch agent {
	case "codex", "claude", "opencode":
		return event == "PostToolUse", event == "Stop"
	case "gemini":
		return event == "AfterTool", event == "AfterAgent"
	case "cursor":
		return event == "afterShellExecution", event == "stop"
	case "copilot":
		return event == "postToolUse" || event == "PostToolUse", event == "agentStop" || event == "Stop"
	default:
		return false, false
	}
}

func neutralHookOutput(agent string, cmd *cobra.Command) error {
	if agent == "gemini" || agent == "cursor" || agent == "copilot" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "{}")
		return err
	}
	return nil
}

func agentHookMessage(agent string, cmd *cobra.Command, message string, block bool, event string) error {
	var response map[string]any
	switch agent {
	case "codex":
		response = map[string]any{"continue": !block, "systemMessage": message}
		if block {
			response["stopReason"] = message
		}
	case "claude":
		if block {
			response = map[string]any{"decision": "block", "reason": message}
		} else {
			response = map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": event, "additionalContext": message}}
		}
	case "gemini":
		if block {
			response = map[string]any{"decision": "deny", "reason": message}
		} else {
			response = map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": event, "additionalContext": message}}
		}
	case "cursor":
		if block {
			response = map[string]any{"followup_message": message}
		} else {
			response = map[string]any{"user_message": message}
		}
	case "copilot":
		if block {
			response = map[string]any{"decision": "block", "reason": message}
		} else {
			response = map[string]any{"additionalContext": message}
		}
	case "opencode":
		return nil
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(response)
}

func hookCommand(event map[string]any) string {
	for _, key := range []string{"tool_input", "input", "toolArgs", "tool_args"} {
		if nested, ok := event[key].(map[string]any); ok {
			for _, commandKey := range []string{"command", "cmd"} {
				if command, ok := nested[commandKey].(string); ok {
					return command
				}
			}
		}
	}
	for _, key := range []string{"command", "cmd"} {
		if command, ok := event[key].(string); ok {
			return command
		}
	}
	return ""
}

func matchesVerifierCommand(command, configured string) bool {
	var checks []string
	if command == "" || json.Unmarshal([]byte(configured), &checks) != nil {
		return false
	}
	command = normalizeShell(command)
	for _, check := range checks {
		if command == normalizeShell(check) {
			return true
		}
	}
	return false
}

func normalizeShell(command string) string { return strings.Join(strings.Fields(command), " ") }

var safeShellArg = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func shellQuote(value string) string {
	if value != "" && safeShellArg.MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellJoin(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}
