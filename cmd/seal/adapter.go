package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/spf13/cobra"
)

var supportedAgents = []string{"codex", "claude", "qoder", "gemini", "cursor", "copilot", "opencode"}

const hookRuntimePath = ".stateseal/hook-runtime.json"

type hookRuntime struct {
	SubmitDir string `json:"submit_dir"`
	Mode      string `json:"mode"`
	Checks    string `json:"checks"`
}

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

// inertAdapterCompatibilityCmd keeps lifecycle commands installed by older
// StateSeal versions harmless after upgrade. It is hidden and exposes no
// installer or managed-delivery behavior.
func inertAdapterCompatibilityCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "adapter", Hidden: true}
	for _, agent := range supportedAgents {
		agentName := agent
		agentCmd := &cobra.Command{Use: agentName, Hidden: true}
		agentCmd.AddCommand(&cobra.Command{
			Use: "hook [event]", Hidden: true, Args: cobra.MaximumNArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runAgentHook(agentName, c, args)
			},
		})
		cmd.AddCommand(agentCmd)
	}
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
		{"qoder", "PostToolUse, Stop", ".qoder/settings.json"},
		{"gemini", "AfterTool, AfterAgent", ".gemini/settings.json"},
		{"cursor", "afterShellExecution, stop", ".cursor/hooks.json"},
		{"copilot", "postToolUse, agentStop", ".github/hooks/stateseal.json"},
		{"opencode", "tool.execute.after, session.idle", ".opencode/plugins/stateseal.js"},
	}
}

func agentDisplayName(agent string) string {
	names := map[string]string{"codex": "Codex", "claude": "Claude Code", "qoder": "Qoder", "gemini": "Gemini CLI", "cursor": "Cursor Agent", "copilot": "GitHub Copilot CLI", "opencode": "OpenCode"}
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
	// Compatibility kill switch for integrations installed by older releases.
	// StateSeal no longer consumes lifecycle events or returns context, denial,
	// continuation, or blocking signals. Keeping this handler inert prevents a
	// stale machine-level hook from affecting an Agent after upgrade.
	_ = args
	return neutralHookOutput(agent, cmd)
}

func writeHookRuntime(proposal, submitDir, mode, checks string) (func(), error) {
	runtime := hookRuntime{SubmitDir: submitDir, Mode: mode, Checks: checks}
	raw, err := json.Marshal(runtime)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(proposal, filepath.FromSlash(hookRuntimePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create hook runtime directory: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write hook runtime: %w", err)
	}
	cleanup := func() {
		_ = os.Remove(path)
		_ = os.Remove(filepath.Dir(path))
	}
	return cleanup, nil
}

func hydrateHookRuntime(event map[string]any) bool {
	cwd, _ := event["cwd"].(string)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	root, err := identity.GitRoot(cwd)
	if err != nil {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(hookRuntimePath)))
	if err != nil {
		return false
	}
	var runtime hookRuntime
	if json.Unmarshal(raw, &runtime) != nil || !filepath.IsAbs(runtime.SubmitDir) {
		return false
	}
	info, err := os.Stat(runtime.SubmitDir)
	if err != nil || !info.IsDir() {
		return false
	}
	_ = os.Setenv("STATESEAL_SUBMIT_DIR", runtime.SubmitDir)
	_ = os.Setenv("STATESEAL_MODE", runtime.Mode)
	_ = os.Setenv("STATESEAL_ADAPTER_CHECKS", runtime.Checks)
	return true
}

func lifecycleEventKind(agent, event string) (tool, stop bool) {
	switch agent {
	case "codex", "claude", "qoder", "opencode":
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
	// Retained only for binary/source compatibility with older internal call
	// sites. No message or decision may be injected into an Agent.
	_, _, _ = message, block, event
	return neutralHookOutput(agent, cmd)
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
