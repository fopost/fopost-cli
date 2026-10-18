package cmd

import (
	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newXCmd) }

// Pass-through reads on X with a connected account's token. Every call is an
// X API call metered to the workspace.
func newXCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "x",
		Aliases: []string{"twitter"},
		Short:   "Look up and search X posts with a connected account",
	}
	cmd.AddCommand(newXPostCmd(state), newXSearchCmd(state))
	return cmd
}

func newXPostCmd(state *State) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "post <post-id>",
		Short: "Show one X post by its id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			post, err := client.Inbox.GetXPost(cmd.Context(), args[0], account)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(post, func() {
				printer.Fields([][2]string{
					{"ID", post.ID},
					{"Author", output.Dash(deref(post.AuthorName))},
					{"Handle", output.Dash(deref(post.AuthorHandle))},
					{"Text", output.Dash(deref(post.Text))},
					{"Likes", intLabel(post.Likes)},
					{"Replies", intLabel(post.Replies)},
					{"Reposts", intLabel(post.Reposts)},
					{"Quotes", intLabel(post.Quotes)},
					{"Posted", xStamp(post.PlatformCreatedAt)},
					{"Permalink", output.Dash(deref(post.Permalink))},
				})
			})
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "X account whose token does the read")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newXSearchCmd(state *State) *cobra.Command {
	var account, query, nextToken string
	var maxResults int
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search recent X posts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			page, err := client.Inbox.SearchXPosts(cmd.Context(), &fopost.XSearchParams{
				AccountID:  account,
				Q:          query,
				MaxResults: maxResults,
				NextToken:  nextToken,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(page, func() {
				rows := make([][]string, 0, len(page.Data))
				for _, p := range page.Data {
					rows = append(rows, []string{
						p.ID,
						output.Dash(deref(p.AuthorHandle)),
						output.Truncate(output.Dash(deref(p.Text)), 60),
						intLabel(p.Likes),
						intLabel(p.Reposts),
						xStamp(p.PlatformCreatedAt),
					})
				}
				printer.Table([]string{"id", "handle", "text", "likes", "reposts", "posted"}, rows)
				if page.Meta.NextToken != nil && *page.Meta.NextToken != "" {
					printer.Line("Next page: --next-token %s", *page.Meta.NextToken)
				}
			})
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "X account whose token does the search")
	cmd.Flags().StringVar(&query, "q", "", "Search query, up to 512 characters")
	cmd.Flags().IntVar(&maxResults, "max-results", 0, "Posts per page, 10 to 100")
	cmd.Flags().StringVar(&nextToken, "next-token", "", "Cursor from the previous page")
	_ = cmd.MarkFlagRequired("account")
	_ = cmd.MarkFlagRequired("q")
	return cmd
}

func xStamp(t *fopost.Time) string {
	if t == nil {
		return "-"
	}
	return output.Stamp(t.Time, t.Raw)
}
