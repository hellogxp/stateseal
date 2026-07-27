package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hellogxp/stateseal/internal/i18n"
	workspacepkg "github.com/hellogxp/stateseal/internal/workspace"
	"github.com/spf13/cobra"
)

func workspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Inspect non-Git folders that contain one or more Git repositories",
	}
	cmd.AddCommand(workspaceListCmd())
	return cmd
}

func workspaceListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List repositories available to StateSeal in a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			inspection, err := workspacepkg.Inspect(path)
			if err != nil {
				return codedError{10, err}
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(inspection)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\n", inspection.Root)
			if len(inspection.Repositories) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No Git repositories found.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nREPOSITORY\tPATH")
			for _, repository := range inspection.Repositories {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", repository.Name, repository.RelativePath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable workspace information")
	return cmd
}

func resolveRunRepository(cmd *cobra.Command, selector, goal string, locale i18n.Locale, silent bool) (string, workspacepkg.Decision, error) {
	decision, err := workspacepkg.Route(".", selector, goal)
	if err != nil {
		return "", workspacepkg.Decision{}, err
	}
	if decision.Selected == nil {
		if len(decision.Inspection.Repositories) == 0 {
			return "", decision, fmt.Errorf("workspace %s contains no Git repositories", decision.Inspection.Root)
		}
		if silent || !isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
			return "", decision, fmt.Errorf(
				"StateSeal could not safely infer which project this task changes; interactive Agents should clarify the project naturally, while automation may use --repo")
		}
		selected, chooseErr := chooseWorkspaceProject(cmd.InOrStdin(), cmd.OutOrStdout(), decision, locale)
		if chooseErr != nil {
			return "", decision, chooseErr
		}
		decision, err = workspacepkg.Route(decision.Inspection.Root, selected, goal)
		if err != nil {
			return "", workspacepkg.Decision{}, err
		}
	}
	return decision.Selected.Root, decision, nil
}

func chooseWorkspaceProject(r io.Reader, w io.Writer, decision workspacepkg.Decision, locale i18n.Locale) (string, error) {
	fmt.Fprintln(w, integrationText(locale,
		"StateSeal found several projects that may match this task.",
		"StateSeal 发现多个可能与本任务相关的项目。"))
	for index, repository := range decision.Inspection.Repositories {
		fmt.Fprintf(w, "  %d. %s\n", index+1, repository.RelativePath)
	}
	fmt.Fprint(w, integrationText(locale,
		"Which project should this task change? Enter a number: ",
		"本次任务要修改哪个项目？请输入序号："))
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	index, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || index < 1 || index > len(decision.Inspection.Repositories) {
		return "", fmt.Errorf("%s", integrationText(locale,
			"project selection must be one of the displayed numbers",
			"项目选择必须是上面展示的序号"))
	}
	return decision.Inspection.Repositories[index-1].RelativePath, nil
}

func printWorkspaceRoute(w io.Writer, decision workspacepkg.Decision, locale i18n.Locale) {
	if decision.Selected == nil {
		return
	}
	fmt.Fprintf(w, "\n%s\n", integrationText(locale, "StateSeal · Project routing", "StateSeal · 项目路由"))
	fmt.Fprintf(w, "  Workspace:  %s\n", decision.Inspection.Root)
	fmt.Fprintf(w, "  Repository: %s\n", decision.Selected.RelativePath)
	fmt.Fprintf(w, "  %s: %s · %s\n", integrationText(locale, "Selection", "选择方式"), decision.Method, decision.Confidence)
	if len(decision.Evidence) > 0 {
		fmt.Fprintf(w, "  %s: %s\n", integrationText(locale, "Evidence", "选择依据"), strings.Join(decision.Evidence, "; "))
	}
}
