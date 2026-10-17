package cmd

import (
	"fmt"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newKnowledgeCmd) }

func newKnowledgeCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "knowledge",
		Aliases: []string{"kb"},
		Short:   "Manage what the agent knows about your business",
		Long: "The workspace knowledge base: your own answers, your own pages and your own\n" +
			"files. A drafted inbox reply quotes these instead of inventing a policy.",
	}
	cmd.AddCommand(
		newKnowledgeListCmd(state),
		newKnowledgeAddCmd(state),
		newKnowledgeSyncCmd(state),
		newKnowledgeDeleteCmd(state),
		newKnowledgeSearchCmd(state),
	)
	return cmd
}

func newKnowledgeListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List knowledge sources",
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
			sources, err := client.Knowledge.List(cmd.Context(), resolved.Workspace)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(sources, func() {
				rows := make([][]string, 0, len(sources))
				for _, source := range sources {
					rows = append(rows, []string{
						source.ID,
						output.Truncate(source.Title, 32),
						source.Kind,
						source.Status,
						fmt.Sprintf("%d", source.ChunkCount),
					})
				}
				printer.Table([]string{"ID", "TITLE", "KIND", "STATUS", "PASSAGES"}, rows)
			})
		},
	}
}

func newKnowledgeAddCmd(state *State) *cobra.Command {
	var kind, title, content, url, mediaID, brandVoiceID string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a knowledge source and queue it for indexing",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" {
				return usageErrorf("--title is required")
			}
			switch kind {
			case "faq", "text":
				if content == "" {
					return usageErrorf("--content is required for a %s source", kind)
				}
			case "url":
				if url == "" {
					return usageErrorf("--url is required for a url source")
				}
			case "file":
				if mediaID == "" {
					return usageErrorf("--media-id is required for a file source")
				}
			default:
				return usageErrorf("--kind must be faq, text, url, or file")
			}

			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			source, err := client.Knowledge.Create(cmd.Context(), fopost.CreateKnowledgeSourceRequest{
				Kind:         kind,
				Title:        title,
				Content:      content,
				URL:          url,
				MediaID:      mediaID,
				BrandVoiceID: brandVoiceID,
				WorkspaceID:  workspaceID,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(source, func() {
				printer.Success("Added %s (%s). Indexing in the background.", source.Title, source.ID)
			})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "faq", "faq, text, url, or file")
	cmd.Flags().StringVar(&title, "title", "", "what to call it (required)")
	cmd.Flags().StringVar(&content, "content", "", "the text, for a faq or text source")
	cmd.Flags().StringVar(&url, "url", "", "a page on your own site, for a url source")
	cmd.Flags().StringVar(&mediaID, "media-id", "", "a text or CSV media item, for a file source")
	cmd.Flags().StringVar(&brandVoiceID, "brand-voice-id", "", "scope the source to one brand")
	return cmd
}

func newKnowledgeSyncCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "sync <source-id>",
		Short: "Read a source again; a url source is re-fetched",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Knowledge.Sync(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]string{"id": args[0], "status": "pending"}, func() {
				printer.Success("Queued %s for re-indexing.", args[0])
			})
		},
	}
}

func newKnowledgeDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <source-id>",
		Short: "Delete a source and every passage indexed from it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete knowledge source %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Knowledge.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted knowledge source %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newKnowledgeSearchCmd(state *State) *cobra.Command {
	var topK int
	cmd := &cobra.Command{
		Use:   "search <question>",
		Short: "Show the passages closest to a question",
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
			matches, err := client.Knowledge.Search(cmd.Context(), args[0], fopost.SearchKnowledgeParams{
				TopK:        topK,
				WorkspaceID: resolved.Workspace,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(matches, func() {
				if len(matches) == 0 {
					printer.Success("Nothing saved answers that yet.")
					return
				}
				rows := make([][]string, 0, len(matches))
				for _, match := range matches {
					rows = append(rows, []string{
						output.Truncate(match.SourceTitle, 28),
						fmt.Sprintf("%.2f", match.Score),
						output.Truncate(match.Text, 60),
					})
				}
				printer.Table([]string{"SOURCE", "SCORE", "PASSAGE"}, rows)
			})
		},
	}
	cmd.Flags().IntVar(&topK, "top", 5, "how many passages to show, max 20")
	return cmd
}
