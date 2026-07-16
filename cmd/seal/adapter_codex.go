package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/spf13/cobra"
)

const stateSealHookMarker = "adapter codex hook"

func adapterCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "adapter", Short: "Configure thin coding-agent integrations"}
	cmd.AddCommand(codexAdapterCmd())
	return cmd
}

func codexAdapterCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "codex", Short: "Integrate StateSeal with Codex lifecycle hooks"}
	var force bool
	var binary string
	install := &cobra.Command{Use: "install", Short: "Install repository-local Codex lifecycle hooks", RunE: func(cmd *cobra.Command, _ []string) error {
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
		path := filepath.Join(root, ".codex", "hooks.json")
		if err := installCodexHooks(path, binary, force); err != nil {
			return codedError{10, err}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Codex hooks installed at %s\nReview and trust them with `/hooks` before the first managed run, then commit the file as protected infrastructure.\n", path)
		return nil
	}}
	install.Flags().BoolVar(&force, "force", false, "replace an existing StateSeal hook definition")
	install.Flags().StringVar(&binary, "binary", "", "absolute StateSeal binary path embedded in the hook")
	hook := &cobra.Command{Use: "hook", Short: "Process a Codex lifecycle event", Hidden: true, RunE: runCodexHook}
	cmd.AddCommand(install, hook)
	return cmd
}

func sealExecutable() (string, error) {
	if path, err := exec.LookPath("seal"); err == nil {
		return filepath.Abs(path)
	}
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate StateSeal executable: %w", err)
	}
	return filepath.Abs(path)
}

func installCodexHooks(path, binary string, force bool) error {
	if !filepath.IsAbs(binary) {
		return fmt.Errorf("--binary must be an absolute path")
	}
	root := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parse existing %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	command := shellQuote(binary) + " " + stateSealHookMarker
	for _, event := range []string{"PostToolUse", "Stop"} {
		groups, _ := hooks[event].([]any)
		found := false
		for _, group := range groups {
			if containsStateSealHook(group) {
				found = true
			}
		}
		if found && !force {
			return fmt.Errorf("StateSeal %s hook already exists; use --force to replace it", event)
		}
		if force {
			filtered := groups[:0]
			for _, group := range groups {
				if cleaned, keep := removeStateSealHooks(group); keep {
					filtered = append(filtered, cleaned)
				}
			}
			groups = filtered
		}
		group := map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 1800, "statusMessage": "Sealing candidate state"}},
		}
		if event == "PostToolUse" {
			group["matcher"] = "^Bash$"
		}
		hooks[event] = append(groups, group)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func containsStateSealHook(value any) bool {
	raw, _ := json.Marshal(value)
	return strings.Contains(string(raw), stateSealHookMarker)
}

func removeStateSealHooks(value any) (any, bool) {
	group, ok := value.(map[string]any)
	if !ok {
		return value, true
	}
	handlers, ok := group["hooks"].([]any)
	if !ok {
		return value, true
	}
	filtered := handlers[:0]
	for _, handler := range handlers {
		if !containsStateSealHook(handler) {
			filtered = append(filtered, handler)
		}
	}
	if len(filtered) == 0 {
		return nil, false
	}
	group["hooks"] = filtered
	return group, true
}

func runCodexHook(cmd *cobra.Command, _ []string) error {
	if os.Getenv("STATESEAL_SUBMIT_DIR") == "" {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
	if err != nil {
		return codexHookMessage(cmd, "StateSeal could not read the Codex hook event: "+err.Error(), false)
	}
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		return codexHookMessage(cmd, "StateSeal received a malformed Codex hook event.", false)
	}
	eventName, _ := event["hook_event_name"].(string)
	if eventName == "PostToolUse" && !matchesVerifierCommand(hookCommand(event), os.Getenv("STATESEAL_ADAPTER_CHECKS")) {
		return nil
	}
	if eventName != "PostToolUse" && eventName != "Stop" {
		return nil
	}
	receipt, err := requestSubmission()
	if err != nil {
		return codexHookMessage(cmd, "StateSeal could not seal this candidate: "+err.Error(), false)
	}
	if receipt.Verdict == protocol.VerdictAdmitted {
		return nil
	}
	message := fmt.Sprintf("StateSeal %s", receipt.Verdict)
	if receipt.RuleID != "" {
		message += " (" + receipt.RuleID + ")"
	}
	if receipt.Reason != "" {
		message += ": " + receipt.Reason
	}
	blockStop := eventName == "Stop" && os.Getenv("STATESEAL_MODE") == "enforce" && receipt.RuleID != protocol.RuleCandidateBudgetExhausted
	return codexHookMessage(cmd, message, blockStop)
}

func codexHookMessage(cmd *cobra.Command, message string, stop bool) error {
	response := map[string]any{"continue": !stop, "systemMessage": message}
	if stop {
		response["stopReason"] = message
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(response)
}

func hookCommand(event map[string]any) string {
	for _, key := range []string{"tool_input", "input"} {
		if nested, ok := event[key].(map[string]any); ok {
			for _, commandKey := range []string{"command", "cmd"} {
				if command, ok := nested[commandKey].(string); ok {
					return command
				}
			}
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
