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

func init() { register(newPostsCmd) }

func newPostsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "posts",
		Aliases: []string{"post"},
		Short:   "Compose, schedule, publish, and inspect posts",
	}
	cmd.AddCommand(
		newPostsListCmd(state),
		newPostsGetCmd(state),
		newPostsCreateCmd(state),
		newPostsPublishCmd(state),
		newPostsCancelCmd(state),
		newPostsDeleteCmd(state),
		newPostsDuplicateCmd(state),
		newPostsPreflightCmd(state),
		newPostsDeliveriesCmd(state),
	)
	return cmd
}

func newPostsListCmd(state *State) *cobra.Command {
	params := &fopost.ListPostsParams{}
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List posts in a workspace",
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
			params.WorkspaceID = resolved.Workspace
			printer := state.Printer()

			if all {
				posts, err := client.Posts.ListAll(cmd.Context(), params)
				if err != nil {
					return err
				}
				return printer.Value(posts, func() { printer.Table(postColumns(), postRows(posts)) })
			}

			page, err := client.Posts.List(cmd.Context(), params)
			if err != nil {
				return err
			}
			return printer.Value(page, func() {
				printer.Table(postColumns(), postRows(page.Data))
				if page.Meta.Total > 0 {
					printer.Line("")
					printer.Line("Page %d of %d · %d post(s) total", max(page.Meta.CurrentPage, 1), max(page.Meta.LastPage, 1), page.Meta.Total)
				}
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&params.Status, "status", "", "draft, scheduled, publishing, published, partially_failed, failed, or cancelled")
	flags.StringVar(&params.Search, "search", "", "match post text")
	flags.StringVar(&params.Platform, "platform", "", "narrow to one platform")
	flags.StringVar(&params.Label, "label", "", "narrow to one label")
	flags.StringVar(&params.AccountID, "account", "", "narrow to one account id")
	flags.StringVar(&params.From, "from", "", "start of the window, YYYY-MM-DD")
	flags.StringVar(&params.To, "to", "", "end of the window, YYYY-MM-DD")
	flags.StringVar(&params.Sort, "sort", "", "\"oldest\" to reverse the newest-first order")
	flags.IntVar(&params.Page, "page", 0, "page number")
	flags.IntVar(&params.PerPage, "per-page", 0, "posts per page")
	flags.BoolVar(&all, "all", false, "walk every page instead of one")
	return cmd
}

func postColumns() []string {
	return []string{"id", "status", "scheduled", "accounts", "text"}
}

func postRows(posts []fopost.Post) [][]string {
	rows := make([][]string, 0, len(posts))
	for _, post := range posts {
		rows = append(rows, []string{
			post.ID,
			post.Status,
			output.Stamp(post.ScheduleAt.Time, post.ScheduleAt.Raw),
			itoa(len(post.Accounts)),
			output.Truncate(firstText(post.Content), 48),
		})
	}
	return rows
}

func firstText(blocks []fopost.ContentBlock) string {
	for _, block := range blocks {
		if strings.TrimSpace(block.Text) != "" {
			return block.Text
		}
	}
	return ""
}

func newPostsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <post-id>",
		Short: "Show one post with its content and targets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			post, err := client.Posts.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(post, func() {
				printer.Fields([][2]string{
					{"ID", post.ID},
					{"Workspace", post.WorkspaceID},
					{"Status", post.Status},
					{"Type", output.Dash(post.ContentType)},
					{"Scheduled", output.Stamp(post.ScheduleAt.Time, post.ScheduleAt.Raw)},
					{"Created", output.Stamp(post.CreatedAt.Time, post.CreatedAt.Raw)},
				})
				for index, block := range post.Content {
					printer.Line("")
					if len(post.Content) > 1 {
						printer.Line("--- block %d ---", index+1)
					}
					printer.Line("%s", block.Text)
					for _, media := range block.Media {
						printer.Line("  [%s] %s", media.Type, output.Dash(media.Name))
					}
				}
				if len(post.Accounts) == 0 {
					return
				}
				printer.Line("")
				rows := make([][]string, 0, len(post.Accounts))
				for _, account := range post.Accounts {
					rows = append(rows, []string{
						account.ID,
						account.Platform,
						output.Dash(account.Username),
						output.Dash(account.PublishStatus),
						output.Dash(account.ExternalURL),
					})
				}
				printer.Table([]string{"account id", "platform", "username", "delivery", "url"}, rows)
			})
		},
	}
}

type createOptions struct {
	accounts   []string
	group      string
	text       string
	textFile   string
	media      []string
	scheduleAt string
	draft      bool
	publish    bool
	labels     []string
	title      string
}

type createResult struct {
	Post    *fopost.Post           `json:"post"`
	Media   []fopost.UploadedMedia `json:"media,omitempty"`
	Publish *fopost.PublishResult  `json:"publish,omitempty"`
}

func newPostsCreateCmd(state *State) *cobra.Command {
	opts := &createOptions{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a draft or scheduled post, optionally publishing it",
		Long: "Creates a post from --text, a file, or standard input.\n\n" +
			"Local files given with --media are uploaded to the workspace's media library\n" +
			"first and attached to the post. Without --schedule-at or --publish the post is\n" +
			"created as a draft; nothing reaches a platform until it is published or its\n" +
			"scheduled time arrives.",
		Example: strings.Join([]string{
			"  fopost posts create --account acc_1 --text \"Shipping today\" --publish",
			"  fopost posts create --account acc_1 --text-file post.md --schedule-at \"2026-09-01 09:00\"",
			"  git log -1 --pretty=%s | fopost posts create --account acc_1 --text-file - --draft",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPostsCreate(cmd, state, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringArrayVar(&opts.accounts, "account", nil, "account id to post to (repeatable; this or --group is required)")
	flags.StringVar(&opts.group, "group", "", "account group id; posts to every account in it")
	flags.StringVar(&opts.text, "text", "", "post text")
	flags.StringVar(&opts.textFile, "text-file", "", "read the post text from a file, or \"-\" for stdin")
	flags.StringArrayVar(&opts.media, "media", nil, "local file to upload and attach (repeatable)")
	flags.StringVar(&opts.scheduleAt, "schedule-at", "", "when to publish, RFC 3339 or \"2026-09-01 09:00\" in local time")
	flags.BoolVar(&opts.draft, "draft", false, "save as a draft (the default when no schedule is given)")
	flags.BoolVar(&opts.publish, "publish", false, "publish immediately after creating")
	flags.StringArrayVar(&opts.labels, "label", nil, "label id to attach (repeatable)")
	flags.StringVar(&opts.title, "title", "", "title, for platforms that use one")
	return cmd
}

func runPostsCreate(cmd *cobra.Command, state *State, opts *createOptions) error {
	if len(opts.accounts) == 0 && opts.group == "" {
		return usageErrorf("at least one --account or a --group is required")
	}
	if opts.publish && opts.scheduleAt != "" {
		return usageErrorf("--publish and --schedule-at are alternatives; pass one")
	}
	if opts.publish && opts.draft {
		return usageErrorf("--publish and --draft are alternatives; pass one")
	}

	text, err := readText(opts.text, opts.textFile, state.In)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" && len(opts.media) == 0 {
		return usageErrorf("a post needs text or media. Pass --text, --text-file, or --media")
	}

	scheduleAt, err := parseSchedule(opts.scheduleAt)
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
	printer := state.Printer()

	block := fopost.ContentBlock{Text: text}
	var uploaded []fopost.UploadedMedia
	if len(opts.media) > 0 {
		uploaded, err = uploadFiles(cmd, client, workspaceID, opts.media)
		if err != nil {
			return err
		}
		for _, asset := range uploaded {
			block.Media = append(block.Media, asset.AsMediaItem())
		}
		printer.Line("Uploaded %d file(s).", len(uploaded))
	}

	body := &fopost.CreatePostRequest{
		WorkspaceID:    workspaceID,
		Accounts:       opts.accounts,
		AccountGroupID: opts.group,
		Content:        []fopost.ContentBlock{block},
		Labels:         opts.labels,
		Status:         fopost.PostStatusDraft,
	}
	if opts.title != "" {
		body.Title = fopost.String(opts.title)
	}
	if scheduleAt != nil {
		body.Status = fopost.PostStatusScheduled
		body.ScheduleAt = scheduleAt
	}

	post, err := client.Posts.Create(cmd.Context(), body)
	if err != nil {
		return err
	}

	result := createResult{Post: post, Media: uploaded}
	if opts.publish {
		published, err := client.Posts.Publish(cmd.Context(), post.ID, nil)
		if err != nil {
			// The draft exists; say so rather than leaving it a mystery.
			printer.Warn("Post %s was created but publishing failed.", post.ID)
			return err
		}
		result.Publish = published
	}

	return printer.Value(result, func() {
		switch {
		case result.Publish != nil:
			printer.Success("Created %s and queued %d delivery(s).", post.ID, len(result.Publish.Deliveries))
			printDeliveries(printer, result.Publish.Deliveries)
			for _, warning := range result.Publish.HealthWarnings {
				printer.Warn("%s (%s): %s", warning.Platform, warning.AccountID, warning.Message)
			}
		case body.Status == fopost.PostStatusScheduled:
			printer.Success("Scheduled %s for %s.", post.ID, output.Stamp(post.ScheduleAt.Time, post.ScheduleAt.Raw))
		default:
			printer.Success("Created draft %s.", post.ID)
		}
	})
}

func uploadFiles(cmd *cobra.Command, client *fopost.Client, workspaceID string, paths []string) ([]fopost.UploadedMedia, error) {
	files := make([]fopost.File, 0, len(paths))
	handles := make([]*os.File, 0, len(paths))
	defer func() {
		for _, handle := range handles {
			handle.Close()
		}
	}()
	for _, path := range paths {
		handle, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", path, err)
		}
		handles = append(handles, handle)
		files = append(files, fopost.File{Name: filepath.Base(path), Content: handle})
	}
	return client.Media.Upload(cmd.Context(), workspaceID, files...)
}

func printDeliveries(printer *output.Printer, deliveries []fopost.PublishDelivery) {
	if len(deliveries) == 0 {
		return
	}
	rows := make([][]string, 0, len(deliveries))
	for _, delivery := range deliveries {
		rows = append(rows, []string{
			delivery.AccountID,
			delivery.Status,
			output.Dash(delivery.ExternalURL),
			output.Dash(delivery.ErrorMessage),
		})
	}
	printer.Table([]string{"account id", "status", "url", "error"}, rows)
}

func newPostsPublishCmd(state *State) *cobra.Command {
	var (
		accounts []string
		dryRun   bool
	)
	cmd := &cobra.Command{
		Use:   "publish <post-id>",
		Short: "Queue a post for immediate delivery",
		Long:  "Queues a post for delivery. It returns once the deliveries are queued, not once\nthey are live — poll `fopost posts deliveries <id>` for the outcome.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Posts.Publish(cmd.Context(), args[0], &fopost.PublishOptions{
				AccountIDs: accounts,
				DryRun:     dryRun,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				if result.DryRun {
					printer.Line("Dry run — nothing was sent.")
					rows := make([][]string, 0, len(result.Accounts))
					for _, account := range result.Accounts {
						rows = append(rows, []string{account.AccountID, account.Platform})
					}
					printer.Table([]string{"account id", "platform"}, rows)
					return
				}
				printer.Success("Queued %d delivery(s). Post is %s.", len(result.Deliveries), output.Dash(result.PostStatus))
				printDeliveries(printer, result.Deliveries)
				for _, warning := range result.HealthWarnings {
					printer.Warn("%s (%s): %s", warning.Platform, warning.AccountID, warning.Message)
				}
			})
		},
	}
	cmd.Flags().StringArrayVar(&accounts, "account", nil, "publish to a subset of the post's accounts (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without sending anything to a platform")
	return cmd
}

func newPostsCancelCmd(state *State) *cobra.Command {
	var accounts []string
	cmd := &cobra.Command{
		Use:   "cancel <post-id>",
		Short: "Stop the deliveries that have not gone out yet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Posts.Cancel(cmd.Context(), args[0], &fopost.CancelOptions{AccountIDs: accounts})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Success("Cancelled %d delivery(s). Post is %s.", len(result.Deliveries), output.Dash(result.PostStatus))
			})
		},
	}
	cmd.Flags().StringArrayVar(&accounts, "account", nil, "cancel only these account ids (repeatable)")
	return cmd
}

func newPostsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <post-id>",
		Short: "Delete a post",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete post %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Posts.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted post %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newPostsDuplicateCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "duplicate <post-id>",
		Short: "Copy a post into a new draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			copied, err := client.Posts.Duplicate(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(copied, func() {
				printer.Success("Duplicated into %s (%s).", copied.ID, copied.Status)
			})
		},
	}
}

func newPostsPreflightCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "preflight <post-id>",
		Short: "Check a post against every target platform without publishing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Posts.Preflight(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			if err := printer.Value(result, func() {
				rows := make([][]string, 0, len(result.Accounts))
				for _, account := range result.Accounts {
					rows = append(rows, []string{
						account.AccountID,
						account.Platform,
						boolLabel(account.Ready, "ready", "blocked"),
						output.Dash(strings.Join(account.Issues, "; ")),
					})
				}
				printer.Table([]string{"account id", "platform", "state", "issues"}, rows)
				for _, account := range result.Accounts {
					for _, signal := range account.Signals {
						printer.Line("%s · %s: %s", account.Platform, signal.Level, signal.Message)
					}
				}
			}); err != nil {
				return err
			}
			if !result.Ready {
				return usageErrorf("the post is not ready to publish")
			}
			return nil
		},
	}
}

func newPostsDeliveriesCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "deliveries <post-id>",
		Short: "Show the current delivery record per account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			deliveries, err := client.Posts.Deliveries(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deliveries, func() {
				rows := make([][]string, 0, len(deliveries))
				for _, delivery := range deliveries {
					rows = append(rows, []string{
						delivery.AccountID,
						delivery.Platform,
						delivery.Status,
						fmt.Sprintf("%d/%d", delivery.Attempts, delivery.MaxAttempts),
						output.Stamp(delivery.PostedAt.Time, delivery.PostedAt.Raw),
						output.Dash(delivery.ExternalURL),
					})
				}
				printer.Table([]string{"account id", "platform", "status", "attempts", "posted", "url"}, rows)
			})
		},
	}
}
