package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/spf13/cobra"
)

// Agent integration is machine/user-scoped. Project policy remains repository
// scoped and is created separately on the first managed task.
type integrationSpec struct {
	ID           string
	DisplayName  string
	Aliases      []string
	ConfigPath   string
	Surfaces     string
	SupportLevel string
	Restart      bool
	Kind         string
	Agent        string
}

const (
	integrationKindHooks = "hooks"
	integrationKindMCP   = "mcp"
)

var userIntegrationSpecs = []integrationSpec{
	{
		ID:           "codex-desktop",
		DisplayName:  "Codex Desktop",
		Aliases:      []string{"codex-desktop"},
		ConfigPath:   ".codex/config.toml",
		Surfaces:     "Desktop",
		SupportLevel: "experimental-desktop",
		Restart:      true,
		Kind:         integrationKindMCP,
		Agent:        "codex",
	},
	{
		ID:           "codex",
		DisplayName:  "Codex CLI",
		Aliases:      []string{"codex", "codex-cli"},
		ConfigPath:   ".codex/hooks.json",
		Surfaces:     "CLI",
		SupportLevel: "lifecycle",
		Kind:         integrationKindHooks,
		Agent:        "codex",
	},
	{
		ID:           "claude",
		DisplayName:  "Claude Code",
		Aliases:      []string{"claude", "claude-code"},
		ConfigPath:   ".claude/settings.json",
		Surfaces:     "CLI, IDE",
		SupportLevel: "experimental-desktop",
		Kind:         integrationKindHooks,
		Agent:        "claude",
	},
	{
		ID:           "qoder",
		DisplayName:  "Qoder",
		Aliases:      []string{"qoder", "qoder-cli", "qoder-desktop", "qoder-ide"},
		ConfigPath:   ".qoder/settings.json",
		Surfaces:     "CLI, IDE, JetBrains",
		SupportLevel: "experimental",
		Restart:      true,
		Kind:         integrationKindHooks,
		Agent:        "qoder",
	},
}

func integrateCmd() *cobra.Command {
	var binary, configPath string
	var force bool
	cmd := &cobra.Command{
		Use:   "integrate <agent>",
		Short: "Install a user-level Agent integration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := findIntegrationSpec(args[0])
			if err != nil {
				return codedError{10, err}
			}
			if binary == "" {
				binary, err = sealExecutable()
				if err != nil {
					return codedError{10, err}
				}
			}
			if !filepath.IsAbs(binary) {
				return codedError{10, fmt.Errorf("--binary must be an absolute path")}
			}
			path, err := resolveIntegrationPath(spec, configPath)
			if err != nil {
				return codedError{10, err}
			}
			if err := installUserIntegration(spec, path, binary, force); err != nil {
				return codedError{10, err}
			}
			if err := validateUserIntegration(spec, path); err != nil {
				return codedError{11, fmt.Errorf("integration self-check failed: %w", err)}
			}
			if spec.Kind == integrationKindMCP {
				if err := validateMCPServer(binary, spec.Agent); err != nil {
					return codedError{11, fmt.Errorf("integration protocol self-check failed: %w", err)}
				}
			}
			locale := i18n.Detect()
			fmt.Fprintln(cmd.OutOrStdout(), integrationText(locale, "StateSeal Agent integration", "StateSeal Agent 集成"))
			if spec.Kind == integrationKindMCP {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", integrationText(locale, spec.DisplayName+" local MCP server registered", spec.DisplayName+" 本地 MCP 服务已注册"))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", integrationText(locale, spec.DisplayName+" lifecycle hooks installed", spec.DisplayName+" 生命周期 Hook 已安装"))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s: %s\n", integrationText(locale, "Existing configuration preserved", "已有配置已保留"), path)
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s: %s\n", integrationText(locale, "Static conformance", "静态一致性检查"), integrationText(locale, "passed", "通过"))
			if spec.Kind == integrationKindMCP {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", integrationText(locale, "MCP initialize and tool handshake passed", "MCP 初始化与工具握手通过"))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s: %s\n", integrationText(locale, "Support level", "支持等级"), spec.SupportLevel)
			if spec.Restart {
				fmt.Fprintln(cmd.OutOrStdout(), integrationText(locale, "Restart the Agent application so it reloads the integration.", "请重启 Agent 应用，使其重新加载集成。"))
			}
			if spec.Kind == integrationKindMCP {
				fmt.Fprintln(cmd.OutOrStdout(), integrationText(locale,
					"Open a Git project and describe the development goal normally. The first task asks for one native project-enable approval.",
					"在 Agent 中打开 Git 项目并正常描述开发目标；首次任务只会请求一次原生项目启用授权。"))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), integrationText(locale,
					"Run seal from a Git project. The first managed task asks you to confirm its verification contract.",
					"请在 Git 项目中运行 seal；首次受控任务会要求确认项目验证合同。"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&binary, "binary", "", "absolute StateSeal binary path embedded in the integration")
	cmd.Flags().StringVar(&configPath, "config", "", "override the Agent user configuration path")
	cmd.Flags().BoolVar(&force, "force", false, "replace StateSeal-owned integration entries")
	cmd.AddCommand(integrationListCmd(), integrationStatusCmd(), integrationDoctorCmd(), integrationUninstallCmd())
	return cmd
}

func integrationListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List user-level Agent integrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "AGENT\tSURFACES\tSUPPORT\tCONFIG")
			for _, spec := range userIntegrationSpecs {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t~/%s\n", spec.ID, spec.Surfaces, spec.SupportLevel, spec.ConfigPath)
			}
			return nil
		},
	}
}

func integrationStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show installed user-level integrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "AGENT\tSTATUS\tSUPPORT\tCONFIG")
			for _, spec := range userIntegrationSpecs {
				path, err := resolveIntegrationPath(spec, "")
				if err != nil {
					return err
				}
				status := "not-installed"
				if err := validateUserIntegration(spec, path); err == nil {
					status = "installed"
				} else if _, statErr := os.Stat(path); statErr == nil {
					status = "needs-repair"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", spec.ID, status, spec.SupportLevel, path)
			}
			return nil
		},
	}
}

func integrationDoctorCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "doctor <agent>",
		Short: "Validate an installed Agent integration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := findIntegrationSpec(args[0])
			if err != nil {
				return codedError{10, err}
			}
			path, err := resolveIntegrationPath(spec, configPath)
			if err != nil {
				return codedError{10, err}
			}
			if err := validateUserIntegration(spec, path); err != nil {
				return codedError{10, err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ %s integration configuration\n", spec.DisplayName)
			if spec.Kind == integrationKindMCP {
				binary, err := mcpIntegrationBinaryFromPath(path)
				if err != nil {
					return codedError{10, err}
				}
				if err := validateMCPServer(binary, spec.Agent); err != nil {
					return codedError{10, err}
				}
				fmt.Fprintln(cmd.OutOrStdout(), "✓ MCP initialize handshake")
				fmt.Fprintln(cmd.OutOrStdout(), "✓ inspect, enable, deliver, status, apply, and reject tools")
				fmt.Fprintln(cmd.OutOrStdout(), "✓ native approval policy for enable_project and apply_verified")
				fmt.Fprintf(cmd.OutOrStdout(), "Support: %s\n", spec.SupportLevel)
				return nil
			}
			for _, event := range integrationEvents(spec) {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ %s boundary\n", event)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Support: %s\n", spec.SupportLevel)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "override the Agent user configuration path")
	return cmd
}

func integrationUninstallCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "uninstall <agent>",
		Short: "Remove only StateSeal-owned Agent integration entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := findIntegrationSpec(args[0])
			if err != nil {
				return codedError{10, err}
			}
			path, err := resolveIntegrationPath(spec, configPath)
			if err != nil {
				return codedError{10, err}
			}
			removed, err := removeUserIntegration(spec, path)
			if err != nil {
				return codedError{10, err}
			}
			if removed {
				fmt.Fprintf(cmd.OutOrStdout(), "StateSeal integration removed from %s; unrelated configuration was preserved.\n", path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "No StateSeal integration was present in %s.\n", path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "override the Agent user configuration path")
	return cmd
}

func findIntegrationSpec(value string) (integrationSpec, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, spec := range userIntegrationSpecs {
		for _, alias := range spec.Aliases {
			if value == alias {
				return spec, nil
			}
		}
	}
	return integrationSpec{}, fmt.Errorf("unsupported user-level integration %q; choose one of: codex-desktop, codex-cli, claude-code, qoder", value)
}

func resolveIntegrationPath(spec integrationSpec, override string) (string, error) {
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("--config must be an absolute path")
		}
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate user home: %w", err)
	}
	return filepath.Join(home, filepath.FromSlash(spec.ConfigPath)), nil
}

func installUserIntegration(spec integrationSpec, path, binary string, force bool) error {
	if err := backupIntegrationConfig(path); err != nil {
		return err
	}
	if spec.Kind == integrationKindMCP {
		if err := installMCPIntegration(spec, path, binary, force); err != nil {
			return err
		}
		legacyPath := filepath.Join(filepath.Dir(path), "hooks.json")
		_, err := removeIntegrationMarkers(legacyPath, []string{adapterCommandMarker("codex", "desktop-hook")}, legacyCodexDesktopEvents())
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy Codex Desktop hooks: %w", err)
		}
		return nil
	}
	if spec.ID == "codex" {
		if _, err := removeIntegrationMarkers(path, []string{adapterMarker("codex")}, []string{"PostToolUse", "Stop"}); err != nil {
			return fmt.Errorf("migrate legacy Codex integration: %w", err)
		}
	}
	return installUserNestedHooks(path, binary, spec.ID, force, integrationHookSpecs(spec))
}

func integrationHookSpecs(spec integrationSpec) []nestedHookSpec {
	switch spec.Agent {
	case "codex":
		return []nestedHookSpec{{"PostToolUse", "^Bash$", 1800}, {"Stop", "", 1800}}
	case "qoder":
		return []nestedHookSpec{{"PostToolUse", "^(Bash|run_in_terminal)$", 30}, {"Stop", "", 30}}
	default:
		return []nestedHookSpec{{"PostToolUse", "^Bash$", 1800}, {"Stop", "", 1800}}
	}
}

func integrationEvents(spec integrationSpec) []string {
	specs := integrationHookSpecs(spec)
	events := make([]string, 0, len(specs))
	for _, hook := range specs {
		events = append(events, hook.event)
	}
	return events
}

func backupIntegrationConfig(path string) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	backup := path + ".stateseal.bak"
	if _, err := os.Stat(backup); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(backup, raw, 0o600)
}

func validateUserIntegration(spec integrationSpec, path string) error {
	if spec.Kind == integrationKindMCP {
		return validateMCPIntegration(spec, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s has no hooks object", path)
	}
	marker := userIntegrationMarker(spec)
	for _, event := range integrationEvents(spec) {
		if !containsMarker(hooks[event], marker) {
			return fmt.Errorf("%s integration is missing %s", spec.DisplayName, event)
		}
	}
	return nil
}

func removeUserIntegration(spec integrationSpec, path string) (bool, error) {
	if spec.Kind == integrationKindMCP {
		removed, err := removeMCPIntegration(spec, path)
		if err != nil {
			return false, err
		}
		legacyPath := filepath.Join(filepath.Dir(path), "hooks.json")
		legacyRemoved, legacyErr := removeIntegrationMarkers(legacyPath, []string{adapterCommandMarker("codex", "desktop-hook")}, legacyCodexDesktopEvents())
		if legacyErr != nil && !os.IsNotExist(legacyErr) {
			return false, legacyErr
		}
		return removed || legacyRemoved, nil
	}
	markers := []string{userIntegrationMarker(spec)}
	if spec.ID == "codex" {
		markers = append(markers, adapterMarker(spec.ID))
	}
	return removeIntegrationMarkers(path, markers, integrationEvents(spec))
}

func removeIntegrationMarkers(path string, markers, events []string) (bool, error) {
	root, err := readJSONObject(path)
	if err != nil {
		return false, err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return false, nil
	}
	removed := false
	for _, event := range events {
		groups, _ := hooks[event].([]any)
		for _, marker := range markers {
			if containsMarker(groups, marker) {
				removed = true
			}
			groups = removeMarkedNestedGroups(groups, marker)
		}
		if len(groups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = groups
		}
	}
	if !removed {
		return false, nil
	}
	return true, writeJSONObject(path, root)
}

func userIntegrationMarker(spec integrationSpec) string {
	return adapterMarker(spec.Agent)
}

func legacyCodexDesktopEvents() []string {
	return []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
}

func integrationText(locale i18n.Locale, english, chinese string) string {
	if locale.IsChinese() {
		return chinese
	}
	return english
}
