package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newFontsCmd) }

func newFontsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fonts",
		Aliases: []string{"font"},
		Short:   "List the font library and manage uploaded fonts",
	}
	cmd.AddCommand(newFontsListCmd(state), newFontsUploadCmd(state), newFontsDeleteCmd(state))
	return cmd
}

func newFontsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the font library and the workspace's uploaded fonts",
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
			fonts, err := client.Fonts.List(cmd.Context(), resolved.Workspace)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(fonts, func() {
				uploaded := make([][]string, 0, len(fonts.Uploaded))
				for _, font := range fonts.Uploaded {
					uploaded = append(uploaded, []string{
						font.ID,
						output.Truncate(font.Family, 32),
						itoa(font.Weight),
						font.Style,
						font.Format,
					})
				}
				if len(uploaded) > 0 {
					printer.Table([]string{"id", "family", "weight", "style", "format"}, uploaded)
					printer.Line("")
				}
				library := make([][]string, 0, len(fonts.Library))
				for _, font := range fonts.Library {
					weights := make([]string, 0, len(font.Weights))
					for _, weight := range font.Weights {
						weights = append(weights, itoa(weight))
					}
					library = append(library, []string{
						font.Slug,
						output.Truncate(font.Family, 32),
						font.Category,
						output.Dash(strings.Join(weights, " ")),
					})
				}
				printer.Table([]string{"slug", "family", "category", "weights"}, library)
			})
		},
	}
}

func newFontsUploadCmd(state *State) *cobra.Command {
	var (
		family string
		weight int
		style  string
	)
	cmd := &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload a font file (up to 5 MB) to the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			handle, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("opening %s: %w", args[0], err)
			}
			defer handle.Close()
			font, err := client.Fonts.Upload(cmd.Context(), &fopost.UploadFontRequest{
				WorkspaceID: workspaceID,
				File:        fopost.File{Name: filepath.Base(args[0]), Content: handle},
				Family:      family,
				Weight:      weight,
				Style:       style,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(font, func() {
				printer.Success("Uploaded font %s.", font.ID)
				printer.Fields([][2]string{
					{"Family", font.Family},
					{"Weight", itoa(font.Weight)},
					{"Style", font.Style},
					{"Format", font.Format},
				})
			})
		},
	}
	cmd.Flags().StringVar(&family, "family", "", "family name (defaults to the file name)")
	cmd.Flags().IntVar(&weight, "weight", 0, "weight from 100 to 1000 (defaults to 400)")
	cmd.Flags().StringVar(&style, "style", "", "normal or italic (defaults to normal)")
	return cmd
}

func newFontsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <font-id>",
		Short: "Remove an uploaded font",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete font %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Fonts.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted font %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
