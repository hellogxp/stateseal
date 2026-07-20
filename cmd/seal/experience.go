package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func setupCmd() *cobra.Command {
	var agent, binary string
	var force bool
	cmd := &cobra.Command{
		Use:   "setup --agent <agent>",
		Short: "Configure project policy and an Agent integration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			locale := i18n.Detect()
			if !isSupportedAgent(agent) {
				return codedError{10, fmt.Errorf("unsupported agent %q; choose one of: %s", agent, strings.Join(supportedAgents, ", "))}
			}
			root, err := identity.GitRoot(".")
			if err != nil {
				return codedError{10, err}
			}
			if err := identity.CheckpointIdentity(root); err != nil {
				return codedError{10, err}
			}
			if _, err := exec.LookPath(agentExecutable(agent)); err != nil {
				return codedError{10, fmt.Errorf("%s executable %q was not found in PATH", agentDisplayName(agent), agentExecutable(agent))}
			}
			if binary == "" {
				binary, err = sealExecutable()
				if err != nil {
					return codedError{10, err}
				}
			}

			result, err := configureProject(root, agent, binary, force)
			if err != nil {
				return codedError{10, err}
			}
			settings, _ := store.LoadProjectSettings(root)
			settings.Agent = agent
			if err := store.SaveProjectSettings(root, settings); err != nil {
				return codedError{11, err}
			}

			fmt.Fprintln(cmd.OutOrStdout(), locale.T(i18n.SetupTitle))
			if result.CreatedPolicy {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", locale.T(i18n.PolicyCreated, result.PolicyPath))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", locale.T(i18n.PolicyPreserved, result.PolicyPath))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", locale.T(i18n.IntegrationReady, agentDisplayName(agent), result.AdapterPath))
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", locale.T(i18n.IdentityReady))
			printVerificationPlan(cmd.OutOrStdout(), result.Policy, result.Detected)
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n\n%s\n", locale.T(i18n.BeforeFirstRun, relativeDisplay(root, result.PolicyPath), relativeDisplay(root, result.AdapterPath)), locale.T(i18n.StartTask))
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "codex", "coding Agent to configure")
	cmd.Flags().StringVar(&binary, "binary", "", "absolute StateSeal binary path embedded in the integration")
	cmd.Flags().BoolVar(&force, "force", false, "replace a conflicting StateSeal-owned integration")
	return cmd
}

type projectSetupResult struct {
	Policy        config.Policy
	PolicyPath    string
	AdapterPath   string
	Detected      string
	CreatedPolicy bool
}

func configureProject(root, agent, binary string, force bool) (projectSetupResult, error) {
	result := projectSetupResult{PolicyPath: filepath.Join(root, "seal.yaml")}
	if _, statErr := os.Stat(result.PolicyPath); os.IsNotExist(statErr) {
		result.Policy, result.Detected = discoveredProjectPolicy(root)
		result.Policy.Task.Goal = "Runtime goals are supplied by seal run."
		if err := config.Write(result.PolicyPath, result.Policy); err != nil {
			return result, err
		}
		result.CreatedPolicy = true
	} else if statErr != nil {
		return result, statErr
	} else {
		var err error
		result.Policy, _, err = config.Load(root)
		if err != nil {
			return result, err
		}
		result.Detected = verifierSummary(result.Policy)
	}
	if err := identity.EnsureLocalExclude(root, ".stateseal/"); err != nil {
		return result, err
	}
	var err error
	result.AdapterPath, err = installAgentAdapter(root, agent, binary, force)
	if err != nil {
		if result.CreatedPolicy {
			_ = os.Remove(result.PolicyPath)
		}
		return result, err
	}
	return result, nil
}

func ensureManagedSetup(cmd *cobra.Command, root, agent string, locale i18n.Locale, approved, silent bool) error {
	policyMissing := false
	if _, err := os.Stat(filepath.Join(root, "seal.yaml")); os.IsNotExist(err) {
		policyMissing = true
	} else if err != nil {
		return err
	}
	adapterMissing := !adapterConfigured(root, agent)
	if !policyMissing && !adapterMissing {
		settings, _ := store.LoadProjectSettings(root)
		settings.Agent = agent
		return store.SaveProjectSettings(root, settings)
	}

	dirty, err := identity.Git(root, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(dirty)) != "" {
		return fmt.Errorf("%s", locale.T(i18n.FirstRunDirty))
	}
	if !approved {
		if silent || !isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
			return fmt.Errorf("%s", locale.T(i18n.FirstRunInteractive))
		}
		preview, previewErr := projectPolicyPreview(root, policyMissing)
		if previewErr != nil {
			return previewErr
		}
		printProjectSetupPreview(cmd.OutOrStdout(), root, agent, preview, locale)
		fmt.Fprint(cmd.OutOrStdout(), locale.T(i18n.FirstRunConfirm))
		line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if readErr != nil && len(line) == 0 {
			return readErr
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer == "n" || answer == "no" {
			return fmt.Errorf("%s", locale.T(i18n.FirstRunCancelled))
		}
	}
	binary, err := sealExecutable()
	if err != nil {
		return err
	}
	result, err := configureProject(root, agent, binary, false)
	if err != nil {
		return err
	}
	paths := []string{relativeDisplay(root, result.PolicyPath), relativeDisplay(root, result.AdapterPath)}
	if _, err := identity.Git(root, append([]string{"add", "--"}, paths...)...); err != nil {
		return fmt.Errorf("stage StateSeal configuration: %w", err)
	}
	staged, err := identity.Git(root, "diff", "--cached", "--quiet")
	_ = staged
	if err != nil {
		if _, commitErr := identity.Git(root, "commit", "-m", "chore(stateseal): configure verified agent delivery"); commitErr != nil {
			return fmt.Errorf("commit StateSeal configuration: %w", commitErr)
		}
	}
	settings, _ := store.LoadProjectSettings(root)
	settings.Agent = agent
	settings.TrustedHookAutomation = agent == "codex"
	if err := store.SaveProjectSettings(root, settings); err != nil {
		return err
	}
	if !silent {
		fmt.Fprintln(cmd.OutOrStdout(), locale.T(i18n.SetupComplete, agentDisplayName(agent), result.Detected))
	}
	return nil
}

func projectPolicyPreview(root string, missing bool) (config.Policy, error) {
	if !missing {
		policy, _, err := config.Load(root)
		return policy, err
	}
	policy, _ := discoveredProjectPolicy(root)
	return policy, nil
}

func discoveredProjectPolicy(root string) (config.Policy, string) {
	plan := discoverVerificationPlan(root)
	policy := config.Default(identity.NormalizeTaskID(filepath.Base(root)), plan.Admission)
	policy.Completion.Checks = plan.Completion
	for _, gap := range plan.Uncovered {
		policy.ResidualRisks = append(policy.ResidualRisks, gap+".")
	}
	return policy, strings.Join(plan.Detected, ", ")
}

func ensureDesktopMCPSetup(root, agent string) error {
	if _, _, err := config.Load(root); err != nil {
		return fmt.Errorf("StateSeal project policy is not enabled: %w", err)
	}
	settings, err := store.LoadProjectSettings(root)
	if err != nil {
		return fmt.Errorf("StateSeal Desktop project is not enabled: %w", err)
	}
	if !settings.DesktopEnabled || settings.DesktopSurface != "mcp" {
		return fmt.Errorf("StateSeal Desktop project is not enabled; approve enable_project first")
	}
	if !desktopAgentEnabled(settings, agent) {
		return fmt.Errorf("StateSeal Desktop is not enabled for %s in this project", agentDisplayName(agent))
	}
	if settings.DesktopPolicyDigest == "" || settings.DesktopPolicyDigest != rawPolicyDigest(root) {
		return fmt.Errorf("seal.yaml changed after Desktop enablement; review and enable the project again")
	}
	return nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func desktopAgentEnabled(settings store.ProjectSettings, agent string) bool {
	return containsString(settings.DesktopAgents, agent) ||
		(len(settings.DesktopAgents) == 0 && settings.Agent == agent)
}

func printProjectSetupPreview(w io.Writer, root, agent string, policy config.Policy, locale i18n.Locale) {
	fmt.Fprintln(w, integrationText(locale, "StateSeal · First project setup", "StateSeal · 首次项目配置"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s: %s\n", integrationText(locale, "Project", "项目"), filepath.Base(root))
	fmt.Fprintf(w, "  Agent: %s\n", agentDisplayName(agent))
	fmt.Fprintf(w, "  Admission: %s\n", checkSetSummary(policy.Admission.Checks))
	fmt.Fprintf(w, "  Completion: %s\n", checkSetSummary(policy.Completion.Checks))
	fmt.Fprintf(w, "  %s: seal.yaml, %s\n", integrationText(locale, "Protected", "受保护配置"), relativeDisplay(root, agentAdapterPath(root, agent)))
	fmt.Fprintln(w)
	fmt.Fprintln(w, integrationText(locale,
		"Checks run locally in isolated evaluators. The generated policy and lifecycle integration will be committed as auditable project configuration.",
		"检查将在本地隔离 Evaluator 中执行；生成的策略和生命周期集成会作为可审计的项目配置提交。"))
	fmt.Fprintln(w)
}

func isSupportedAgent(agent string) bool {
	for _, candidate := range supportedAgents {
		if agent == candidate {
			return true
		}
	}
	return false
}

func agentExecutable(agent string) string {
	switch agent {
	case "cursor":
		return "cursor-agent"
	case "qoder":
		return "qodercli"
	default:
		return agent
	}
}

func agentAdapterPath(root, agent string) string {
	switch agent {
	case "codex":
		return filepath.Join(root, ".codex", "hooks.json")
	case "claude":
		return filepath.Join(root, ".claude", "settings.json")
	case "qoder":
		return filepath.Join(root, ".qoder", "settings.json")
	case "gemini":
		return filepath.Join(root, ".gemini", "settings.json")
	case "cursor":
		return filepath.Join(root, ".cursor", "hooks.json")
	case "copilot":
		return filepath.Join(root, ".github", "hooks", "stateseal.json")
	case "opencode":
		return filepath.Join(root, ".opencode", "plugins", "stateseal.js")
	default:
		return ""
	}
}

func adapterConfigured(root, agent string) bool {
	raw, err := os.ReadFile(agentAdapterPath(root, agent))
	if err != nil {
		return false
	}
	if agent == "opencode" {
		return strings.Contains(string(raw), `"opencode", "hook"`)
	}
	return strings.Contains(string(raw), adapterMarker(agent))
}

func adapterHandshake(root, agent string) error {
	path := agentAdapterPath(root, agent)
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s integration: %w", agentDisplayName(agent), err)
	}
	marker := adapterMarker(agent)
	if agent == "opencode" {
		if !strings.Contains(string(raw), `"opencode", "hook"`) {
			return fmt.Errorf("%s integration is missing the StateSeal lifecycle handler", agentDisplayName(agent))
		}
	} else if !strings.Contains(string(raw), marker) {
		return fmt.Errorf("%s integration is missing marker %q", agentDisplayName(agent), marker)
	}
	rows := adapterRows()
	for _, row := range rows {
		if row[0] == agent && row[1] != "" {
			return nil
		}
	}
	return fmt.Errorf("%s has no declared lifecycle capability", agentDisplayName(agent))
}

func agentDisplayNameFromExecutable(executable string) string {
	base := strings.ToLower(filepath.Base(executable))
	for _, agent := range supportedAgents {
		if base == agentExecutable(agent) || strings.Contains(base, agent) {
			return agentDisplayName(agent)
		}
	}
	return filepath.Base(executable)
}

func agentLaunch(agent, goal string, autonomous, trustedHookAutomation bool) ([]string, error) {
	prompt := managedPrompt(goal)
	switch agent {
	case "codex":
		args := []string{"codex", "exec", "--ephemeral", "-s", "workspace-write"}
		if trustedHookAutomation {
			args = append(args, "--dangerously-bypass-hook-trust")
		}
		return append(args, prompt), nil
	case "claude":
		return []string{"claude", "-p", prompt}, nil
	case "qoder":
		args := []string{"qodercli", "--prompt", prompt, "--permission-mode", "accept_edits"}
		if autonomous {
			args[len(args)-1] = "auto"
		}
		return args, nil
	case "gemini":
		return []string{"gemini", "-p", prompt}, nil
	case "cursor":
		if !autonomous {
			return nil, fmt.Errorf("Cursor headless editing requires --autonomous")
		}
		return []string{"cursor-agent", "-p", "--force", prompt}, nil
	case "copilot":
		args := []string{"copilot", "-p", prompt}
		if autonomous {
			args = append(args, "--allow-all-tools", "--allow-all-paths", "--no-ask-user")
		}
		return args, nil
	case "opencode":
		return []string{"opencode", "run", prompt}, nil
	default:
		return nil, fmt.Errorf("unsupported agent %q", agent)
	}
}

func authorizeTrustedHooks(cmd *cobra.Command, root, agent string, locale i18n.Locale, approved, silent bool) (bool, error) {
	if agent != "codex" {
		return false, nil
	}
	settings, err := store.LoadProjectSettings(root)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if settings.TrustedHookAutomation {
		return true, nil
	}
	if !approved {
		if silent {
			return false, fmt.Errorf("%s", locale.T(i18n.HookTrustInteractive))
		}
		if !isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
			return false, nil
		}
		fmt.Fprint(cmd.OutOrStdout(), locale.T(i18n.HookTrustConfirm))
		line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if readErr != nil && len(line) == 0 {
			return false, readErr
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer == "n" || answer == "no" {
			return false, nil
		}
	}
	settings.Agent, settings.TrustedHookAutomation = agent, true
	if err := store.SaveProjectSettings(root, settings); err != nil {
		return false, err
	}
	return true, nil
}

func detectAgent() (string, error) {
	for _, agent := range supportedAgents {
		if _, err := exec.LookPath(agentExecutable(agent)); err == nil {
			return agent, nil
		}
	}
	return "", fmt.Errorf("no supported coding Agent found in PATH; install one of: %s", strings.Join(supportedAgents, ", "))
}

func managedPrompt(goal string) string {
	return "You are working in a StateSeal-managed isolated proposal.\n\n" +
		"Goal:\n" + strings.TrimSpace(goal) + "\n\n" +
		"Implement the goal, run relevant tests, and keep existing behavior compatible. " +
		"Do not modify seal.yaml or Agent lifecycle configuration. " +
		"If StateSeal rejects completion, use its feedback and continue until the configured checks pass."
}

func newTaskID(goal string, now time.Time) string {
	slug := identity.NormalizeTaskID(strings.ToLower(goal))
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "._-")
	}
	nonce := identity.Digest([]byte(fmt.Sprintf("%s\x00%d", goal, now.UnixNano())))[:10]
	return slug + "-" + nonce
}

func printVerificationPlan(w io.Writer, policy config.Policy, detected string) {
	fmt.Fprintln(w, "\nVerification plan")
	fmt.Fprintf(w, "  Admission:  %s\n", checkSetSummary(policy.Admission.Checks))
	fmt.Fprintf(w, "  Completion: %s (fresh evaluator)\n", checkSetSummary(policy.Completion.Checks))
	if detected != "" {
		fmt.Fprintf(w, "  Covered:    %s\n", detected)
	}
	if len(policy.ResidualRisks) > 2 {
		fmt.Fprintf(w, "  Not covered: %s\n", strings.Join(policy.ResidualRisks[2:], "; "))
	}
}

func verifierSummary(policy config.Policy) string { return checkSetSummary(policy.Admission.Checks) }

func checkSetSummary(checks []config.Check) string {
	parts := make([]string, 0, len(checks))
	for _, check := range checks {
		parts = append(parts, shellJoin(check.Command))
	}
	return strings.Join(parts, "; ")
}

func relativeDisplay(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

func isInteractiveTerminal(r io.Reader, w io.Writer) bool {
	in, inputOK := r.(*os.File)
	out, outputOK := w.(*os.File)
	return inputOK && outputOK && term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

func confirmApply(r io.Reader, w io.Writer, locale i18n.Locale) (bool, error) {
	fmt.Fprint(w, locale.T(i18n.ConfirmApply))
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
