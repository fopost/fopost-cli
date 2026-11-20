package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newVideoEditsCmd) }

func newVideoEditsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "video-edits",
		Aliases: []string{"video-edit"},
		Short:   "Run edit jobs on a media library video",
		Long: "A video edit job runs its operations in order on one media library video\n" +
			"(trim, cut, subtitles, clips, reframe and the rest) in the background.\n" +
			"Pass --wait to block until it finishes.",
	}
	cmd.AddCommand(newVideoEditsOpsCmd(state), newVideoEditsCreateCmd(state), newVideoEditsGetCmd(state))
	return cmd
}

func newVideoEditsOpsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "ops",
		Short: "List the operations a job may use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			ops, err := client.VideoEdits.Ops(cmd.Context())
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(ops, func() {
				rows := make([][]string, 0, len(ops))
				for _, op := range ops {
					rows = append(rows, []string{op.Name, yesNo(op.Available), yesNo(op.UsesAI)})
				}
				printer.Table([]string{"op", "available", "uses ai"}, rows)
			})
		},
	}
}

func newVideoEditsCreateCmd(state *State) *cobra.Command {
	var (
		mediaID        string
		ops            []string
		operationsFile string
		wait           waitFlags
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Queue an edit job on a video",
		Long: "Each --op is an operation name, optionally followed by its params as a JSON\n" +
			"object: --op 'trim={\"start\":2,\"end\":12}'. For a longer job, pass\n" +
			"--operations-file with a JSON array of {\"op\": …, \"params\": {…}} (- for stdin).\n" +
			"The result is a new media library item; the source video is left untouched.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if mediaID == "" {
				return usageErrorf("--media is required")
			}
			operations, err := videoEditOperations(state, ops, operationsFile)
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
			job, err := client.VideoEdits.Create(cmd.Context(), &fopost.CreateVideoEditRequest{
				WorkspaceID: workspaceID,
				MediaID:     mediaID,
				Operations:  operations,
			})
			if err != nil {
				return err
			}
			if wait.enabled {
				if job, err = waitForVideoEdit(cmd.Context(), client, job, workspaceID, wait.interval); err != nil {
					return err
				}
			}
			return printVideoEditResult(state, job, wait.enabled)
		},
	}
	cmd.Flags().StringVar(&mediaID, "media", "", "media library id of the source video (required)")
	cmd.Flags().StringArrayVar(&ops, "op", nil, "operation, as name or name={json params} (repeatable, run in order)")
	cmd.Flags().StringVar(&operationsFile, "operations-file", "", "JSON array of operations (- for stdin)")
	wait.register(cmd)
	return cmd
}

func newVideoEditsGetCmd(state *State) *cobra.Command {
	var wait waitFlags
	cmd := &cobra.Command{
		Use:   "get <job-id>",
		Short: "Show an edit job's status and result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			job, err := client.VideoEdits.Get(cmd.Context(), args[0], resolved.Workspace)
			if err != nil {
				return err
			}
			if wait.enabled {
				if job, err = waitForVideoEdit(cmd.Context(), client, job, resolved.Workspace, wait.interval); err != nil {
					return err
				}
			}
			return printVideoEditResult(state, job, wait.enabled)
		},
	}
	wait.register(cmd)
	return cmd
}

type waitFlags struct {
	enabled  bool
	interval time.Duration
}

func (w *waitFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&w.enabled, "wait", false, "block until the job succeeds or fails")
	cmd.Flags().DurationVar(&w.interval, "poll-interval", 3*time.Second, "how often --wait checks the job")
}

// videoEditOperations reads the job's steps from --op and --operations-file.
func videoEditOperations(state *State, ops []string, file string) ([]fopost.VideoEditOperation, error) {
	if len(ops) > 0 && file != "" {
		return nil, usageErrorf("--op and --operations-file are alternatives; pass one")
	}
	if file != "" {
		var operations []fopost.VideoEditOperation
		if err := readJSONFile(state, file, &operations); err != nil {
			return nil, err
		}
		if len(operations) == 0 {
			return nil, usageErrorf("%s holds no operations", file)
		}
		return operations, nil
	}
	if len(ops) == 0 {
		return nil, usageErrorf("at least one --op is required")
	}
	operations := make([]fopost.VideoEditOperation, 0, len(ops))
	for _, value := range ops {
		name, params, hasParams := strings.Cut(value, "=")
		operation := fopost.VideoEditOperation{Op: strings.TrimSpace(name)}
		if operation.Op == "" {
			return nil, usageErrorf("--op %q has no operation name", value)
		}
		if hasParams {
			if err := json.Unmarshal([]byte(params), &operation.Params); err != nil {
				return nil, usageErrorf("--op %s: params must be a JSON object: %v", operation.Op, err)
			}
		}
		operations = append(operations, operation)
	}
	return operations, nil
}

// waitForVideoEdit polls until the job succeeds or fails.
func waitForVideoEdit(ctx context.Context, client *fopost.Client, job *fopost.VideoEdit, workspaceID string, interval time.Duration) (*fopost.VideoEdit, error) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for !videoEditDone(job) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		next, err := client.VideoEdits.Get(ctx, job.ID, workspaceID)
		if err != nil {
			return nil, err
		}
		job = next
	}
	return job, nil
}

func videoEditDone(job *fopost.VideoEdit) bool {
	return job.Status == fopost.VideoEditSucceeded || job.Status == fopost.VideoEditFailed
}

func printVideoEditResult(state *State, job *fopost.VideoEdit, waited bool) error {
	if waited && job.Status == fopost.VideoEditFailed {
		return fmt.Errorf("video edit %s failed: %s", job.ID, output.Dash(strValue(job.Error)))
	}
	printer := state.Printer()
	return printer.Value(job, func() { printVideoEdit(printer, job) })
}

func printVideoEdit(printer *output.Printer, job *fopost.VideoEdit) {
	steps := make([]string, 0, len(job.Operations))
	for _, operation := range job.Operations {
		steps = append(steps, operation.Op)
	}
	result := strValue(job.ResultMediaID)
	if job.ResultMedia != nil && job.ResultMedia.PreviewURL != nil {
		result += " " + *job.ResultMedia.PreviewURL
	}
	completed := "-"
	if job.CompletedAt != nil {
		completed = output.Stamp(job.CompletedAt.Time, job.CompletedAt.Raw)
	}
	printer.Fields([][2]string{
		{"ID", job.ID},
		{"Status", job.Status},
		{"Progress", fmt.Sprintf("%.0f%%", job.Progress*100)},
		{"Source", job.SourceMediaID},
		{"Operations", output.Dash(strings.Join(steps, ", "))},
		{"Result", output.Dash(strings.TrimSpace(result))},
		{"Error", output.Dash(strValue(job.Error))},
		{"Created", output.Stamp(job.CreatedAt.Time, job.CreatedAt.Raw)},
		{"Completed", completed},
	})
	if len(job.ExtraOutputs) == 0 {
		return
	}
	rows := make([][]string, 0, len(job.ExtraOutputs))
	for _, extra := range job.ExtraOutputs {
		rows = append(rows, []string{extra.MediaID, output.Dash(extra.Label), extra.Mime, output.Dash(strValue(extra.PreviewURL))})
	}
	printer.Line("")
	printer.Table([]string{"media id", "label", "type", "preview"}, rows)
}
