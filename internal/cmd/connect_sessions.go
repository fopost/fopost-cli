package cmd

import (
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newConnectSessionsCmd) }

func newConnectSessionsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connect-sessions",
		Aliases: []string{"connect-session"},
		Short:   "Mint and inspect hosted connect links",
		Long: "A connect session is a single-use page, open for one hour, where one of your\n" +
			"end users connects a social account into your workspace with no FoPost login.",
	}
	cmd.AddCommand(
		newConnectSessionsCreateCmd(state),
		newConnectSessionsGetCmd(state),
	)
	return cmd
}

func newConnectSessionsCreateCmd(state *State) *cobra.Command {
	var (
		externalUserID string
		platforms      []string
		redirectURL    string
		brandName      string
		brandLogoURL   string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Mint a hosted connect link for one of your users",
		Long: "Mints a link and prints it once. --redirect-url must sit on a domain in the\n" +
			"workspace redirect allowlist.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if externalUserID == "" {
				return usageErrorf("--external-user-id is required")
			}
			if len(platforms) == 0 {
				return usageErrorf("at least one --platform is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			body := &fopost.CreateConnectSessionRequest{
				WorkspaceID:    workspaceID,
				ExternalUserID: externalUserID,
				Platforms:      platforms,
				RedirectURL:    redirectURL,
			}
			if brandName != "" || brandLogoURL != "" {
				body.Branding = &fopost.CreateConnectSessionBranding{Name: brandName, LogoURL: brandLogoURL}
			}
			session, err := client.ConnectSessions.Create(cmd.Context(), body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(session, func() {
				printer.Success("Created connect session %s.", session.ID)
				printer.Fields([][2]string{
					{"URL", session.URL},
					{"Platforms", strings.Join(session.Platforms, ", ")},
					{"Expires", output.Stamp(session.ExpiresAt.Time, session.ExpiresAt.Raw)},
				})
			})
		},
	}
	cmd.Flags().StringVar(&externalUserID, "external-user-id", "", "your own id for the user (required)")
	cmd.Flags().StringArrayVar(&platforms, "platform", nil, "network the user may pick (repeatable, at least one required)")
	cmd.Flags().StringVar(&redirectURL, "redirect-url", "", "where to send the user once the account is connected")
	cmd.Flags().StringVar(&brandName, "branding-name", "", "name the hosted page shows")
	cmd.Flags().StringVar(&brandLogoURL, "branding-logo-url", "", "logo the hosted page shows")
	return cmd
}

func newConnectSessionsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <session-id>",
		Short: "Show a connect session's status and the account it connected",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			session, err := client.ConnectSessions.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(session, func() {
				printer.Fields([][2]string{
					{"ID", session.ID},
					{"Status", session.Status},
					{"External user", session.ExternalUserID},
					{"Platforms", strings.Join(session.Platforms, ", ")},
					{"Account", output.Dash(strValue(session.AccountID))},
					{"Expires", output.Stamp(session.ExpiresAt.Time, session.ExpiresAt.Raw)},
					{"Used", output.Stamp(session.UsedAt.Time, session.UsedAt.Raw)},
				})
			})
		},
	}
}
