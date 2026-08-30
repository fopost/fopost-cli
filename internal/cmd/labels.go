package cmd

import (
	"fmt"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newLabelsCmd) }

func newLabelsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "labels",
		Aliases: []string{"label"},
		Short:   "List, create, and remove campaign labels",
	}
	cmd.AddCommand(newLabelsListCmd(state), newLabelsCreateCmd(state), newLabelsDeleteCmd(state))
	return cmd
}

func newLabelsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List labels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			labels, err := client.Labels.List(cmd.Context(), resolved.Workspace)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(labels, func() {
				rows := make([][]string, 0, len(labels))
				for _, label := range labels {
					workspace := "-"
					if label.Workspace != nil {
						workspace = label.Workspace.Name
					}
					rows = append(rows, []string{label.ID, output.Truncate(label.Name, 28), output.Dash(label.Color), workspace})
				}
				printer.Table([]string{"id", "name", "color", "workspace"}, rows)
			})
		},
	}
}

func newLabelsCreateCmd(state *State) *cobra.Command {
	var name, color string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a label",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return usageErrorf("--name is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			label, err := client.Labels.Create(cmd.Context(), &fopost.CreateLabelRequest{
				WorkspaceID: workspaceID,
				Name:        name,
				Color:       color,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(label, func() {
				printer.Success("Created label %s (%s).", label.Name, label.ID)
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "label name (required)")
	cmd.Flags().StringVar(&color, "color", "#2563eb", "hex color")
	return cmd
}

func newLabelsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <label-id>",
		Short: "Delete a label and unlink it from every post carrying it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete label %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Labels.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted label %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
