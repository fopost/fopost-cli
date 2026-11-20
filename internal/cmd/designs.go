package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newDesignsCmd) }

func newDesignsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "designs",
		Aliases: []string{"design"},
		Short:   "List, create, export, and remove designs",
	}
	cmd.AddCommand(
		newDesignsListCmd(state),
		newDesignsGetCmd(state),
		newDesignsCreateCmd(state),
		newDesignsDuplicateCmd(state),
		newDesignsExportCmd(state),
		newDesignsDeleteCmd(state),
	)
	return cmd
}

func newDesignsListCmd(state *State) *cobra.Command {
	var (
		kind   string
		limit  int
		offset int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the workspace's designs, newest first",
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
			designs, err := client.Designs.List(cmd.Context(), &fopost.ListDesignsParams{
				WorkspaceID: workspaceID,
				Kind:        kind,
				Limit:       limit,
				Offset:      offset,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(designs, func() {
				rows := make([][]string, 0, len(designs))
				for _, design := range designs {
					rows = append(rows, []string{
						design.ID,
						design.Kind,
						output.Truncate(design.Name, 36),
						fmt.Sprintf("%dx%d", design.Width, design.Height),
						output.Stamp(design.UpdatedAt.Time, design.UpdatedAt.Raw),
					})
				}
				printer.Table([]string{"id", "kind", "name", "size", "updated"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "only this kind: story, post, carousel, thumbnail, card or collage")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of designs to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "number of designs to skip")
	return cmd
}

func newDesignsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <design-id>",
		Short: "Show one design; --json includes its document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			design, err := client.Designs.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(design, func() { printDesign(printer, design) })
		},
	}
}

func newDesignsCreateCmd(state *State) *cobra.Command {
	var (
		kind         string
		name         string
		documentFile string
		brandVoiceID string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Save a new design, blank or from a document file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if kind == "" {
				return usageErrorf("--kind is required")
			}
			var document fopost.DesignDocument
			if documentFile != "" {
				if err := readJSONFile(state, documentFile, &document); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			design, err := client.Designs.Create(cmd.Context(), &fopost.CreateDesignRequest{
				WorkspaceID:  workspaceID,
				Kind:         kind,
				Name:         name,
				Document:     document,
				BrandVoiceID: brandVoiceID,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(design, func() {
				printer.Success("Created design %s.", design.ID)
				printDesign(printer, design)
			})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "story, post, carousel, thumbnail, card or collage (required)")
	cmd.Flags().StringVar(&name, "name", "", "design name")
	cmd.Flags().StringVar(&documentFile, "document-file", "", "JSON editor document to start from (- for stdin); blank page when omitted")
	cmd.Flags().StringVar(&brandVoiceID, "brand-voice-id", "", "brand whose colours and fonts the design wears")
	return cmd
}

func newDesignsDuplicateCmd(state *State) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "duplicate <design-id>",
		Short: "Copy a design",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			design, err := client.Designs.Duplicate(cmd.Context(), args[0], name)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(design, func() {
				printer.Success("Created design %s.", design.ID)
				printDesign(printer, design)
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "name of the copy")
	return cmd
}

func newDesignsExportCmd(state *State) *cobra.Command {
	var (
		format   string
		pages    []int
		duration int
	)
	cmd := &cobra.Command{
		Use:   "export <design-id>",
		Short: "Render a design into the media library",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format == "" {
				return usageErrorf("--format is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Designs.Export(cmd.Context(), args[0], &fopost.ExportDesignRequest{
				Format:      strings.ToLower(format),
				PageIndexes: pages,
				DurationSec: duration,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				rows := make([][]string, 0, len(result.Files))
				for _, file := range result.Files {
					rows = append(rows, []string{file.MediaID, itoa(file.PageIndex), file.MimeType, output.Dash(file.PreviewURL)})
				}
				printer.Table([]string{"media id", "page", "type", "preview"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "png, jpg, pdf, mp4 or gif (required)")
	cmd.Flags().IntSliceVar(&pages, "page", nil, "page index to export, from 0 (repeatable; every page when omitted)")
	cmd.Flags().IntVar(&duration, "duration", 0, "seconds for mp4 or gif: 5, 10 or 15")
	return cmd
}

func newDesignsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <design-id>",
		Short: "Remove a design",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete design %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if _, err := client.Designs.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted design %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func printDesign(printer *output.Printer, design *fopost.Design) {
	printer.Fields([][2]string{
		{"ID", design.ID},
		{"Name", design.Name},
		{"Kind", design.Kind},
		{"Size", fmt.Sprintf("%dx%d", design.Width, design.Height)},
		{"Thumbnail", output.Dash(strValue(design.ThumbnailURL))},
		{"Created", output.Stamp(design.CreatedAt.Time, design.CreatedAt.Raw)},
		{"Updated", output.Stamp(design.UpdatedAt.Time, design.UpdatedAt.Raw)},
	})
}
