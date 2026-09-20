package cmd

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAccountsRedditCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reddit",
		Short: "List a Reddit account's subreddits, their rules and flairs, and set its default",
	}
	cmd.AddCommand(
		newRedditSubredditsCmd(state),
		newRedditRulesCmd(state),
		newRedditFlairsCmd(state),
		newRedditSetDefaultCmd(state),
	)
	return cmd
}

func newRedditSubredditsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "subreddits <account-id>",
		Short: "List the subreddits a Reddit account can post to",
		Long:  "Lists the subreddits the account is subscribed to, busiest first, plus its own\nprofile page. A subreddit it may read but not submit to is listed with post=no.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			subreddits, err := client.Accounts.ListRedditSubreddits(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(subreddits, func() {
				rows := make([][]string, 0, len(subreddits))
				for _, s := range subreddits {
					rows = append(rows, []string{
						"r/" + s.Name,
						output.Dash(s.Title),
						strconv.FormatInt(s.Subscribers, 10),
						boolLabel(s.CanPost, "yes", "no"),
						boolLabel(s.FlairEnabled, "yes", "no"),
						boolLabel(s.IsDefault, "yes", "no"),
					})
				}
				printer.Table([]string{"name", "title", "subscribers", "post", "flairs", "default"}, rows)
			})
		},
	}
}

func newRedditRulesCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "rules <account-id> <subreddit>",
		Short: "List a subreddit's rules",
		Long:  "Lists the rules the subreddit publishes, in its own order. Read them before\npublishing there. Pass the subreddit without the r/ prefix.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			rules, err := client.Accounts.ListRedditSubredditRules(cmd.Context(), args[0], subredditArg(args[1]))
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(rules, func() {
				rows := make([][]string, 0, len(rules.Rules))
				for _, r := range rules.Rules {
					rows = append(rows, []string{r.Name, output.Dash(r.AppliesTo), output.Dash(r.Description)})
				}
				printer.Table([]string{"rule", "applies to", "description"}, rows)
			})
		},
	}
}

func newRedditFlairsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "flairs <account-id> <subreddit>",
		Short: "List a subreddit's post flairs",
		Long:  "Lists the post flairs the subreddit offers. A flair id is valid only in the\nsubreddit it came from, and one from elsewhere fails preflight.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			flairs, err := client.Accounts.ListRedditFlairs(cmd.Context(), args[0], subredditArg(args[1]))
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(flairs, func() {
				rows := make([][]string, 0, len(flairs.Flairs))
				for _, f := range flairs.Flairs {
					rows = append(rows, []string{f.ID, f.Text, boolLabel(f.Editable, "yes", "no")})
				}
				printer.Table([]string{"id", "text", "editable"}, rows)
			})
		},
	}
}

func newRedditSetDefaultCmd(state *State) *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "set-default <account-id> [subreddit]",
		Short: "Set where a Reddit account's posts go when a post names no subreddit",
		Long:  "Sets the account's default subreddit. With --clear, posts fall back to the\naccount's own profile page, which always takes a post.",
		Example: strings.Join([]string{
			"  fopost accounts reddit set-default acc_1 webdev",
			"  fopost accounts reddit set-default acc_1 --clear",
		}, "\n"),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			subreddit := ""
			if len(args) == 2 {
				subreddit = subredditArg(args[1])
			}
			if clear && subreddit != "" {
				return usageErrorf("a subreddit and --clear are alternatives; pass one")
			}
			if !clear && subreddit == "" {
				return usageErrorf("pass a subreddit, or --clear to use the profile page")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.SetRedditDefaultSubreddit(cmd.Context(), args[0], subreddit)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Fields([][2]string{{"Default Subreddit", output.Dash(result.Subreddit)}})
			})
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "post to the account's own profile page instead")
	return cmd
}

// A subreddit is named without its prefix; accepting "r/name" costs nothing.
func subredditArg(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "r/")
}
