package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newAutomationsCmd) }

func newAutomationsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "automations",
		Aliases: []string{"automation"},
		Short:   "Inspect, toggle, and trigger automations",
	}
	cmd.AddCommand(
		newAutomationsListCmd(state),
		newAutomationsGetCmd(state),
		newAutomationsToggleCmd(state),
		newAutomationsTriggerCmd(state),
		newAutomationsRunsCmd(state),
	)
	return cmd
}

func newAutomationsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List automations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			automations, err := client.Automations.List(cmd.Context())
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(automations, func() {
				rows := make([][]string, 0, len(automations))
				for _, automation := range automations {
					rows = append(rows, []string{
						automation.ID,
						output.Truncate(automation.Name, 30),
						automation.TriggerType,
						boolLabel(automation.Active, "on", "off"),
						itoa(automation.RunCount),
						output.Stamp(automation.LastTriggeredAt.Time, automation.LastTriggeredAt.Raw),
					})
				}
				printer.Table([]string{"id", "name", "trigger", "state", "runs", "last run"}, rows)
			})
		},
	}
}

func newAutomationsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <automation-id>",
		Short: "Show one automation with its steps",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			automation, err := client.Automations.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(automation, func() {
				printer.Fields([][2]string{
					{"ID", automation.ID},
					{"Name", automation.Name},
					{"Workspace", automation.WorkspaceID},
					{"Trigger", automation.TriggerType},
					{"State", boolLabel(automation.Active, "on", "off")},
					{"Runs", itoa(automation.RunCount)},
					{"Last run", output.Stamp(automation.LastTriggeredAt.Time, automation.LastTriggeredAt.Raw)},
				})
				if len(automation.Steps) == 0 {
					return
				}
				printer.Line("")
				rows := make([][]string, 0, len(automation.Steps))
				for _, step := range automation.Steps {
					rows = append(rows, []string{itoa(step.Position), step.ActionType})
				}
				printer.Table([]string{"position", "action"}, rows)
			})
		},
	}
}

func newAutomationsToggleCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "toggle <automation-id>",
		Short: "Switch an automation on or off",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Automations.Toggle(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Success("Automation %s is now %s.", result.ID, boolLabel(result.Active, "on", "off"))
			})
		},
	}
}

func newAutomationsTriggerCmd(state *State) *cobra.Command {
	var payload string
	cmd := &cobra.Command{
		Use:   "trigger <automation-id>",
		Short: "Fire an api_webhook automation with a payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readPayload(payload, state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Automations.Trigger(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Success("Started run %d.", result.RunID)
			})
		},
	}
	cmd.Flags().StringVar(&payload, "payload", "", "JSON object the steps can read; @file to read a file, - for stdin")
	return cmd
}

// readPayload accepts inline JSON, @path, or "-" for standard input.
func readPayload(value string, state *State) (map[string]any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[string]any{}, nil
	}
	raw := []byte(value)
	switch {
	case value == "-":
		read, err := readAll(state)
		if err != nil {
			return nil, err
		}
		raw = read
	case strings.HasPrefix(value, "@"):
		read, err := os.ReadFile(value[1:])
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", value[1:], err)
		}
		raw = read
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, usageErrorf("--payload must be a JSON object: %v", err)
	}
	return payload, nil
}

func newAutomationsRunsCmd(state *State) *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "runs <automation-id>",
		Short: "List an automation's executions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			runs, err := client.Automations.Runs(cmd.Context(), args[0], page, perPage)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(runs, func() {
				rows := make([][]string, 0, len(runs.Data))
				for _, run := range runs.Data {
					rows = append(rows, []string{
						itoa(run.ID),
						run.Status,
						output.Stamp(run.StartedAt.Time, run.StartedAt.Raw),
						output.Stamp(run.CompletedAt.Time, run.CompletedAt.Raw),
						output.Truncate(run.ErrorMessage, 40),
					})
				}
				printer.Table([]string{"run", "status", "started", "completed", "error"}, rows)
			})
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "runs per page")
	return cmd
}
