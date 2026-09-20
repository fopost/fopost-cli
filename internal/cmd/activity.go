package cmd

import (
	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newActivityCmd) }

func newActivityCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Read what happened in a workspace, including the audit log",
	}
	cmd.AddCommand(newActivityListCmd(state), newActivityAuditCmd(state))
	return cmd
}

func listActivity(state *State, cmd *cobra.Command, kind, cursor string, limit int) error {
	client, err := state.Client()
	if err != nil {
		return err
	}
	resolved, err := state.Resolved()
	if err != nil {
		return err
	}
	page, err := client.Activity.List(cmd.Context(), &fopost.ListActivityParams{
		WorkspaceID: resolved.Workspace,
		Kind:        kind,
		Cursor:      cursor,
		Limit:       limit,
	})
	if err != nil {
		return err
	}
	printer := state.Printer()
	return printer.Value(page, func() {
		rows := make([][]string, 0, len(page.Events))
		for _, event := range page.Events {
			actor := event.Actor.Name
			if actor == "" {
				actor = event.Actor.Type
			}
			rows = append(rows, []string{
				event.Time.String(),
				event.Kind,
				actor,
				output.Truncate(event.Summary, 60),
			})
		}
		printer.Table([]string{"time", "kind", "actor", "summary"}, rows)
		if page.NextCursor != "" {
			printer.Success("More to read: pass --cursor %s.", page.NextCursor)
		}
	})
}

func newActivityListCmd(state *State) *cobra.Command {
	var kind, cursor string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List activity, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listActivity(state, cmd, kind, cursor, limit)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "publish, connection, webhook, inbox, automation, billing, or security")
	cmd.Flags().StringVar(&cursor, "cursor", "", "next_cursor from the previous page")
	cmd.Flags().IntVar(&limit, "limit", 0, "how many to return, 1 to 100")
	return cmd
}

func newActivityAuditCmd(state *State) *cobra.Command {
	var cursor string
	var limit int
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "The security audit log: who changed access, and when",
		Long: "Members joining, leaving, or changing role and access, plus changes to two-step\n" +
			"verification, passkeys, single sign-on, and signed-in devices. These rows are\n" +
			"append-only and never expire.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listActivity(state, cmd, fopost.ActivityKindSecurity, cursor, limit)
		},
	}
	cmd.Flags().StringVar(&cursor, "cursor", "", "next_cursor from the previous page")
	cmd.Flags().IntVar(&limit, "limit", 0, "how many to return, 1 to 100")
	return cmd
}
