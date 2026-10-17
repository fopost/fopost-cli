package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() {
	register(newBlogCmd)
}

func newBlogCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blog",
		Short: "Plan keywords, write articles and publish them to a website",
		Long: "Plan keywords, write articles and publish them to a website.\n\n" +
			"A post goes to a social network; an article goes to a website. Planning " +
			"keywords, requesting an article and regenerating one spend AI credits. " +
			"Nothing publishes until `blog articles publish` is run.",
	}
	cmd.AddCommand(
		newBlogSettingsCmd(state),
		newBlogKeywordsCmd(state),
		newBlogArticlesCmd(state),
	)
	return cmd
}

// secretNote is printed beside a website secret, which the API never shows twice.
const secretNote = "Store this website secret now. It is not shown again."

func newBlogSettingsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Read and change how the blog plans, writes and publishes",
	}
	cmd.AddCommand(
		newBlogSettingsGetCmd(state),
		newBlogSettingsSetCmd(state),
		newBlogSettingsRotateSecretCmd(state),
		newBlogSettingsTestWebsiteCmd(state),
	)
	return cmd
}

func strOrDash(s *string) string {
	if s == nil {
		return "-"
	}
	return output.Dash(*s)
}

func planRunText(run *fopost.BlogPlanRun) string {
	if run == nil {
		return "-"
	}
	text := run.Status
	if run.Added != nil {
		text += fmt.Sprintf(", %d added", *run.Added)
	}
	if run.Error != nil {
		text += ": " + *run.Error
	}
	return text
}

func settingsRows(settings *fopost.BlogSettings) [][2]string {
	return [][2]string{
		{"site url", strOrDash(settings.SiteURL)},
		{"topic", strOrDash(settings.Topic)},
		{"audience", strOrDash(settings.Audience)},
		{"goal", strOrDash(settings.Goal)},
		{"focus topics", strOrDash(settings.FocusTopics)},
		{"avoid topics", strOrDash(settings.AvoidTopics)},
		{"language", output.Dash(settings.Language)},
		{"location code", strconv.Itoa(settings.LocationCode)},
		{"cadence per week", strconv.Itoa(settings.CadencePerWeek)},
		{"review hold days", strconv.Itoa(settings.ReviewHoldDays)},
		{"image tier", output.Dash(settings.ImageTier)},
		{"in-body images", strconv.Itoa(settings.InBodyImages)},
		{"words", fmt.Sprintf("%d-%d", settings.MinWords, settings.MaxWords)},
		{"brand voice", strOrDash(settings.BrandVoiceID)},
		{"target", strOrDash(settings.TargetKind)},
		{"target account", strOrDash(settings.TargetAccountID)},
		{"website url", strOrDash(settings.WebsiteURL)},
		{"website secret", map[bool]string{true: "set", false: "-"}[settings.HasWebsiteSecret]},
		{"plan run", planRunText(settings.PlanRun)},
		{"keyword data", strconv.FormatBool(settings.KeywordDataAvailable)},
		{"updated", settings.UpdatedAt.String()},
	}
}

func newBlogSettingsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show the blog settings",
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
			settings, err := client.Blog.GetSettings(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(settings, func() {
				printer.Fields(settingsRows(settings))
			})
		},
	}
}

func newBlogSettingsSetCmd(state *State) *cobra.Command {
	var siteURL, topic, audience, goal, focusTopics, avoidTopics, language, imageTier, brandVoiceID, targetKind, targetAccountID, websiteURL string
	var locationCode, cadencePerWeek, reviewHoldDays, inBodyImages, minWords, maxWords int
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Change blog settings; only the flags given are sent",
		Long: "Change blog settings; only the flags given are sent. Setting --website-url " +
			"for the first time issues the signing secret, printed once and never again. " +
			"Run `blog settings rotate-secret` to replace it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().NFlag() == 0 {
				return usageErrorf("pass at least one setting flag; see `fopost blog settings set --help`")
			}
			flags := cmd.Flags()
			patch := &fopost.UpdateBlogSettingsRequest{Language: language, ImageTier: imageTier}
			setStr := func(name string, value string, into **string) {
				if flags.Changed(name) {
					*into = &value
				}
			}
			setInt := func(name string, value int, into **int) {
				if flags.Changed(name) {
					*into = &value
				}
			}
			setStr("site-url", siteURL, &patch.SiteURL)
			setStr("topic", topic, &patch.Topic)
			setStr("audience", audience, &patch.Audience)
			setStr("goal", goal, &patch.Goal)
			setStr("focus-topics", focusTopics, &patch.FocusTopics)
			setStr("avoid-topics", avoidTopics, &patch.AvoidTopics)
			setStr("brand-voice", brandVoiceID, &patch.BrandVoiceID)
			setStr("target-kind", targetKind, &patch.TargetKind)
			setStr("target-account", targetAccountID, &patch.TargetAccountID)
			setStr("website-url", websiteURL, &patch.WebsiteURL)
			setInt("location-code", locationCode, &patch.LocationCode)
			setInt("cadence-per-week", cadencePerWeek, &patch.CadencePerWeek)
			setInt("review-hold-days", reviewHoldDays, &patch.ReviewHoldDays)
			setInt("in-body-images", inBodyImages, &patch.InBodyImages)
			setInt("min-words", minWords, &patch.MinWords)
			setInt("max-words", maxWords, &patch.MaxWords)

			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			updated, err := client.Blog.UpdateSettings(cmd.Context(), workspaceID, patch)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(updated, func() {
				printer.Success("Updated blog settings.")
				printer.Fields(settingsRows(&updated.Settings))
				if updated.WebsiteSecret != nil {
					printer.Line("%s", *updated.WebsiteSecret)
					printer.Warn(secretNote)
				}
			})
		},
	}
	cmd.Flags().StringVar(&siteURL, "site-url", "", "the website the articles are written for")
	cmd.Flags().StringVar(&topic, "topic", "", "what the blog is about")
	cmd.Flags().StringVar(&audience, "audience", "", "who reads it")
	cmd.Flags().StringVar(&goal, "goal", "", "what the blog is for: traffic, leads, sales, signups, or authority")
	cmd.Flags().StringVar(&focusTopics, "focus-topics", "", "what the blog wants to write about")
	cmd.Flags().StringVar(&avoidTopics, "avoid-topics", "", "what the blog never writes about")
	cmd.Flags().StringVar(&language, "language", "", "language code the articles are written in")
	cmd.Flags().IntVar(&locationCode, "location-code", 0, "location the keyword data is read for")
	cmd.Flags().IntVar(&cadencePerWeek, "cadence-per-week", 0, "how many articles to write a week")
	cmd.Flags().IntVar(&reviewHoldDays, "review-hold-days", 0, "days before its planned date that a picked keyword starts being written")
	cmd.Flags().StringVar(&imageTier, "image-tier", "", "image quality: draft, standard, or best")
	cmd.Flags().IntVar(&inBodyImages, "in-body-images", 0, "images generated inside the body")
	cmd.Flags().IntVar(&minWords, "min-words", 0, "shortest article to write")
	cmd.Flags().IntVar(&maxWords, "max-words", 0, "longest article to write")
	cmd.Flags().StringVar(&brandVoiceID, "brand-voice", "", "the brand the writer follows")
	cmd.Flags().StringVar(&targetKind, "target-kind", "", "where articles publish: product_blog, account, or website")
	cmd.Flags().StringVar(&targetAccountID, "target-account", "", "the connected account for the account target")
	cmd.Flags().StringVar(&websiteURL, "website-url", "", "the endpoint that receives published articles as a signed webhook")
	return cmd
}

func newBlogSettingsRotateSecretCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rotate-secret",
		Short: "Issue a new website secret; the old one stops verifying at once",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				prompt := "Rotate the website secret? Deliveries signed with the old one stop verifying. This cannot be undone."
				if err := confirm(state, prompt); err != nil {
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
			secret, err := client.Blog.RotateWebsiteSecret(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(struct {
				WebsiteSecret string `json:"websiteSecret"`
			}{secret}, func() {
				printer.Line("%s", secret)
				printer.Warn(secretNote)
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newBlogSettingsTestWebsiteCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "test-website",
		Short: "Send a signed test delivery to the website URL",
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
			result, err := client.Blog.TestWebsite(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				status := "-"
				if result.Status != nil {
					status = strconv.Itoa(*result.Status)
				}
				printer.Fields([][2]string{
					{"ok", strconv.FormatBool(result.OK)},
					{"status", status},
					{"error", strOrDash(result.Error)},
				})
			})
		},
	}
}

func newBlogKeywordsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keywords",
		Aliases: []string{"keyword"},
		Short:   "Plan the keywords the articles are written for",
	}
	cmd.AddCommand(
		newBlogKeywordsListCmd(state),
		newBlogKeywordsPlanCmd(state),
		newBlogKeywordsAddCmd(state),
		newBlogKeywordsUpdateCmd(state),
		newBlogKeywordsDeleteCmd(state),
	)
	return cmd
}

func intOrDash(n *int) string {
	if n == nil {
		return "-"
	}
	return strconv.Itoa(*n)
}

func keywordRow(keyword fopost.BlogKeyword) []string {
	cpc := "-"
	if keyword.CPC != nil {
		cpc = strconv.FormatFloat(*keyword.CPC, 'f', 2, 64)
	}
	return []string{
		keyword.ID,
		output.Truncate(keyword.Keyword, 36),
		keyword.Status,
		intOrDash(keyword.SearchVolume),
		intOrDash(keyword.Difficulty),
		cpc,
		strOrDash(keyword.Intent),
		strOrDash(keyword.PlannedFor),
	}
}

var keywordHeaders = []string{"id", "keyword", "status", "volume", "difficulty", "cpc", "intent", "planned"}

func newBlogKeywordsListCmd(state *State) *cobra.Command {
	var status []string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the keyword plan",
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
			keywords, err := client.Blog.ListKeywords(cmd.Context(), workspaceID, &fopost.ListBlogKeywordsParams{
				Status: status,
				Limit:  limit,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(keywords, func() {
				rows := make([][]string, 0, len(keywords))
				for _, keyword := range keywords {
					rows = append(rows, keywordRow(keyword))
				}
				printer.Table(keywordHeaders, rows)
			})
		},
	}
	cmd.Flags().StringSliceVar(&status, "status", nil, "suggested, planned, written, or dismissed; repeatable")
	cmd.Flags().IntVar(&limit, "limit", 0, "most keywords to return")
	return cmd
}

func newBlogKeywordsPlanCmd(state *State) *cobra.Command {
	var seeds []string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Research keywords in the background; uses AI credits",
		Long: "Research keywords in the background; uses AI credits. With no --seed the " +
			"research starts from the topic in the settings. New keywords arrive as " +
			"suggested; run `blog keywords list` to see them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			if err := client.Blog.PlanKeywords(cmd.Context(), workspaceID, seeds); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(struct {
				Status string `json:"status"`
			}{"queued"}, func() {
				printer.Success("Queued keyword research.")
			})
		},
	}
	cmd.Flags().StringSliceVar(&seeds, "seed", nil, "a term to research from; repeatable")
	return cmd
}

func newBlogKeywordsAddCmd(state *State) *cobra.Command {
	var plannedFor string
	cmd := &cobra.Command{
		Use:   "add <keyword>",
		Short: "Add a keyword to the plan",
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
			keyword, err := client.Blog.AddKeyword(cmd.Context(), workspaceID, &fopost.AddBlogKeywordRequest{
				Keyword:    args[0],
				PlannedFor: plannedFor,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(keyword, func() {
				printer.Success("Added keyword %s.", keyword.ID)
			})
		},
	}
	cmd.Flags().StringVar(&plannedFor, "planned-for", "", "the day to write it, as YYYY-MM-DD")
	return cmd
}

func newBlogKeywordsUpdateCmd(state *State) *cobra.Command {
	var status, plannedFor string
	cmd := &cobra.Command{
		Use:   "update <keyword-id>",
		Short: "Change a keyword's status or planned date",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return usageErrorf("pass --status or --planned-for")
			}
			patch := &fopost.UpdateBlogKeywordRequest{Status: status}
			if cmd.Flags().Changed("planned-for") {
				patch.PlannedFor = &plannedFor
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			keyword, err := client.Blog.UpdateKeyword(cmd.Context(), workspaceID, args[0], patch)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(keyword, func() {
				printer.Table(keywordHeaders, [][]string{keywordRow(*keyword)})
			})
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "suggested, planned, written, or dismissed")
	cmd.Flags().StringVar(&plannedFor, "planned-for", "", "the day to write it, as YYYY-MM-DD; empty clears it")
	return cmd
}

func newBlogKeywordsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <keyword-id>",
		Short: "Remove a keyword from the plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf("Delete keyword %s? This cannot be undone.", args[0])
				if err := confirm(state, prompt); err != nil {
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
			if err := client.Blog.DeleteKeyword(cmd.Context(), workspaceID, args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted keyword %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newBlogArticlesCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "articles",
		Aliases: []string{"article"},
		Short:   "Request, review and publish articles",
	}
	cmd.AddCommand(
		newBlogArticlesListCmd(state),
		newBlogArticlesRequestCmd(state),
		newBlogArticlesGetCmd(state),
		newBlogArticlesEditCmd(state),
		newBlogArticlesRegenerateCmd(state),
		newBlogArticlesPublishCmd(state),
		newBlogArticlesRejectCmd(state),
		newBlogArticlesDeleteCmd(state),
	)
	return cmd
}

func timeOrDash(t *fopost.Time) string {
	if t == nil {
		return "-"
	}
	return output.Dash(t.String())
}

func newBlogArticlesListCmd(state *State) *cobra.Command {
	var status []string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List articles, newest first",
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
			articles, err := client.Blog.ListArticles(cmd.Context(), workspaceID, &fopost.ListBlogArticlesParams{
				Status: status,
				Limit:  limit,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(articles, func() {
				rows := make([][]string, 0, len(articles))
				for _, article := range articles {
					rows = append(rows, []string{
						article.ID,
						output.Truncate(article.Title, 40),
						article.Status,
						strconv.Itoa(article.WordCount),
						strOrDash(article.Keyword),
						timeOrDash(article.PublishAt),
						timeOrDash(article.PublishedAt),
					})
				}
				printer.Table([]string{"id", "title", "status", "words", "keyword", "publish at", "published"}, rows)
			})
		},
	}
	cmd.Flags().StringSliceVar(&status, "status", nil, "queued, writing, draft, scheduled, publishing, published, failed, or rejected; repeatable")
	cmd.Flags().IntVar(&limit, "limit", 0, "most articles to return")
	return cmd
}

func newBlogArticlesRequestCmd(state *State) *cobra.Command {
	var title, keyword, keywordID, instructions string
	cmd := &cobra.Command{
		Use:   "request",
		Short: "Queue a new article; uses AI credits",
		Long: "Queue a new article; uses AI credits. Give it a --title, a --keyword, or a " +
			"--keyword-id from the plan. It comes back queued and is written in the " +
			"background; `blog articles get` shows it once it is a draft.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" && keyword == "" && keywordID == "" {
				return usageErrorf("pass one of --title, --keyword, or --keyword-id")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			article, err := client.Blog.RequestArticle(cmd.Context(), workspaceID, &fopost.RequestBlogArticleRequest{
				Title:        title,
				Keyword:      keyword,
				KeywordID:    keywordID,
				Instructions: instructions,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Success("Queued article %s. Nothing is published until you publish it.", article.ID)
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "the title to write to")
	cmd.Flags().StringVar(&keyword, "keyword", "", "the search term to write for")
	cmd.Flags().StringVar(&keywordID, "keyword-id", "", "a keyword from the plan")
	cmd.Flags().StringVar(&instructions, "instructions", "", "guidance for the writer")
	return cmd
}

func articleRows(article *fopost.BlogArticleFull) [][2]string {
	return [][2]string{
		{"title", article.Title},
		{"slug", article.Slug},
		{"status", article.Status},
		{"keyword", strOrDash(article.Keyword)},
		{"words", strconv.Itoa(article.WordCount)},
		{"tags", output.Dash(strings.Join(article.Tags, ", "))},
		{"excerpt", output.Truncate(strOrDash(article.Excerpt), 80)},
		{"meta title", strOrDash(article.MetaTitle)},
		{"meta description", output.Truncate(strOrDash(article.MetaDescription), 80)},
		{"images", strconv.Itoa(len(article.Images))},
		{"review until", timeOrDash(article.ReviewUntil)},
		{"publish at", timeOrDash(article.PublishAt)},
		{"published", timeOrDash(article.PublishedAt)},
		{"published url", strOrDash(article.PublishedURL)},
		{"error", strOrDash(article.Error)},
		{"updated", article.UpdatedAt.String()},
	}
}

func newBlogArticlesGetCmd(state *State) *cobra.Command {
	var body bool
	cmd := &cobra.Command{
		Use:   "get <article-id>",
		Short: "Show one article; add --body for the markdown",
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
			article, err := client.Blog.GetArticle(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				if body {
					printer.Line("%s", article.BodyMarkdown)
					return
				}
				printer.Fields(articleRows(article))
			})
		},
	}
	cmd.Flags().BoolVar(&body, "body", false, "print the markdown body instead of the metadata")
	return cmd
}

// readBody resolves the markdown from --body or --body-file, where "-" is stdin.
func readBody(state *State, body, bodyFile string) (string, error) {
	switch {
	case body != "" && bodyFile != "":
		return "", usageErrorf("--body and --body-file are alternatives; pass one")
	case body != "" || bodyFile == "":
		return body, nil
	case bodyFile == "-":
		raw, err := readAll(state)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(raw), "\n"), nil
	default:
		raw, err := os.ReadFile(bodyFile)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", bodyFile, err)
		}
		return strings.TrimRight(string(raw), "\n"), nil
	}
}

func newBlogArticlesEditCmd(state *State) *cobra.Command {
	var title, slug, excerpt, metaTitle, metaDescription, body, bodyFile, publishAt string
	var tags []string
	cmd := &cobra.Command{
		Use:   "edit <article-id>",
		Short: "Edit a draft; only the flags given are sent",
		Long: "Edit a draft; only the flags given are sent. An article still being written " +
			"or already published cannot be edited. --publish-at puts the draft on the " +
			"calendar without publishing it now.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return usageErrorf("pass at least one field flag; see `fopost blog articles edit --help`")
			}
			markdown, err := readBody(state, body, bodyFile)
			if err != nil {
				return err
			}
			when, err := parseSchedule(publishAt)
			if err != nil {
				return err
			}
			flags := cmd.Flags()
			patch := &fopost.UpdateBlogArticleRequest{
				Title:        title,
				Slug:         slug,
				BodyMarkdown: markdown,
				Tags:         tags,
				PublishAt:    when,
			}
			if flags.Changed("excerpt") {
				patch.Excerpt = &excerpt
			}
			if flags.Changed("meta-title") {
				patch.MetaTitle = &metaTitle
			}
			if flags.Changed("meta-description") {
				patch.MetaDescription = &metaDescription
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			article, err := client.Blog.UpdateArticle(cmd.Context(), workspaceID, args[0], patch)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Success("Updated article %s.", article.ID)
				printer.Fields(articleRows(article))
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&slug, "slug", "", "new slug")
	cmd.Flags().StringVar(&excerpt, "excerpt", "", "new excerpt; empty clears it")
	cmd.Flags().StringVar(&metaTitle, "meta-title", "", "new meta title; empty clears it")
	cmd.Flags().StringVar(&metaDescription, "meta-description", "", "new meta description; empty clears it")
	cmd.Flags().StringVar(&body, "body", "", "the markdown body")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "read the markdown body from a file, or - for stdin")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "replace the tags; repeatable")
	cmd.Flags().StringVar(&publishAt, "publish-at", "", "schedule it for this time")
	return cmd
}

func newBlogArticlesRegenerateCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "regenerate <article-id>",
		Short: "Rewrite an article from its brief; uses AI credits",
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
			article, err := client.Blog.RegenerateArticle(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Success("Queued a rewrite of article %s.", article.ID)
			})
		},
	}
}

func newBlogArticlesPublishCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "publish <article-id>",
		Short: "Publish an article to the configured target",
		Long: "Publish an article to the configured target. The settings must name a " +
			"target first; without one the API answers no_target.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf("Publish article %s? It goes live on the configured target.", args[0])
				if err := confirm(state, prompt); err != nil {
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
			article, err := client.Blog.PublishArticle(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Success("Publishing article %s.", article.ID)
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newBlogArticlesRejectCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "reject <article-id>",
		Short: "Reject an article so it never publishes",
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
			article, err := client.Blog.RejectArticle(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Success("Rejected article %s.", article.ID)
			})
		},
	}
}

func newBlogArticlesDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <article-id>",
		Short: "Delete an article",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf("Delete article %s? This cannot be undone.", args[0])
				if err := confirm(state, prompt); err != nil {
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
			if err := client.Blog.DeleteArticle(cmd.Context(), workspaceID, args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted article %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
