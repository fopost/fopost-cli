package cmd

import (
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newAccountsCmd) }

func newAccountsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "accounts",
		Aliases: []string{"account"},
		Short:   "Inspect connected social accounts and their health",
	}
	cmd.AddCommand(
		newAccountsListCmd(state),
		newAccountsGetCmd(state),
		newAccountsRenameCmd(state),
		newAccountsMoveCmd(state),
		newAccountsHealthCmd(state),
		newAccountsValidateCmd(state),
		newAccountsRefreshCmd(state),
		newAccountsTelegramCmd(state),
		newAccountsSlackCmd(state),
		newAccountsWhatsappCmd(state),
	)
	return cmd
}

func newAccountsListCmd(state *State) *cobra.Command {
	var group string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List connected accounts",
		Long:  "Lists connected accounts. Without --workspace and without a saved default, the API\nanswers with every account the key can reach.",
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
			accounts, err := client.Accounts.ListWithParams(cmd.Context(), &fopost.ListAccountsParams{
				WorkspaceID: resolved.Workspace,
				GroupID:     group,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(accounts, func() {
				rows := make([][]string, 0, len(accounts))
				for _, account := range accounts {
					rows = append(rows, []string{
						account.ID,
						account.Platform,
						output.Dash(account.Username),
						output.Truncate(account.Name, 24),
						output.Dash(account.HealthStatus),
						boolLabel(account.Active, "active", "inactive"),
					})
				}
				printer.Table([]string{"id", "platform", "username", "name", "health", "state"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "only accounts in this account group")
	return cmd
}

func newAccountsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <account-id>",
		Short: "Show one connected account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			account, err := client.Accounts.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(account, func() {
				printer.Fields([][2]string{
					{"ID", account.ID},
					{"Platform", account.Platform},
					{"Username", output.Dash(account.Username)},
					{"Name", output.Dash(account.Name)},
					{"Platform Name", output.Dash(account.PlatformName)},
					{"Workspace", account.Workspace.Name + " (" + account.WorkspaceID + ")"},
					{"Connected", output.Stamp(account.CreatedAt.Time, account.CreatedAt.Raw)},
				})
			})
		},
	}
}

func newAccountsRenameCmd(state *State) *cobra.Command {
	var reset bool
	cmd := &cobra.Command{
		Use:   "rename <account-id> [display-name]",
		Short: "Set the name shown for an account, or restore the platform name",
		Example: strings.Join([]string{
			"  fopost accounts rename acc_1 \"Client A\"",
			"  fopost accounts rename acc_1 --reset",
		}, "\n"),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 2 {
				name = strings.TrimSpace(args[1])
			}
			if reset && name != "" {
				return usageErrorf("a display name and --reset are alternatives; pass one")
			}
			if !reset && name == "" {
				return usageErrorf("a display name is required, or --reset to restore the platform name")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			renamed, err := client.Accounts.Rename(cmd.Context(), args[0], name)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(renamed, func() {
				printer.Success("Account %s is now shown as %s.", renamed.ID, renamed.Name)
			})
		},
	}
	cmd.Flags().BoolVar(&reset, "reset", false, "restore the platform name")
	return cmd
}

func newAccountsMoveCmd(state *State) *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "move <account-id>",
		Short: "Move an account to another workspace you own",
		Long: "Moves the account, its connection, and its inbox and analytics history to another\n" +
			"workspace. The account leaves its account groups.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return usageErrorf("--to is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			moved, err := client.Accounts.Move(cmd.Context(), args[0], to)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(moved, func() {
				printer.Success("Moved account %s to workspace %s.", moved.ID, moved.WorkspaceID)
			})
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "target workspace id (required)")
	return cmd
}

func newAccountsHealthCmd(state *State) *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:   "health [account-id]",
		Short: "Show connection health, for one account or all of them",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			printer := state.Printer()

			if len(args) == 1 {
				health, err := client.Accounts.Health(cmd.Context(), args[0], refresh)
				if err != nil {
					return err
				}
				return printer.Value(health, func() {
					printer.Fields([][2]string{
						{"ID", health.ID},
						{"Platform", health.Platform},
						{"Username", output.Dash(health.Username)},
						{"Health", output.Dash(health.HealthStatus)},
						{"State", boolLabel(health.Active, "active", "inactive")},
						{"Checked", output.Stamp(health.LastHealthCheck.Time, health.LastHealthCheck.Raw)},
					})
				})
			}

			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			summary, err := client.Accounts.HealthSummary(cmd.Context(), resolved.Workspace)
			if err != nil {
				return err
			}
			return printer.Value(summary, func() {
				rows := make([][]string, 0, len(summary.Accounts))
				for _, account := range summary.Accounts {
					rows = append(rows, []string{
						account.ID,
						account.Platform,
						output.Dash(account.Username),
						output.Dash(account.HealthStatus),
						output.Stamp(account.LastHealthCheck.Time, account.LastHealthCheck.Raw),
					})
				}
				printer.Table([]string{"id", "platform", "username", "health", "checked"}, rows)
				printer.Line("")
				printer.Line("%d total · %d healthy · %d degraded · %d expired · %d revoked · %d unknown",
					summary.Summary.Total, summary.Summary.Healthy, summary.Summary.Degraded,
					summary.Summary.Expired, summary.Summary.Revoked, summary.Summary.Unknown)
			})
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-check the account live instead of reading the stored result")
	return cmd
}

func newAccountsValidateCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <account-id>",
		Short: "Check the account's stored credentials against the platform",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.Validate(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			if err := printer.Value(result, func() {
				printer.Fields([][2]string{
					{"ID", result.AccountID},
					{"Platform", result.Platform},
					{"Valid", boolLabel(result.Valid, "yes", "no")},
					{"Health", output.Dash(result.HealthStatus)},
				})
			}); err != nil {
				return err
			}
			if !result.Valid {
				return usageErrorf("the account's credentials are no longer valid — reconnect it in the dashboard")
			}
			return nil
		},
	}
}

func newAccountsRefreshCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh <account-id>",
		Short: "Renew the account's OAuth token ahead of its expiry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			refreshed, err := client.Accounts.RefreshToken(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(refreshed, func() {
				printer.Success("Token refreshed. Expires %s", output.Stamp(refreshed.ExpiresAt.Time, refreshed.ExpiresAt.Raw))
			})
		},
	}
}

func boolLabel(value bool, yes, no string) string {
	if value {
		return yes
	}
	return no
}
