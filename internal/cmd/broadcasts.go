package cmd

import (
	"fmt"
	"strconv"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() {
	register(newBroadcastsCmd)
	register(newSequencesCmd)
}

func newBroadcastsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "broadcasts",
		Aliases: []string{"broadcast"},
		Short:   "Write and send one message into many conversations",
		Long: "Write and send one message into many conversations.\n\n" +
			"A broadcast is not a post and not a cold DM: every message lands in a " +
			"direct-message thread the contact already started. Nothing is sent into a " +
			"closed messaging window — Messenger and Instagram take a business-initiated " +
			"message only within 24 hours of the contact's last one, so recipients outside " +
			"it are skipped with window_closed rather than attempted. The number sent is " +
			"therefore often lower than the audience, and that is correct rather than a " +
			"failure; `broadcasts recipients --status skipped` says who and why.",
	}
	cmd.AddCommand(
		newBroadcastsListCmd(state),
		newBroadcastsGetCmd(state),
		newBroadcastsCreateCmd(state),
		newBroadcastsSendCmd(state),
		newBroadcastsCancelCmd(state),
		newBroadcastsRecipientsCmd(state),
		newBroadcastsDeleteCmd(state),
	)
	return cmd
}

func newBroadcastsListCmd(state *State) *cobra.Command {
	var status string
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List broadcasts, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			list, err := client.Broadcasts.List(cmd.Context(), &fopost.ListBroadcastsParams{
				WorkspaceID: workspaceID,
				Status:      status,
				Page:        page,
				PerPage:     perPage,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(list, func() {
				rows := make([][]string, 0, len(list.Data))
				for _, broadcast := range list.Data {
					rows = append(rows, []string{
						broadcast.ID,
						output.Truncate(broadcast.Name, 28),
						broadcast.Status,
						strconv.Itoa(broadcast.Counts.Sent),
						strconv.Itoa(broadcast.Counts.Skipped),
						broadcast.ScheduledAt.String(),
					})
				}
				printer.Table(
					[]string{"id", "name", "status", "sent", "skipped", "scheduled"},
					rows,
				)
			})
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "draft, scheduled, sending, sent, or cancelled")
	cmd.Flags().IntVar(&page, "page", 0, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "results per page, up to 100")
	return cmd
}

func newBroadcastsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <broadcast-id>",
		Short: "Show one broadcast, its message and what became of it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			broadcast, err := client.Broadcasts.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(broadcast, func() {
				printer.Table([]string{"field", "value"}, [][]string{
					{"name", broadcast.Name},
					{"status", broadcast.Status},
					{"message", output.Truncate(broadcast.Text, 60)},
					{"sent", strconv.Itoa(broadcast.Counts.Sent)},
					{"skipped", strconv.Itoa(broadcast.Counts.Skipped)},
					{"failed", strconv.Itoa(broadcast.Counts.Failed)},
					{"scheduled", broadcast.ScheduledAt.String()},
					{"sent at", broadcast.SentAt.String()},
				})
			})
		},
	}
}

func newBroadcastsCreateCmd(state *State) *cobra.Command {
	var accountID, name, text, mediaID, scheduledAt string
	var platforms, labelIDs []string
	var source string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Write a broadcast without sending it",
		Long: "Write a broadcast without sending it. Give --scheduled-at to have it go " +
			"out on its own at that time, or run `broadcasts send` when you are ready. " +
			"With no audience flags it reaches every contact in the workspace.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			body := &fopost.CreateBroadcastRequest{
				WorkspaceID: workspaceID,
				AccountID:   accountID,
				Name:        name,
				Text:        text,
				MediaID:     mediaID,
			}
			if len(platforms) > 0 || len(labelIDs) > 0 || source != "" {
				body.Audience = &fopost.AudienceFilter{
					Platforms: platforms,
					LabelIDs:  labelIDs,
					Source:    source,
				}
			}
			when, err := parseSchedule(scheduledAt)
			if err != nil {
				return err
			}
			body.ScheduledAt = when
			broadcast, err := client.Broadcasts.Create(cmd.Context(), body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(broadcast, func() {
				printer.Success("Wrote broadcast %s. Nothing has been sent yet.", broadcast.ID)
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "the connected account the messages go out from")
	cmd.Flags().StringVar(&name, "name", "", "what you call it; never sent to anyone")
	cmd.Flags().StringVar(&text, "text", "", "the message itself")
	cmd.Flags().StringVar(&mediaID, "media", "", "a media library asset to attach")
	cmd.Flags().StringSliceVar(&platforms, "platform", nil, "only contacts with a handle on this network")
	cmd.Flags().StringSliceVar(&labelIDs, "label", nil, "only contacts carrying this label")
	cmd.Flags().StringVar(&source, "source", "", "only contacts from inbox, radar, or import")
	cmd.Flags().StringVar(&scheduledAt, "scheduled-at", "", "send it at this time instead of on demand")
	_ = cmd.MarkFlagRequired("account")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("text")
	return cmd
}

func newBroadcastsSendCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "send <broadcast-id>",
		Short: "Send a broadcast into every conversation its audience matches",
		Long: "Send a broadcast into every conversation its audience matches. Contacts a " +
			"network's messaging window has closed on are skipped rather than attempted, " +
			"so the number sent can be lower than the audience.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf(
					"Send broadcast %s? The messages go out and cannot be recalled.", args[0])
				if err := confirm(state, prompt); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			sent, err := client.Broadcasts.Send(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(sent, func() {
				printer.Success("Sending to %d contacts.", sent.Recipients)
				printer.Warn("Anyone whose messaging window has closed is skipped, " +
					"so fewer may be messaged. Run `fopost broadcasts recipients " +
					args[0] + " --status skipped` to see who.")
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newBroadcastsCancelCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <broadcast-id>",
		Short: "Stop a broadcast where it stands",
		Long: "Stop a broadcast where it stands. Anyone not yet written to stays unsent; " +
			"messages already delivered are not recalled.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			broadcast, err := client.Broadcasts.Cancel(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(broadcast, func() {
				printer.Success("Cancelled broadcast %s.", args[0])
			})
		},
	}
}

func newBroadcastsRecipientsCmd(state *State) *cobra.Command {
	var status string
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "recipients <broadcast-id>",
		Short: "List who a broadcast reached, and who it skipped",
		Long: "List who a broadcast reached, and who it skipped. A skipped row says why: " +
			"window_closed means the network's messaging window had shut and nothing was " +
			"attempted.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			list, err := client.Broadcasts.Recipients(cmd.Context(), args[0], &fopost.ListRecipientsParams{
				Status:  status,
				Page:    page,
				PerPage: perPage,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(list, func() {
				rows := make([][]string, 0, len(list.Data))
				for _, recipient := range list.Data {
					rows = append(rows, []string{
						recipient.ContactID,
						output.Truncate(output.Dash(recipient.DisplayName), 24),
						recipient.Status,
						output.Dash(recipient.SkipReason),
						recipient.SentAt.String(),
					})
				}
				printer.Table([]string{"contact", "name", "status", "reason", "sent"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "pending, sent, skipped, or failed")
	cmd.Flags().IntVar(&page, "page", 0, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "results per page, up to 200")
	return cmd
}

func newBroadcastsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <broadcast-id>",
		Short: "Delete a broadcast; messages already sent stay in their conversations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf("Delete broadcast %s? This cannot be undone.", args[0])
				if err := confirm(state, prompt); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Broadcasts.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted broadcast %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newSequencesCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sequences",
		Aliases: []string{"sequence"},
		Short:   "Walk contacts through a series of messages on a delay",
		Long: "Walk contacts through a series of messages on a delay.\n\n" +
			"The messaging window applies to every step: one that comes due outside it is " +
			"skipped rather than sent, and the enrollment carries on. So someone can " +
			"complete a sequence having received only some of its messages.",
	}
	cmd.AddCommand(
		newSequencesListCmd(state),
		newSequencesGetCmd(state),
		newSequencesCreateCmd(state),
		newSequencesEnrollCmd(state),
		newSequencesUnenrollCmd(state),
		newSequencesEnrollmentsCmd(state),
		newSequencesPauseCmd(state, true),
		newSequencesPauseCmd(state, false),
		newSequencesDeleteCmd(state),
	)
	return cmd
}

func newSequencesListCmd(state *State) *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List drip sequences",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			list, err := client.Sequences.List(cmd.Context(), &fopost.ListSequencesParams{
				WorkspaceID: workspaceID,
				Page:        page,
				PerPage:     perPage,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(list, func() {
				rows := make([][]string, 0, len(list.Data))
				for _, sequence := range list.Data {
					rows = append(rows, []string{
						sequence.ID,
						output.Truncate(sequence.Name, 28),
						sequence.Status,
						strconv.Itoa(len(sequence.Steps)),
						strconv.Itoa(sequence.Enrollments.Active),
						strconv.Itoa(sequence.Enrollments.Completed),
					})
				}
				printer.Table(
					[]string{"id", "name", "status", "steps", "active", "completed"},
					rows,
				)
			})
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "results per page, up to 100")
	return cmd
}

func newSequencesGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <sequence-id>",
		Short: "Show one sequence and the steps it walks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			sequence, err := client.Sequences.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(sequence, func() {
				rows := make([][]string, 0, len(sequence.Steps))
				for i, step := range sequence.Steps {
					rows = append(rows, []string{
						strconv.Itoa(i + 1),
						fmt.Sprintf("+%gh", step.DelayHours),
						output.Truncate(step.Text, 56),
					})
				}
				printer.Table([]string{"step", "after", "message"}, rows)
			})
		},
	}
}

func newSequencesCreateCmd(state *State) *cobra.Command {
	var accountID, name string
	var steps []string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Write a sequence; creating one enrolls nobody",
		Long: "Write a sequence; creating one enrolls nobody. Each --step is " +
			"`<hours>:<message>`, counted from the step before it, so 0 on the first " +
			"means straight away.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			parsed, err := parseSteps(steps)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			sequence, err := client.Sequences.Create(cmd.Context(), &fopost.CreateSequenceRequest{
				WorkspaceID: workspaceID,
				AccountID:   accountID,
				Name:        name,
				Steps:       parsed,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(sequence, func() {
				printer.Success("Wrote sequence %s with %d steps. Nobody is enrolled yet.",
					sequence.ID, len(sequence.Steps))
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "the connected account every step is sent from")
	cmd.Flags().StringVar(&name, "name", "", "what you call it; never sent to anyone")
	cmd.Flags().StringArrayVar(&steps, "step", nil, "a step as <hours>:<message>, repeatable")
	_ = cmd.MarkFlagRequired("account")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("step")
	return cmd
}

// "0:Thanks for the follow" — the hours are counted from the step before.
func parseSteps(raw []string) ([]fopost.SequenceStep, error) {
	steps := make([]fopost.SequenceStep, 0, len(raw))
	for _, entry := range raw {
		hours, text, found := strings.Cut(entry, ":")
		if !found {
			return nil, fmt.Errorf("step %q must be <hours>:<message>", entry)
		}
		delay, err := strconv.ParseFloat(strings.TrimSpace(hours), 64)
		if err != nil {
			return nil, fmt.Errorf("step %q: %q is not a number of hours", entry, hours)
		}
		steps = append(steps, fopost.SequenceStep{DelayHours: delay, Text: text})
	}
	return steps, nil
}

func newSequencesEnrollCmd(state *State) *cobra.Command {
	var contactIDs, platforms []string
	cmd := &cobra.Command{
		Use:   "enroll <sequence-id>",
		Short: "Put contacts on a sequence",
		Long: "Put contacts on a sequence, by id or by audience. Re-enrolling someone " +
			"restarts their walk from the first step rather than running two in parallel.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(contactIDs) == 0 && len(platforms) == 0 {
				return fmt.Errorf("name --contact ids or a --platform audience")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			body := &fopost.EnrollRequest{ContactIDs: contactIDs}
			if len(platforms) > 0 {
				body.Audience = &fopost.AudienceFilter{Platforms: platforms}
			}
			enrolled, err := client.Sequences.Enroll(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(enrolled, func() {
				printer.Success("Enrolled %d contacts.", enrolled.Enrolled)
			})
		},
	}
	cmd.Flags().StringSliceVar(&contactIDs, "contact", nil, "a contact id, repeatable")
	cmd.Flags().StringSliceVar(&platforms, "platform", nil, "enroll every contact with a handle here")
	return cmd
}

func newSequencesUnenrollCmd(state *State) *cobra.Command {
	var contactIDs []string
	cmd := &cobra.Command{
		Use:   "unenroll <sequence-id>",
		Short: "Take contacts off a sequence; nothing further fires for them",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			stopped, err := client.Sequences.Unenroll(cmd.Context(), args[0], contactIDs)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(stopped, func() {
				printer.Success("Stopped %d enrollments.", stopped.Stopped)
			})
		},
	}
	cmd.Flags().StringSliceVar(&contactIDs, "contact", nil, "a contact id, repeatable")
	_ = cmd.MarkFlagRequired("contact")
	return cmd
}

func newSequencesEnrollmentsCmd(state *State) *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "enrollments <sequence-id>",
		Short: "List who is on a sequence and where they stand",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			list, err := client.Sequences.Enrollments(cmd.Context(), args[0], &fopost.ListEnrollmentsParams{
				Page:    page,
				PerPage: perPage,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(list, func() {
				rows := make([][]string, 0, len(list.Data))
				for _, enrollment := range list.Data {
					rows = append(rows, []string{
						enrollment.ContactID,
						output.Truncate(output.Dash(enrollment.DisplayName), 24),
						strconv.Itoa(enrollment.Step),
						enrollment.Status,
						enrollment.NextAt.String(),
					})
				}
				printer.Table([]string{"contact", "name", "step", "status", "next"}, rows)
			})
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "results per page, up to 200")
	return cmd
}

func newSequencesPauseCmd(state *State, pause bool) *cobra.Command {
	use, short := "resume <sequence-id>", "Resume a sequence; enrollments pick up where they stood"
	status := fopost.SequenceActive
	if pause {
		use = "pause <sequence-id>"
		short = "Pause a sequence; nothing fires without ending any enrollment"
		status = fopost.SequencePaused
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			sequence, err := client.Sequences.Update(cmd.Context(), args[0], &fopost.UpdateSequenceRequest{
				Status: &status,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(sequence, func() {
				printer.Success("Sequence %s is now %s.", args[0], sequence.Status)
			})
		},
	}
}

func newSequencesDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <sequence-id>",
		Short: "Delete a sequence and every enrollment on it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf(
					"Delete sequence %s and every enrollment on it? This cannot be undone.", args[0])
				if err := confirm(state, prompt); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Sequences.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted sequence %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
