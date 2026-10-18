package cmd

import (
	"fmt"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newQueueCmd) }

func newQueueCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage the posting queue and its weekly slots",
		Long: "The posting queue is a set of weekly time slots. A post created with\n" +
			"--schedule queue lands in the next free one.",
	}
	cmd.AddCommand(
		newQueueSlotsCmd(state),
		newQueueNextCmd(state),
		newQueuePreviewCmd(state),
	)
	return cmd
}

func newQueueSlotsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "slots",
		Aliases: []string{"slot"},
		Short:   "List, add, change, and remove queue slots",
	}
	cmd.AddCommand(
		newQueueSlotsListCmd(state),
		newQueueSlotsCreateCmd(state),
		newQueueSlotsUpdateCmd(state),
		newQueueSlotsDeleteCmd(state),
	)
	return cmd
}

var weekdayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func weekdayLabel(weekday int) string {
	if weekday < 0 || weekday >= len(weekdayNames) {
		return itoa(weekday)
	}
	return weekdayNames[weekday]
}

func accountLabel(id *string) string {
	if id == nil {
		return "all"
	}
	return *id
}

func newQueueSlotsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the workspace's queue slots",
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
			slots, err := client.Queue.ListSlots(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(slots, func() {
				rows := make([][]string, 0, len(slots))
				for _, slot := range slots {
					rows = append(rows, []string{
						slot.ID,
						weekdayLabel(slot.Weekday),
						slot.Time,
						slot.Timezone,
						accountLabel(slot.AccountID),
					})
				}
				printer.Table([]string{"id", "weekday", "time", "timezone", "account"}, rows)
			})
		},
	}
}

func newQueueSlotsCreateCmd(state *State) *cobra.Command {
	var (
		weekday  int
		at       string
		timezone string
		account  string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Add a weekly queue slot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("weekday") {
				return usageErrorf("--weekday is required")
			}
			if at == "" || timezone == "" {
				return usageErrorf("--time and --timezone are required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			body := &fopost.CreateQueueSlotRequest{
				WorkspaceID: workspaceID,
				Weekday:     weekday,
				Time:        at,
				Timezone:    timezone,
			}
			if account != "" {
				body.AccountID = fopost.String(account)
			}
			slot, err := client.Queue.CreateSlot(cmd.Context(), body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(slot, func() {
				printer.Success("Created slot %s: %s %s %s.", slot.ID, weekdayLabel(slot.Weekday), slot.Time, slot.Timezone)
			})
		},
	}
	cmd.Flags().IntVar(&weekday, "weekday", 0, "0 (Sunday) through 6 (Saturday) (required)")
	cmd.Flags().StringVar(&at, "time", "", "HH:MM, 24-hour (required)")
	cmd.Flags().StringVar(&timezone, "timezone", "", "IANA zone name, such as Europe/Zagreb (required)")
	cmd.Flags().StringVar(&account, "account", "", "account id the slot applies to; omit for every account")
	return cmd
}

func printMoved(printer *output.Printer, moved []fopost.QueueMove) {
	if len(moved) == 0 {
		return
	}
	rows := make([][]string, 0, len(moved))
	for _, move := range moved {
		rows = append(rows, []string{
			move.ID,
			output.Stamp(move.From.Time, move.From.Raw),
			output.Stamp(move.To.Time, move.To.Raw),
		})
	}
	printer.Table([]string{"post id", "from", "to"}, rows)
}

func newQueueSlotsUpdateCmd(state *State) *cobra.Command {
	var (
		weekday  int
		at       string
		timezone string
		account  string
	)
	cmd := &cobra.Command{
		Use:   "update <slot-id>",
		Short: "Change a queue slot; queued posts on it move with it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := &fopost.UpdateQueueSlotRequest{Time: at, Timezone: timezone}
			if cmd.Flags().Changed("weekday") {
				body.Weekday = fopost.Int(weekday)
			}
			if cmd.Flags().Changed("account") {
				if account == "" {
					body.AccountID = nil
				} else {
					body.AccountID = fopost.String(account)
				}
			}
			if body.Weekday == nil && at == "" && timezone == "" && !cmd.Flags().Changed("account") {
				return usageErrorf("pass at least one of --weekday, --time, --timezone, or --account")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			slot, err := client.Queue.UpdateSlot(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(slot, func() {
				printer.Success("Updated slot %s: %s %s %s. Moved %d post(s).", slot.ID, weekdayLabel(slot.Weekday), slot.Time, slot.Timezone, len(slot.Moved))
				printMoved(printer, slot.Moved)
			})
		},
	}
	cmd.Flags().IntVar(&weekday, "weekday", 0, "0 (Sunday) through 6 (Saturday)")
	cmd.Flags().StringVar(&at, "time", "", "HH:MM, 24-hour")
	cmd.Flags().StringVar(&timezone, "timezone", "", "IANA zone name")
	cmd.Flags().StringVar(&account, "account", "", "account id the slot applies to")
	return cmd
}

func newQueueSlotsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <slot-id>",
		Short: "Remove a queue slot; queued posts on it move to the next free one",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete slot %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Queue.DeleteSlot(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Success("Deleted slot %s. Moved %d post(s).", args[0], len(result.Moved))
				printMoved(printer, result.Moved)
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

type queueNext struct {
	Next *fopost.Time `json:"next"`
}

func newQueueNextCmd(state *State) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show when a post added to the queue now would go out",
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
			next, err := client.Queue.Next(cmd.Context(), workspaceID, account)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(queueNext{Next: next}, func() {
				if next == nil {
					printer.Line("No slot applies.")
					return
				}
				printer.Line("%s", output.Stamp(next.Time, next.Raw))
			})
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "narrow to the slots for one account id")
	return cmd
}

func newQueuePreviewCmd(state *State) *cobra.Command {
	var (
		account string
		count   int
	)
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Show the next free queue times",
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
			times, err := client.Queue.Preview(cmd.Context(), workspaceID, account, count)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(times, func() {
				rows := make([][]string, 0, len(times))
				for index, at := range times {
					rows = append(rows, []string{itoa(index + 1), output.Stamp(at.Time, at.Raw)})
				}
				printer.Table([]string{"#", "time"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "narrow to the slots for one account id")
	cmd.Flags().IntVar(&count, "count", 0, "how many times to show (the API defaults to 5, at most 20)")
	return cmd
}
