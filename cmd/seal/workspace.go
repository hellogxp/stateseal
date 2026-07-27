package main

import (
	"encoding/json"
	"fmt"

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
