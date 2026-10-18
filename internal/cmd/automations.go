package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newAutomationsCmd) }

func newAutomationsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "automations",
		Aliases: []string{"automation"},
		Short:   "Inspect, toggle, trigger, and restore automations",
	}
	cmd.AddCommand(
		newAutomationsListCmd(state),
		newAutomationsGetCmd(state),
		newAutomationsToggleCmd(state),
		newAutomationsTriggerCmd(state),
		newAutomationsRunsCmd(state),
		newAutomationsVersionsCmd(state),
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

func newAutomationsVersionsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "versions",
		Aliases: []string{"version"},
		Short:   "Walk and restore an automation's saved graphs",
	}
	cmd.AddCommand(
		newAutomationsVersionsListCmd(state),
		newAutomationsVersionsGetCmd(state),
		newAutomationsVersionsRestoreCmd(state),
	)
	return cmd
}

func versionAuthor(author *fopost.AutomationVersionAuthor) string {
	if author == nil {
		return ""
	}
	return author.Name
}

func parseVersion(value string) (int, error) {
	version, err := strconv.Atoi(value)
	if err != nil || version < 1 {
		return 0, usageErrorf("version must be a positive number, got %q", value)
	}
	return version, nil
}

func newAutomationsVersionsListCmd(state *State) *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list <automation-id>",
		Short: "List an automation's saved versions, newest first",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			versions, err := client.Automations.ListVersions(cmd.Context(), args[0], page, perPage)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(versions, func() {
				rows := make([][]string, 0, len(versions.Data))
				for _, version := range versions.Data {
					rows = append(rows, []string{
						itoa(version.Version),
						itoa(version.NodeCount),
						"+" + itoa(len(version.NodesAdded)) + " -" + itoa(len(version.NodesRemoved)),
						output.Dash(versionAuthor(version.CreatedBy)),
						output.Stamp(version.CreatedAt.Time, version.CreatedAt.Raw),
						output.Truncate(strValue(version.Note), 40),
					})
				}
				printer.Table([]string{"version", "nodes", "changed", "by", "saved", "note"}, rows)
				if versions.Meta.Total > 0 {
					printer.Line("")
					printer.Line("Page %d of %d · %d version(s) total", max(versions.Meta.Page, 1), max(versions.Meta.TotalPages, 1), versions.Meta.Total)
				}
			})
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "versions per page")
	return cmd
}

func printGraph(printer *output.Printer, graph fopost.WorkflowGraph) {
	rows := make([][]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		rows = append(rows, []string{itoa(node.Position), node.ID, node.Kind, output.Dash(strValue(node.Label))})
	}
	printer.Table([]string{"position", "node id", "kind", "label"}, rows)
}

func newAutomationsVersionsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <automation-id> <version>",
		Short: "Show one saved version with its nodes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := parseVersion(args[1])
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			version, err := client.Automations.GetVersion(cmd.Context(), args[0], number)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(version, func() {
				printer.Fields([][2]string{
					{"Version", itoa(version.Version)},
					{"Saved", output.Stamp(version.CreatedAt.Time, version.CreatedAt.Raw)},
					{"By", output.Dash(versionAuthor(version.CreatedBy))},
					{"Note", output.Dash(strValue(version.Note))},
					{"Edges", itoa(len(version.Graph.Edges))},
				})
				if len(version.Graph.Nodes) == 0 {
					return
				}
				printer.Line("")
				printGraph(printer, version.Graph)
			})
		},
	}
}

func newAutomationsVersionsRestoreCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "restore <automation-id> <version>",
		Short: "Save a version's graph as a new version on top of the history",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := parseVersion(args[1])
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			restored, err := client.Automations.RestoreVersion(cmd.Context(), args[0], number)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(restored, func() {
				printer.Success("Restored version %d as version %d with %d step(s).", number, restored.Version, len(restored.Steps))
			})
		},
	}
}
