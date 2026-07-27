package main

import (
	"fmt"
	"path/filepath"

	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/spf13/cobra"
)

// installCmd is the one-step compatibility installer for users who install the
// StateSeal binary without a host-native Plugin. The Plugin already carries
// its MCP registration; this command remains useful for CLI, headless, and
// repair workflows.
func installCmd() *cobra.Command {
	var binary, configPath string
	var force bool
	cmd := &cobra.Command{
		Use:   "install [agent]",
		Short: "Install StateSeal's Agent integration in one step",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent := "codex-desktop"
			if len(args) == 1 {
				agent = args[0]
			}
			spec, err := findIntegrationSpec(agent)
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
					return codedError{11, fmt.Errorf("MCP self-check failed: %w", err)}
				}
			}
			locale := i18n.Detect()
			fmt.Fprintln(cmd.OutOrStdout(), integrationText(locale,
				"StateSeal installation complete.", "StateSeal 安装完成。"))
			fmt.Fprintf(cmd.OutOrStdout(), "  Agent: %s\n", spec.DisplayName)
			fmt.Fprintf(cmd.OutOrStdout(), "  Integration: %s\n", spec.Kind)
			fmt.Fprintf(cmd.OutOrStdout(), "  Config: %s\n", path)
			if spec.Kind == integrationKindMCP {
				fmt.Fprintln(cmd.OutOrStdout(), integrationText(locale,
					"  MCP: bundled with StateSeal; no separate MCP installation is required.",
					"  MCP：已包含在 StateSeal 中，无需单独安装 MCP。"))
			}
			if spec.Restart {
				fmt.Fprintln(cmd.OutOrStdout(), integrationText(locale,
					"Restart the Agent application, then describe code tasks normally.",
					"请重启 Agent 应用，之后正常描述代码任务即可。"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&binary, "binary", "", "absolute StateSeal binary path embedded in the integration")
	cmd.Flags().StringVar(&configPath, "config", "", "override the Agent user configuration path")
	cmd.Flags().BoolVar(&force, "force", false, "replace StateSeal-owned integration entries")
	return cmd
}
