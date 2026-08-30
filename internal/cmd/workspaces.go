package cmd

import (
	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newWorkspacesCmd) }

func newWorkspacesCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspaces",
		Aliases: []string{"workspace", "ws"},
		Short:   "List, inspect, and create workspaces",
	}
	cmd.AddCommand(
		newWorkspacesListCmd(state),
		newWorkspacesGetCmd(state),
		newWorkspacesCreateCmd(state),
	)
	return cmd
}

func newWorkspacesListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every workspace the key can reach",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaces, err := client.Workspaces.List(cmd.Context())
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(workspaces, func() {
				rows := make([][]string, 0, len(workspaces))
				for _, workspace := range workspaces {
					rows = append(rows, []string{
						workspace.ID,
						output.Truncate(workspace.Name, 32),
						output.Dash(workspace.Slug),
						output.Dash(workspace.Type),
						itoa(len(workspace.Accounts)),
					})
				}
				printer.Table([]string{"id", "name", "slug", "type", "accounts"}, rows)
			})
		},
	}
}

func newWorkspacesGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <workspace-id>",
		Short: "Show one workspace and its connected accounts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspace, err := client.Workspaces.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(workspace, func() {
				printer.Fields([][2]string{
					{"ID", workspace.ID},
					{"Name", workspace.Name},
					{"Slug", output.Dash(workspace.Slug)},
					{"Type", output.Dash(workspace.Type)},
					{"Timezone", output.Dash(workspace.Timezone)},
					{"Website", output.Dash(workspace.Website)},
					{"Created", output.Stamp(workspace.CreatedAt.Time, workspace.CreatedAt.Raw)},
				})
				if len(workspace.Accounts) == 0 {
					return
				}
				printer.Line("")
				rows := make([][]string, 0, len(workspace.Accounts))
				for _, account := range workspace.Accounts {
					rows = append(rows, []string{account.ID, account.Platform, output.Dash(account.Username), output.Truncate(account.Name, 28)})
				}
				printer.Table([]string{"account id", "platform", "username", "name"}, rows)
			})
		},
	}
}

func newWorkspacesCreateCmd(state *State) *cobra.Command {
	var body fopost.CreateWorkspaceRequest
	var website, description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if body.Name == "" {
				return usageErrorf("--name is required")
			}
			if body.Slug == "" {
				return usageErrorf("--slug is required")
			}
			if website != "" {
				body.Website = fopost.String(website)
			}
			if description != "" {
				body.Description = fopost.String(description)
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspace, err := client.Workspaces.Create(cmd.Context(), &body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(workspace, func() {
				printer.Success("Created workspace %s (%s)", workspace.Name, workspace.ID)
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&body.Name, "name", "", "workspace name (required)")
	flags.StringVar(&body.Slug, "slug", "", "URL slug (required)")
	flags.StringVar(&body.Type, "type", "", "workspace type, e.g. TEAM, BRAND, CLIENT")
	flags.StringVar(&body.Timezone, "timezone", "", "IANA timezone, e.g. Europe/Berlin")
	flags.StringVar(&body.Language, "language", "", "primary language code")
	flags.StringVar(&website, "website", "", "website URL")
	flags.StringVar(&description, "description", "", "short description")
	return cmd
}
