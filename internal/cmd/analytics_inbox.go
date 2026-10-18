package cmd

import (
	"fmt"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAnalyticsInboxCmd(state *State) *cobra.Command {
	params := &fopost.InboxAnalyticsParams{}
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Inbox volume, response time, and where it comes from",
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
			inbox, err := client.Analytics.Inbox(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(inbox, func() {
				median := ""
				if inbox.Totals.MedianResponseMinutes != nil {
					median = fmt.Sprintf("%.0f min", *inbox.Totals.MedianResponseMinutes)
				}
				printer.Fields([][2]string{
					{"Received", itoa(inbox.Totals.Received)},
					{"Sent", itoa(inbox.Totals.Sent)},
					{"Answered", itoa(inbox.Totals.Answered)},
					{"Open", itoa(inbox.Totals.Open)},
					{"Conversations", itoa(inbox.Totals.Conversations)},
					{"Median response", output.Dash(median)},
					{"Time zone", output.Dash(inbox.TimeZone)},
				})
				if len(inbox.ByPlatform) == 0 {
					return
				}
				printer.Line("")
				rows := make([][]string, 0, len(inbox.ByPlatform))
				for _, row := range inbox.ByPlatform {
					rows = append(rows, []string{row.Platform, itoa(row.Received), itoa(row.Sent)})
				}
				printer.Table([]string{"platform", "received", "sent"}, rows)
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&params.AccountID, "account", "", "narrow to one account id")
	flags.IntVar(&params.Days, "days", 0, "window length in days, 1 to 365 (the API defaults to 7)")
	flags.StringVar(&params.TZ, "tz", "", "IANA zone the series and heatmap are bucketed in (default UTC)")
	return cmd
}
