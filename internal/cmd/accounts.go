package cmd

import (
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
		newAccountsHealthCmd(state),
		newAccountsValidateCmd(state),
		newAccountsRefreshCmd(state),
	)
	return cmd
}

func newAccountsListCmd(state *State) *cobra.Command {
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
			accounts, err := client.Accounts.List(cmd.Context(), resolved.Workspace)
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
					{"Workspace", account.Workspace.Name + " (" + account.WorkspaceID + ")"},
					{"Connected", output.Stamp(account.CreatedAt.Time, account.CreatedAt.Raw)},
				})
			})
		},
	}
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
