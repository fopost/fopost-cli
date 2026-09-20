package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newAdsCmd) }

func newAdsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ads",
		Short: "Inspect campaigns, change their status, and read insights, leads and ad comments",
	}
	cmd.AddCommand(
		newAdsTreeCmd(state),
		newAdsStatusCmd(state, "pause", fopost.AdStatusPaused),
		newAdsStatusCmd(state, "resume", fopost.AdStatusActive),
		newAdsInsightsCmd(state),
		newAdsLeadsCmd(state),
		newAdsIdentitiesCmd(state),
		newAdsSparkPostsCmd(state),
		newAdsCommentsCmd(state),
		newAdsReplyCmd(state),
		newAdsCommentStateCmd(state, "hide", true),
		newAdsCommentStateCmd(state, "unhide", false),
		newAdsDeleteCommentCmd(state),
	)
	return cmd
}

// The client and the resolved workspace, the two every ads command needs.
func (s *State) clientAndWorkspace() (*fopost.Client, string, error) {
	client, err := s.Client()
	if err != nil {
		return nil, "", err
	}
	resolved, err := s.Resolved()
	if err != nil {
		return nil, "", err
	}
	return client, resolved.Workspace, nil
}

func minorAmount(minor *int) string {
	if minor == nil {
		return "-"
	}
	return fmt.Sprintf("%d.%02d", *minor/100, *minor%100)
}

func newAdsTreeCmd(state *State) *cobra.Command {
	var connection string
	cmd := &cobra.Command{
		Use:   "tree <ad-account-id>",
		Short: "Show every campaign on an ad account with its ad sets and ads",
		Args:  cobra.ExactArgs(1),
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
			tree, err := client.Ads.Tree(cmd.Context(), args[0], &fopost.AdObjectParams{
				WorkspaceID:  resolved.Workspace,
				ConnectionID: connection,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(tree, func() {
				rows := [][]string{}
				for _, campaign := range tree.Campaigns {
					rows = append(rows, []string{"campaign", campaign.ID, output.Truncate(campaign.Name, 32), campaign.Status, minorAmount(campaign.BudgetMinor)})
					for _, set := range campaign.AdSets {
						rows = append(rows, []string{"  ad set", set.ID, output.Truncate(set.Name, 32), set.Status, minorAmount(set.BudgetMinor)})
						for _, ad := range set.Ads {
							rows = append(rows, []string{"    ad", ad.ID, output.Truncate(ad.Name, 32), ad.Status, "-"})
						}
					}
				}
				printer.Table([]string{"level", "id", "name", "status", "budget"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "Meta Ads connection id (required)")
	return cmd
}

func newAdsStatusCmd(state *State, verb, status string) *cobra.Command {
	var connection string
	var campaigns, adSets, ads []string
	cmd := &cobra.Command{
		Use:   verb,
		Short: strings.ToUpper(verb[:1]) + verb[1:] + " campaigns, ad sets and ads",
		Example: strings.Join([]string{
			"  fopost ads " + verb + " --connection conn_1 --campaign 1201 --ad-set 1202",
			"  fopost ads " + verb + " --connection conn_1 --ad 1203 --ad 1204",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if connection == "" {
				return usageErrorf("--connection is required")
			}
			objects := make([]fopost.AdObjectRef, 0, len(campaigns)+len(adSets)+len(ads))
			for _, id := range campaigns {
				objects = append(objects, fopost.AdObjectRef{ID: id, Level: fopost.AdLevelCampaign})
			}
			for _, id := range adSets {
				objects = append(objects, fopost.AdObjectRef{ID: id, Level: fopost.AdLevelAdSet})
			}
			for _, id := range ads {
				objects = append(objects, fopost.AdObjectRef{ID: id, Level: fopost.AdLevelAd})
			}
			if len(objects) == 0 {
				return usageErrorf("at least one --campaign, --ad-set or --ad is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			results, err := client.Ads.SetStatuses(cmd.Context(), &fopost.SetStatusesRequest{
				WorkspaceID:  workspaceID,
				ConnectionID: connection,
				Status:       status,
				Objects:      objects,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(results, func() {
				rows := make([][]string, 0, len(results))
				for _, result := range results {
					rows = append(rows, []string{result.Level, result.ID, boolLabel(result.OK, "ok", "failed"), output.Dash(result.Error)})
				}
				printer.Table([]string{"level", "id", "result", "error"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "Meta Ads connection id (required)")
	cmd.Flags().StringArrayVar(&campaigns, "campaign", nil, "campaign id (repeatable)")
	cmd.Flags().StringArrayVar(&adSets, "ad-set", nil, "ad set id (repeatable)")
	cmd.Flags().StringArrayVar(&ads, "ad", nil, "ad id (repeatable)")
	return cmd
}

func printInsightsMetrics(printer *output.Printer, currency string, m *fopost.InsightsMetrics) {
	spend := m.SpendMinor
	printer.Fields([][2]string{
		{"Impressions", itoa(m.Impressions)},
		{"Reach", itoa(m.Reach)},
		{"Clicks", itoa(m.Clicks)},
		{"CTR", fmt.Sprintf("%.2f%%", m.CTR)},
		{"Spend", strings.TrimSpace(minorAmount(&spend) + " " + currency)},
		{"Leads", itoa(m.Leads)},
	})
}

func newAdsInsightsCmd(state *State) *cobra.Command {
	var connection, since, until, breakdown string
	var daily bool
	cmd := &cobra.Command{
		Use:   "insights <object-id>",
		Short: "Show delivery for an ad account, campaign, ad set or ad over a date range",
		Example: strings.Join([]string{
			"  fopost ads insights act_123 --connection conn_1 --since 2026-09-01 --until 2026-09-07",
			"  fopost ads insights 1201 --connection conn_1 --since 2026-09-01 --until 2026-09-07 --breakdown age",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if connection == "" || since == "" || until == "" {
				return usageErrorf("--connection, --since and --until are required")
			}
			switch breakdown {
			case "", fopost.InsightsByAge, fopost.InsightsByGender, fopost.InsightsByPlacement, fopost.InsightsByCountry:
			default:
				return usageErrorf("--breakdown must be age, gender, placement or country")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			params := &fopost.InsightsParams{
				WorkspaceID:  resolved.Workspace,
				ConnectionID: connection,
				ObjectID:     args[0],
				Since:        since,
				Until:        until,
				Breakdown:    breakdown,
			}
			if daily {
				params.Daily = fopost.Bool(true)
			}
			report, err := client.Ads.Insights(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(report, func() {
				if report.Totals == nil {
					printer.Line("No delivery between %s and %s.", report.Since, report.Until)
					return
				}
				printInsightsMetrics(printer, report.Currency, report.Totals)
				if len(report.Breakdown) > 0 {
					rows := make([][]string, 0, len(report.Breakdown))
					for _, row := range report.Breakdown {
						spend := row.Metrics.SpendMinor
						rows = append(rows, []string{row.Key, itoa(row.Metrics.Impressions), itoa(row.Metrics.Clicks), minorAmount(&spend)})
					}
					printer.Table([]string{report.BreakdownBy, "impressions", "clicks", "spend"}, rows)
				}
				if len(report.Timeline) > 0 {
					rows := make([][]string, 0, len(report.Timeline))
					for _, day := range report.Timeline {
						spend := day.Metrics.SpendMinor
						rows = append(rows, []string{day.Date, itoa(day.Metrics.Impressions), itoa(day.Metrics.Clicks), minorAmount(&spend)})
					}
					printer.Table([]string{"date", "impressions", "clicks", "spend"}, rows)
				}
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "Meta Ads connection id (required)")
	cmd.Flags().StringVar(&since, "since", "", "first day, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&until, "until", "", "last day, YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&breakdown, "breakdown", "", "split by age, gender, placement or country")
	cmd.Flags().BoolVar(&daily, "daily", false, "add a per-day timeline")
	return cmd
}

func leadSummary(fields []fopost.LeadField) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field.Name+"="+strings.Join(field.Values, "|"))
	}
	return strings.Join(parts, " ")
}

func newAdsLeadsCmd(state *State) *cobra.Command {
	var form, page, cursor string
	var limit int
	cmd := &cobra.Command{
		Use:   "leads",
		Short: "List leads collected from subscribed Pages, newest first",
		Long:  "Lists one page of the leads feed. Pass the printed next cursor back as --cursor for the next page.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit < 0 || limit > 100 {
				return usageErrorf("--limit must be between 1 and 100")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			feed, err := client.Ads.LeadsFeed(cmd.Context(), &fopost.LeadsFeedParams{
				WorkspaceID: resolved.Workspace,
				FormID:      form,
				PageID:      page,
				Cursor:      cursor,
				Limit:       limit,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(feed, func() {
				rows := make([][]string, 0, len(feed.Leads))
				for _, lead := range feed.Leads {
					rows = append(rows, []string{
						lead.ID,
						output.Stamp(lead.SubmittedAt.Time, lead.SubmittedAt.Raw),
						output.Dash(lead.FormID),
						output.Truncate(output.Dash(lead.CampaignName), 24),
						output.Truncate(leadSummary(lead.Fields), 48),
					})
				}
				printer.Table([]string{"id", "submitted", "form", "campaign", "fields"}, rows)
				if feed.NextCursor != "" {
					printer.Line("Next cursor: %s", feed.NextCursor)
				}
			})
		},
	}
	cmd.Flags().StringVar(&form, "form", "", "only leads from this lead form")
	cmd.Flags().StringVar(&page, "page", "", "only leads from this Page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "the next cursor of the previous page")
	cmd.Flags().IntVar(&limit, "limit", 0, "leads per page, 1 to 100")
	return cmd
}

func newAdsIdentitiesCmd(state *State) *cobra.Command {
	var connection, adAccount string
	cmd := &cobra.Command{
		Use:   "identities",
		Short: "List the TikTok identities an ad can run as",
		Long: "An identity id is what every other ads command calls a page id. " +
			"TikTok is the only network with this read; others answer unsupported.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if connection == "" || adAccount == "" {
				return usageErrorf("--connection and --ad-account are required")
			}
			client, resolved, err := state.clientAndWorkspace()
			if err != nil {
				return err
			}
			identities, err := client.Ads.TikTokIdentities(cmd.Context(), &fopost.ListAudiencesParams{
				WorkspaceID:  resolved,
				ConnectionID: connection,
				AdAccountID:  adAccount,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(identities, func() {
				rows := make([][]string, 0, len(identities))
				for _, identity := range identities {
					rows = append(rows, []string{identity.ID, output.Truncate(identity.Name, 32), identity.Type})
				}
				printer.Table([]string{"id", "name", "type"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "ads connection id (required)")
	cmd.Flags().StringVar(&adAccount, "ad-account", "", "ad account id (required)")
	return cmd
}

func newAdsSparkPostsCmd(state *State) *cobra.Command {
	var connection, adAccount, identity string
	cmd := &cobra.Command{
		Use:   "spark-posts",
		Short: "List posts already live under an identity, each a candidate Spark ad",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if connection == "" || adAccount == "" || identity == "" {
				return usageErrorf("--connection, --ad-account and --identity are required")
			}
			client, resolved, err := state.clientAndWorkspace()
			if err != nil {
				return err
			}
			posts, err := client.Ads.SparkPosts(cmd.Context(), &fopost.ListSparkPostsParams{
				WorkspaceID:  resolved,
				ConnectionID: connection,
				AdAccountID:  adAccount,
				IdentityID:   identity,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(posts, func() {
				rows := make([][]string, 0, len(posts))
				for _, post := range posts {
					views := "-"
					if post.Views != nil {
						views = fmt.Sprintf("%d", *post.Views)
					}
					rows = append(rows, []string{post.ID, output.Truncate(post.Caption, 48), views})
				}
				printer.Table([]string{"id", "caption", "views"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "ads connection id (required)")
	cmd.Flags().StringVar(&adAccount, "ad-account", "", "ad account id (required)")
	cmd.Flags().StringVar(&identity, "identity", "", "identity id (required)")
	return cmd
}

func newAdsCommentsCmd(state *State) *cobra.Command {
	var connection, ad, cursor string
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "List the comments on an ad, read live from the network",
		Long:  "Pass the printed next cursor back as --cursor for the next page.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if connection == "" || ad == "" {
				return usageErrorf("--connection and --ad are required")
			}
			client, resolved, err := state.clientAndWorkspace()
			if err != nil {
				return err
			}
			page, err := client.Ads.Comments(cmd.Context(), &fopost.ListAdCommentsParams{
				WorkspaceID:  resolved,
				ConnectionID: connection,
				AdID:         ad,
				After:        cursor,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(page, func() {
				rows := make([][]string, 0, len(page.Comments))
				for _, comment := range page.Comments {
					state := "public"
					if comment.Hidden {
						state = "hidden"
					}
					rows = append(rows, []string{
						comment.ID,
						output.Truncate(output.Dash(comment.AuthorName), 20),
						output.Truncate(comment.Text, 48),
						state,
					})
				}
				printer.Table([]string{"id", "author", "text", "state"}, rows)
				if page.NextCursor != "" {
					printer.Line("Next cursor: %s", page.NextCursor)
				}
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "ads connection id (required)")
	cmd.Flags().StringVar(&ad, "ad", "", "the ad whose comments to read (required)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "the next cursor of the previous page")
	return cmd
}

func newAdsReplyCmd(state *State) *cobra.Command {
	var connection, ad, text string
	cmd := &cobra.Command{
		Use:   "reply <comment-id>",
		Short: "Answer a comment on an ad",
		Long:  "The reply is published under the ad's identity. Needs the publish scope as well as ads.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if connection == "" || ad == "" || text == "" {
				return usageErrorf("--connection, --ad and --text are required")
			}
			client, resolved, err := state.clientAndWorkspace()
			if err != nil {
				return err
			}
			replyID, err := client.Ads.ReplyToComment(cmd.Context(), args[0], &fopost.AdCommentRequest{
				WorkspaceID:  resolved,
				ConnectionID: connection,
				AdID:         ad,
				Text:         text,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]string{"replyId": replyID}, func() {
				printer.Line("Replied: %s", replyID)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "ads connection id (required)")
	cmd.Flags().StringVar(&ad, "ad", "", "the ad the comment sits on (required)")
	cmd.Flags().StringVar(&text, "text", "", "what to say (required)")
	return cmd
}

func newAdsCommentStateCmd(state *State, verb string, hidden bool) *cobra.Command {
	var connection, ad string
	cmd := &cobra.Command{
		Use:   verb + " <comment-id>",
		Short: strings.ToUpper(verb[:1]) + verb[1:] + " a comment on an ad",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if connection == "" || ad == "" {
				return usageErrorf("--connection and --ad are required")
			}
			client, resolved, err := state.clientAndWorkspace()
			if err != nil {
				return err
			}
			body := &fopost.AdCommentRequest{
				WorkspaceID:  resolved,
				ConnectionID: connection,
				AdID:         ad,
				Hidden:       fopost.Bool(hidden),
			}
			if err := client.Ads.SetCommentHidden(cmd.Context(), args[0], body); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]string{"id": args[0]}, func() {
				printer.Line("%sd %s", strings.ToUpper(verb[:1])+verb[1:], args[0])
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "ads connection id (required)")
	cmd.Flags().StringVar(&ad, "ad", "", "the ad the comment sits on (required)")
	return cmd
}

func newAdsDeleteCommentCmd(state *State) *cobra.Command {
	var connection, ad string
	cmd := &cobra.Command{
		Use:   "delete-comment <comment-id>",
		Short: "Remove a comment from the ad on the network",
		Long:  "One already gone succeeds. Needs the publish scope as well as ads.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if connection == "" || ad == "" {
				return usageErrorf("--connection and --ad are required")
			}
			client, resolved, err := state.clientAndWorkspace()
			if err != nil {
				return err
			}
			body := &fopost.AdCommentRequest{
				WorkspaceID:  resolved,
				ConnectionID: connection,
				AdID:         ad,
			}
			if err := client.Ads.DeleteComment(cmd.Context(), args[0], body); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]string{"id": args[0]}, func() {
				printer.Line("Deleted %s", args[0])
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "ads connection id (required)")
	cmd.Flags().StringVar(&ad, "ad", "", "the ad the comment sits on (required)")
	return cmd
}
