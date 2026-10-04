package cmd

import (
	"fmt"
	"strconv"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAnalyticsDecayCmd(state *State) *cobra.Command {
	params := &fopost.AnalyticsParams{}
	cmd := &cobra.Command{
		Use:   "decay",
		Short: "How long a post keeps earning after it goes out",
		Long: "Engagement grouped by the post's age at each reading, so every band says " +
			"where the average post had got to by then. --days selects posts by publish " +
			"time, not reading time.",
		Args: cobra.NoArgs,
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
			decay, err := client.Analytics.Decay(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(decay, func() {
				printer.Fields([][2]string{
					{"Posts measured", itoa(decay.PostsMeasured)},
					{"Half of final engagement by", output.Dash(decay.HalfLifeBucket)},
				})
				printer.Line("")
				rows := make([][]string, 0, len(decay.Bands))
				for _, band := range decay.Bands {
					if band.Posts == 0 {
						continue
					}
					rows = append(rows, []string{
						band.Label,
						itoa(band.Posts),
						fmt.Sprintf("%.0f", band.AvgEngagements),
						fmt.Sprintf("%.0f", band.AvgImpressions),
						percent(band.ShareOfFinal),
					})
				}
				printer.Table(
					[]string{"age", "posts", "engagements", "impressions", "share of final"},
					rows,
				)
			})
		},
	}
	analyticsFlags(cmd, params)
	return cmd
}

func newAnalyticsFrequencyCmd(state *State) *cobra.Command {
	params := &fopost.AnalyticsParams{}
	cmd := &cobra.Command{
		Use:   "frequency",
		Short: "What each posting cadence earned per post",
		Long: "Weeks run Monday to Sunday in UTC and are grouped by their own post count, " +
			"so a four-post week is compared against other four-post weeks rather than " +
			"the average. A week with no posts belongs to no band.",
		Args: cobra.NoArgs,
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
			cadence, err := client.Analytics.Frequency(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(cadence, func() {
				best := "-"
				if cadence.Best != nil {
					best = cadence.Best.Label
				}
				printer.Fields([][2]string{
					{"Weeks posted", itoa(len(cadence.Weeks))},
					{"Best cadence", best},
				})
				printer.Line("")
				rows := make([][]string, 0, len(cadence.Bands))
				for _, band := range cadence.Bands {
					if band.Posts == 0 {
						continue
					}
					rows = append(rows, []string{
						band.Label,
						itoa(band.Weeks),
						itoa(band.Posts),
						fmt.Sprintf("%.0f", band.AvgEngagementsPerPost),
						percent(band.EngagementRate),
					})
				}
				printer.Table(
					[]string{"cadence", "weeks", "posts", "per post", "engagement rate"},
					rows,
				)
			})
		},
	}
	analyticsFlags(cmd, params)
	return cmd
}

func newAnalyticsTimelineCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "timeline <post-id-or-permalink>",
		Short: "Every reading held for one post, oldest first",
		Long: "One timeline per delivery, because the same post on two networks decays " +
			"differently. The argument is a FoPost post id, or the permalink of a post " +
			"made natively on the network.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			timeline, err := client.Analytics.Timeline(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(timeline, func() {
				for i, delivery := range timeline.Deliveries {
					if i > 0 {
						printer.Line("")
					}
					printer.Line(fmt.Sprintf("%s @%s", delivery.Platform, delivery.Username))
					rows := make([][]string, 0, len(delivery.Points))
					for _, point := range delivery.Points {
						rows = append(rows, []string{
							age(point.AgeMinutes),
							optional(point.Engagements),
							optional(point.Impressions),
							itoa(point.Delta.Engagements),
						})
					}
					printer.Table([]string{"age", "engagements", "impressions", "moved"}, rows)
				}
			})
		},
	}
}

func newAnalyticsChangesCmd(state *State) *cobra.Command {
	params := &fopost.MetricChangesParams{}
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Metric readings recorded since a timestamp",
		Long: "Oldest first, with a cursor to pass as the next --since. This is how an " +
			"external store mirrors the metrics without refetching the whole history. " +
			"Without --since it answers with the last seven days.",
		Args: cobra.NoArgs,
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
			page, err := client.Analytics.Changes(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(page, func() {
				rows := make([][]string, 0, len(page.Changes))
				for _, change := range page.Changes {
					rows = append(rows, []string{
						change.FetchedAt.String(),
						change.Platform,
						output.Dash(change.PostID),
						change.ExternalPostID,
						optional(change.Engagements),
					})
				}
				printer.Table(
					[]string{"read at", "platform", "post id", "external id", "engagements"},
					rows,
				)
				printer.Line("")
				printer.Fields([][2]string{
					{"Cursor", output.Dash(page.Cursor.String())},
					{"More", strconv.FormatBool(page.HasMore)},
				})
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&params.Since, "since", "", "return readings recorded after this RFC 3339 instant")
	flags.IntVar(&params.Limit, "limit", 0, "how many readings to return")
	flags.StringVar(&params.AccountID, "account", "", "narrow to one account id")
	return cmd
}

func newAnalyticsCollectPostCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "collect-post <post-id-or-permalink>",
		Short: "Refresh one post's metrics now",
		Long: "Re-reads one post from the network instead of waiting for the next " +
			"scheduled collection. One post is still a platform call, so this spends the " +
			"same per-user budget as a full collection run.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Analytics.CollectPost(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				rows := make([][]string, 0, len(result.Deliveries))
				for _, delivery := range result.Deliveries {
					rows = append(rows, []string{
						delivery.Platform,
						delivery.ExternalPostID,
						strconv.FormatBool(delivery.Collected),
						output.Dash(delivery.Message),
					})
				}
				printer.Table([]string{"platform", "external id", "refreshed", "note"}, rows)
			})
		},
	}
}

func newAnalyticsNativePostsCmd(state *State) *cobra.Command {
	params := &fopost.NativePostsParams{}
	var accountID string
	cmd := &cobra.Command{
		Use:   "native-posts",
		Short: "Posts on an account that never went out through FoPost",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			list, err := client.Analytics.NativePosts(cmd.Context(), accountID, params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(list, func() {
				rows := make([][]string, 0, len(list.Data))
				for _, post := range list.Data {
					rows = append(rows, []string{
						post.PostedAt.String(),
						output.Truncate(post.Text, 40),
						optional(post.Metrics.Engagements),
						output.Dash(post.Permalink),
					})
				}
				printer.Table([]string{"posted", "text", "engagements", "permalink"}, rows)
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&accountID, "account", "", "the account id to list (required)")
	flags.IntVar(&params.Page, "page", 0, "page number")
	flags.IntVar(&params.PerPage, "per-page", 0, "page size")
	flags.IntVar(&params.Days, "days", 0, "only posts published in the last this many days")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

// age renders a reading's age in the largest unit that stays readable.
func age(minutes *int) string {
	if minutes == nil {
		return "-"
	}
	switch m := *minutes; {
	case m < 60:
		return fmt.Sprintf("%dm", m)
	case m < 48*60:
		return fmt.Sprintf("%dh", m/60)
	default:
		return fmt.Sprintf("%dd", m/(24*60))
	}
}

// optional renders a metric the network did not report as a dash, not zero.
func optional(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}
