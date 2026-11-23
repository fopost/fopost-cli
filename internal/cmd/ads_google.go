package cmd

import (
	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAdsGoogleCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Conversion actions and GAQL on a Google Ads connection",
		Long: "What only Google has. Insights are on the other ads commands and work\n" +
			"across networks.",
	}
	cmd.AddCommand(
		newGoogleConversionsCmd(state),
		newGoogleQueryCmd(state),
	)
	return cmd
}

// googleFlags are on every command here: the connection and the Google account
// it names. A customer the connection's grant does not reach is not found.
type googleFlags struct {
	connection string
	customer   string
}

func (f *googleFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.connection, "connection", "", "Ad connection id (required)")
	cmd.Flags().StringVar(&f.customer, "customer", "", "Google Ads customer id, digits only (required)")
}

func (f *googleFlags) scope(state *State) (fopost.GoogleScope, error) {
	if f.connection == "" {
		return fopost.GoogleScope{}, usageErrorf("--connection is required")
	}
	if f.customer == "" {
		return fopost.GoogleScope{}, usageErrorf("--customer is required")
	}
	resolved, err := state.Resolved()
	if err != nil {
		return fopost.GoogleScope{}, err
	}
	return fopost.GoogleScope{
		WorkspaceID:  resolved.Workspace,
		ConnectionID: f.connection,
		CustomerID:   f.customer,
	}, nil
}

func newGoogleConversionsCmd(state *State) *cobra.Command {
	flags := &googleFlags{}
	cmd := &cobra.Command{
		Use:   "conversions",
		Short: "List the account's conversion actions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			scope, err := flags.scope(state)
			if err != nil {
				return err
			}
			actions, err := client.GoogleAds.ConversionActions(cmd.Context(), scope)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(actions, func() {
				rows := [][]string{}
				for _, action := range actions {
					rows = append(rows, []string{
						action.ID,
						output.Truncate(action.Name, 32),
						action.Category,
						action.Status,
						minorAmount(action.ValueMinor),
					})
				}
				printer.Table([]string{"id", "name", "category", "status", "value"}, rows)
			})
		},
	}
	flags.bind(cmd)
	return cmd
}

func newGoogleQueryCmd(state *State) *cobra.Command {
	flags := &googleFlags{}
	cmd := &cobra.Command{
		Use:   "query <gaql>",
		Short: "Run a read-only GAQL SELECT",
		Long: "The account read is --customer, never anything named inside the query.\n" +
			"Rows come back exactly as Google returns them, so this prints JSON.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			scope, err := flags.scope(state)
			if err != nil {
				return err
			}
			rows, err := client.GoogleAds.Query(cmd.Context(), &fopost.GoogleQueryRequest{
				GoogleScope: scope,
				Query:       args[0],
			})
			if err != nil {
				return err
			}
			return state.Printer().EmitJSON(rows)
		},
	}
	flags.bind(cmd)
	return cmd
}
