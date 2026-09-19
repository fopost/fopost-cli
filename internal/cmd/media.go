package cmd

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newMediaCmd) }

func newMediaCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "media",
		Short: "List, upload, and remove media library assets",
	}
	cmd.AddCommand(newMediaListCmd(state), newMediaUploadCmd(state), newMediaDeleteCmd(state))
	return cmd
}

func newMediaListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the workspace's media library",
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
			items, err := client.Media.List(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(items, func() {
				rows := make([][]string, 0, len(items))
				for _, item := range items {
					rows = append(rows, []string{
						item.ID,
						item.Type,
						output.Truncate(item.Name, 36),
						humanSize(item.Size),
						output.Stamp(item.CreatedAt.Time, item.CreatedAt.Raw),
					})
				}
				printer.Table([]string{"id", "type", "name", "size", "created"}, rows)
			})
		},
	}
}

func newMediaUploadCmd(state *State) *cobra.Command {
	var direct bool
	cmd := &cobra.Command{
		Use:   "upload <file> [file...]",
		Short: "Upload local files to the media library",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			var uploaded []fopost.UploadedMedia
			if direct {
				uploaded, err = uploadFilesDirect(cmd, client, workspaceID, args)
			} else {
				uploaded, err = uploadFiles(cmd, client, workspaceID, args)
			}
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(uploaded, func() {
				rows := make([][]string, 0, len(uploaded))
				for _, asset := range uploaded {
					rows = append(rows, []string{asset.ID, asset.Type, output.Truncate(asset.Name, 36), humanSize(asset.Size)})
				}
				printer.Table([]string{"id", "type", "name", "size"}, rows)
			})
		},
	}
	cmd.Flags().BoolVar(&direct, "direct", false, "upload each file straight to storage through a presigned URL")
	return cmd
}

// uploadFilesDirect presigns, PUTs, and completes one file at a time.
func uploadFilesDirect(cmd *cobra.Command, client *fopost.Client, workspaceID string, paths []string) ([]fopost.UploadedMedia, error) {
	uploaded := make([]fopost.UploadedMedia, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		mimeType := mime.TypeByExtension(filepath.Ext(path))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		asset, err := client.Media.UploadDirect(cmd.Context(), workspaceID, filepath.Base(path), mimeType, data)
		if err != nil {
			return nil, err
		}
		uploaded = append(uploaded, *asset)
	}
	return uploaded, nil
}

func newMediaDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <media-id>",
		Short: "Remove an asset from the media library",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete media %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Media.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted media %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value, exp := float64(bytes)/unit, 0
	for value >= unit && exp < 3 {
		value /= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", value, "KMGT"[exp])
}
