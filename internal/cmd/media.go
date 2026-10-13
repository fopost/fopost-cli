package cmd

import (
	"fmt"
	"os"
	"strings"

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
	cmd.AddCommand(newMediaListCmd(state), newMediaLimitsCmd(state), newMediaUploadCmd(state), newMediaDeleteCmd(state))
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

func newMediaLimitsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "limits",
		Short: "Show the largest file the account may upload",
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
			limits, err := client.Media.Limits(cmd.Context(), resolved.Workspace)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(limits, func() {
				nextTier := "-"
				if limits.NextTier != nil {
					nextTier = humanSize(limits.NextTier.MaxFileBytes)
					if limits.NextTier.UpToAccounts != nil {
						nextTier += fmt.Sprintf(" up to %d accounts", *limits.NextTier.UpToAccounts)
					}
				}
				printer.Fields([][2]string{
					{"Max File", humanSize(limits.MaxFileBytes)},
					{"Source", limits.Source},
					{"Connected Accounts", itoa(limits.ConnectedAccounts)},
					{"Large Uploads", yesNo(limits.LargeUploadsEnabled)},
					{"Next Tier", nextTier},
					{"Multipart Max File", humanSize(limits.MultipartMaxFileBytes)},
					{"Max Files Per Upload", itoa(limits.MaxFilesPerUpload)},
					{"Direct Upload", yesNo(limits.DirectUploadAvailable)},
					{"Allowed Types", output.Dash(strings.Join(limits.AllowedTypes, ", "))},
				})
			})
		},
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
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
			small, large := args, []string(nil)
			if direct {
				small, large = nil, args
			} else {
				small, large, err = splitByCeiling(cmd, client, workspaceID, args)
				if err != nil {
					return err
				}
			}
			var uploaded []fopost.UploadedMedia
			if len(small) > 0 {
				uploaded, err = uploadFiles(cmd, client, workspaceID, small)
				if err != nil {
					return err
				}
			}
			if len(large) > 0 {
				streamed, err := uploadFilesDirect(cmd, client, workspaceID, large)
				if err != nil {
					return err
				}
				uploaded = append(uploaded, streamed...)
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
	cmd.Flags().BoolVar(&direct, "direct", false, "send every file straight to storage through a presigned URL, whatever its size")
	return cmd
}

// splitByCeiling sorts paths into those a multipart request can carry and
// those that must go direct, reading the ceiling from the API once.
func splitByCeiling(cmd *cobra.Command, client *fopost.Client, workspaceID string, paths []string) (small, large []string, err error) {
	ceiling := fopost.DefaultMultipartMaxFileBytes
	if limits, err := client.Media.Limits(cmd.Context(), workspaceID); err == nil && limits.MultipartMaxFileBytes > 0 {
		ceiling = limits.MultipartMaxFileBytes
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s: %w", path, err)
		}
		if info.Size() > ceiling {
			large = append(large, path)
		} else {
			small = append(small, path)
		}
	}
	return small, large, nil
}

// uploadFilesDirect presigns, streams, and completes one file at a time.
func uploadFilesDirect(cmd *cobra.Command, client *fopost.Client, workspaceID string, paths []string) ([]fopost.UploadedMedia, error) {
	uploaded := make([]fopost.UploadedMedia, 0, len(paths))
	for _, path := range paths {
		file, err := fopost.OpenPostFile(path)
		if err != nil {
			return nil, err
		}
		asset, err := client.Media.UploadDirect(cmd.Context(), &fopost.PresignInput{
			WorkspaceID: workspaceID,
			Filename:    file.Filename,
			MimeType:    file.MIMEType,
			Size:        file.Size,
		}, file.Reader)
		file.Close()
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
