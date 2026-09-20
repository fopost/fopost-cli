package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAdsGoogleCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Keywords, assets and GAQL on a Google Ads connection",
		Long: "The Search surface no other network has. Campaigns, ad groups, ads and\n" +
			"insights are on the other ads commands and work across networks.",
	}
	cmd.AddCommand(
		newGoogleKeywordsCmd(state),
		newGoogleKeywordIdeasCmd(state),
		newGoogleSearchTermsCmd(state),
		newGoogleAssetsCmd(state),
		newGoogleAssetGroupsCmd(state),
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

func newGoogleKeywordsCmd(state *State) *cobra.Command {
	flags := &googleFlags{}
	var adGroup string
	cmd := &cobra.Command{
		Use:   "keywords",
		Short: "List the keywords on an account, or on one ad group",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			scope, err := flags.scope(state)
			if err != nil {
				return err
			}
			var params *fopost.ListGoogleKeywordsParams
			if adGroup != "" {
				params = &fopost.ListGoogleKeywordsParams{AdGroupID: adGroup}
			}
			keywords, err := client.GoogleAds.Keywords(cmd.Context(), scope, params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(keywords, func() {
				rows := [][]string{}
				for _, keyword := range keywords {
					rows = append(rows, []string{
						keyword.ID,
						output.Truncate(keyword.Text, 32),
						keyword.MatchType,
						keyword.Status,
						minorAmount(keyword.CPCBidMinor),
					})
				}
				printer.Table([]string{"id", "keyword", "match", "status", "bid"}, rows)
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVar(&adGroup, "ad-group", "", "Limit to one ad group")
	return cmd
}

func newGoogleKeywordIdeasCmd(state *State) *cobra.Command {
	flags := &googleFlags{}
	var seeds []string
	var url string
	cmd := &cobra.Command{
		Use:   "keyword-ideas",
		Short: "Find keywords to bid on, from seed terms or a landing page",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(seeds) == 0 && url == "" {
				return usageErrorf("give at least one --seed or a --url")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			scope, err := flags.scope(state)
			if err != nil {
				return err
			}
			ideas, err := client.GoogleAds.KeywordIdeas(cmd.Context(), &fopost.GoogleKeywordIdeasRequest{
				GoogleScope: scope,
				Seeds:       seeds,
				URL:         url,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(ideas, func() {
				rows := [][]string{}
				for _, idea := range ideas {
					rows = append(rows, []string{
						output.Truncate(idea.Text, 40),
						fmt.Sprintf("%d", idea.AvgMonthlySearches),
						output.Dash(idea.Competition),
					})
				}
				printer.Table([]string{"keyword", "searches", "competition"}, rows)
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringSliceVar(&seeds, "seed", nil, "A seed keyword; repeatable")
	cmd.Flags().StringVar(&url, "url", "", "A landing page to read ideas from")
	return cmd
}

func newGoogleSearchTermsCmd(state *State) *cobra.Command {
	flags := &googleFlags{}
	var since, until string
	cmd := &cobra.Command{
		Use:   "search-terms",
		Short: "Read what people actually searched, with what each term earned",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if since == "" || until == "" {
				return usageErrorf("--since and --until are required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			scope, err := flags.scope(state)
			if err != nil {
				return err
			}
			terms, err := client.GoogleAds.SearchTerms(
				cmd.Context(), scope, fopost.GoogleDateRange{Since: since, Until: until})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(terms, func() {
				rows := [][]string{}
				for _, term := range terms {
					spend := term.Metrics.SpendMinor
					rows = append(rows, []string{
						output.Truncate(term.Term, 40),
						output.Dash(term.Status),
						fmt.Sprintf("%d", term.Metrics.Impressions),
						fmt.Sprintf("%d", term.Metrics.Clicks),
						minorAmount(&spend),
					})
				}
				printer.Table([]string{"term", "status", "impressions", "clicks", "spend"}, rows)
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVar(&since, "since", "", "Start date, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&until, "until", "", "End date, YYYY-MM-DD (required)")
	return cmd
}

func newGoogleAssetsCmd(state *State) *cobra.Command {
	flags := &googleFlags{}
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "List sitelinks, callouts and snippets, and where each one is attached",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			scope, err := flags.scope(state)
			if err != nil {
				return err
			}
			result, err := client.GoogleAds.Assets(cmd.Context(), scope)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				attached := map[string]int{}
				for _, link := range result.Links {
					attached[link.AssetID]++
				}
				rows := [][]string{}
				for _, asset := range result.Assets {
					where := "library"
					if count := attached[asset.ID]; count > 0 {
						where = fmt.Sprintf("%d place(s)", count)
					}
					rows = append(rows, []string{
						asset.ID,
						asset.Type,
						output.Truncate(asset.Text, 28),
						output.Dash(asset.FinalURL),
						where,
					})
				}
				printer.Table([]string{"id", "type", "text", "destination", "attached"}, rows)
			})
		},
	}
	flags.bind(cmd)
	return cmd
}

func newGoogleAssetGroupsCmd(state *State) *cobra.Command {
	flags := &googleFlags{}
	var campaign string
	cmd := &cobra.Command{
		Use:   "asset-groups",
		Short: "List Performance Max asset groups",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			scope, err := flags.scope(state)
			if err != nil {
				return err
			}
			var params *fopost.ListGoogleAssetGroupsParams
			if campaign != "" {
				params = &fopost.ListGoogleAssetGroupsParams{CampaignID: campaign}
			}
			groups, err := client.GoogleAds.AssetGroups(cmd.Context(), scope, params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(groups, func() {
				rows := [][]string{}
				for _, group := range groups {
					rows = append(rows, []string{
						group.ID,
						output.Truncate(group.Name, 32),
						group.Status,
						output.Dash(strings.Join(group.FinalURLs, ", ")),
					})
				}
				printer.Table([]string{"id", "name", "status", "destination"}, rows)
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVar(&campaign, "campaign", "", "Limit to one campaign")
	return cmd
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
