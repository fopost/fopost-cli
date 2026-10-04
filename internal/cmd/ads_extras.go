package cmd

import (
	"strconv"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func count(n *int) string {
	if n == nil {
		return "-"
	}
	return strconv.Itoa(*n)
}

// newAdsCatalogsCmd lists the product catalogs a connection reaches, and the
// product sets inside one. A catalog ad runs from a set, not the whole catalog.
func newAdsCatalogsCmd(state *State) *cobra.Command {
	var connection, catalog string
	cmd := &cobra.Command{
		Use:   "catalogs",
		Short: "List product catalogs, or the product sets in one",
		Example: strings.Join([]string{
			"  fopost ads catalogs --connection conn_1",
			"  fopost ads catalogs --connection conn_1 --catalog 500123456789",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if connection == "" {
				return usageErrorf("--connection is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			params := &fopost.AdObjectParams{
				WorkspaceID:  resolved.Workspace,
				ConnectionID: connection,
			}
			printer := state.Printer()

			if catalog != "" {
				sets, err := client.Ads.ProductSets(cmd.Context(), catalog, params)
				if err != nil {
					return err
				}
				return printer.Value(sets, func() {
					rows := [][]string{}
					for _, set := range sets {
						rows = append(rows, []string{
							set.ID,
							output.Truncate(set.Name, 40),
							count(set.ProductCount),
						})
					}
					printer.Table([]string{"id", "name", "products"}, rows)
				})
			}

			result, err := client.Ads.Catalogs(cmd.Context(), params)
			if err != nil {
				return err
			}
			return printer.Value(result.Catalogs, func() {
				rows := [][]string{}
				for _, c := range result.Catalogs {
					rows = append(rows, []string{
						c.ID,
						output.Truncate(c.Name, 40),
						c.Vertical,
						count(c.ProductCount),
					})
				}
				printer.Table([]string{"id", "name", "vertical", "products"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "Meta Ads connection id (required)")
	cmd.Flags().StringVar(&catalog, "catalog", "", "Show the product sets in this catalog instead")
	return cmd
}

// newAdsLibraryCmd searches the public ad archive. Results are read live on
// every call and stored nowhere, so an ad that stops running is simply absent
// from the next search.
func newAdsLibraryCmd(state *State) *cobra.Command {
	var connection, country, page string
	var limit int
	cmd := &cobra.Command{
		Use:   "library [keyword]",
		Short: "Search the public ad archive for ads anyone is running",
		Args:  cobra.MaximumNArgs(1),
		Example: strings.Join([]string{
			"  fopost ads library --connection conn_1 --country US \"running shoes\"",
			"  fopost ads library --connection conn_1 --country GB --page 1234567890",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if connection == "" {
				return usageErrorf("--connection is required")
			}
			keyword := ""
			if len(args) == 1 {
				keyword = args[0]
			}
			if keyword == "" && page == "" {
				return usageErrorf("search by keyword or by --page")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			params := &fopost.LibraryParams{
				WorkspaceID:  resolved.Workspace,
				ConnectionID: connection,
				Countries:    []string{country},
				Q:            keyword,
				Limit:        limit,
			}
			if page != "" {
				params.PageIDs = []string{page}
			}
			result, err := client.Ads.Library(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result.Entries, func() {
				rows := [][]string{}
				for _, entry := range result.Entries {
					rows = append(rows, []string{
						entry.ID,
						output.Truncate(entry.PageName, 28),
						output.Truncate(strings.Join(entry.Bodies, " · "), 48),
						strings.Join(entry.PublisherPlatforms, ","),
					})
				}
				printer.Table([]string{"id", "page", "copy", "platforms"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "Meta Ads connection id (required)")
	cmd.Flags().StringVar(&country, "country", "US", "Two-letter code the ad reached")
	cmd.Flags().StringVar(&page, "page", "", "Search one Page instead of a keyword")
	cmd.Flags().IntVar(&limit, "limit", 25, "How many entries to return, 1-100")
	return cmd
}
