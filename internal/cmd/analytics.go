package cmd

import (
	"fmt"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newAnalyticsCmd) }

func newAnalyticsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Read cross-account performance numbers",
	}
	cmd.AddCommand(
		newAnalyticsOverviewCmd(state),
		newAnalyticsTopPostsCmd(state),
		newAnalyticsTimeSeriesCmd(state),
		newAnalyticsDecayCmd(state),
		newAnalyticsFrequencyCmd(state),
		newAnalyticsTimelineCmd(state),
		newAnalyticsChangesCmd(state),
		newAnalyticsCollectPostCmd(state),
		newAnalyticsNativePostsCmd(state),
	)
	return cmd
}

// analyticsFlags binds the window and scope every analytics command shares.
func analyticsFlags(cmd *cobra.Command, params *fopost.AnalyticsParams) {
	flags := cmd.Flags()
	flags.StringVar(&params.AccountID, "account", "", "narrow to one account id")
	flags.IntVar(&params.Days, "days", 0, "window length in days")
	flags.StringVar(&params.From, "from", "", "start of the window, YYYY-MM-DD")
	flags.StringVar(&params.To, "to", "", "end of the window, YYYY-MM-DD")
}

func newAnalyticsOverviewCmd(state *State) *cobra.Command {
	params := &fopost.AnalyticsParams{}
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Headline totals across every account in scope",
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
			params.WorkspaceID = resolved.Workspace
			overview, err := client.Analytics.Overview(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(overview, func() {
				printer.Fields([][2]string{
					{"Accounts", itoa(overview.TotalAccounts)},
					{"Followers", itoa(overview.TotalFollowers)},
					{"Posts", itoa(overview.TotalPosts)},
					{"Engagement", itoa(overview.TotalEngagement)},
					{"Impressions", itoa(overview.TotalImpressions)},
					{"Reach", itoa(overview.TotalReach)},
					{"Likes", itoa(overview.TotalLikes)},
					{"Comments", itoa(overview.TotalComments)},
					{"Shares", itoa(overview.TotalShares)},
					{"Engagement rate", percent(overview.EngagementRate)},
				})
				if len(overview.Platforms) == 0 {
					return
				}
				printer.Line("")
				rows := make([][]string, 0, len(overview.Platforms))
				for _, platform := range overview.Platforms {
					rows = append(rows, []string{platform.Platform, itoa(platform.Accounts), itoa(platform.Followers)})
				}
				printer.Table([]string{"platform", "accounts", "followers"}, rows)
			})
		},
	}
	analyticsFlags(cmd, params)
	return cmd
}

func newAnalyticsTopPostsCmd(state *State) *cobra.Command {
	params := &fopost.AnalyticsParams{}
	cmd := &cobra.Command{
		Use:   "top-posts",
		Short: "Best performing posts in the window",
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
			params.WorkspaceID = resolved.Workspace
			posts, err := client.Analytics.TopPosts(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(posts, func() {
				rows := make([][]string, 0, len(posts))
				for _, post := range posts {
					rows = append(rows, []string{
						itoa(post.Rank),
						output.Dash(post.PostID),
						output.Truncate(post.Preview, 40),
						itoa(post.Metrics.Engagements),
						itoa(post.Metrics.Impressions),
						itoa(post.Metrics.Likes),
					})
				}
				printer.Table([]string{"rank", "post id", "preview", "engagements", "impressions", "likes"}, rows)
			})
		},
	}
	analyticsFlags(cmd, params)
	cmd.Flags().IntVar(&params.Limit, "limit", 0, "how many posts to return")
	cmd.Flags().StringVar(&params.Sort, "sort", "", "\"recent\" to order by date instead of performance")
	cmd.Flags().StringVar(&params.Label, "label", "", "narrow to one campaign label")
	return cmd
}

func newAnalyticsTimeSeriesCmd(state *State) *cobra.Command {
	params := &fopost.AnalyticsParams{}
	cmd := &cobra.Command{
		Use:   "time-series",
		Short: "One point per day across the window",
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
			params.WorkspaceID = resolved.Workspace
			series, err := client.Analytics.TimeSeries(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(series, func() {
				rows := make([][]string, 0, len(series.Series))
				for _, point := range series.Series {
					rows = append(rows, []string{
						point.Date,
						itoa(point.Posts),
						itoa(point.Engagements),
						itoa(point.Impressions),
						itoa(point.Followers),
					})
				}
				printer.Table([]string{"date", "posts", "engagements", "impressions", "followers"}, rows)
			})
		},
	}
	analyticsFlags(cmd, params)
	return cmd
}

func percent(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", *value*100)
}
