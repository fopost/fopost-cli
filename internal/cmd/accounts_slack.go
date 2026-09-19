package cmd

import (
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAccountsSlackCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slack",
		Short: "List a Slack account's channels and members and set its posting identity",
	}
	cmd.AddCommand(
		newSlackChannelsCmd(state),
		newSlackMembersCmd(state),
		newSlackIdentityCmd(state),
		newSlackSetIdentityCmd(state),
	)
	return cmd
}

func newSlackChannelsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "channels <account-id>",
		Short: "List the channels a Slack account can post to",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			channels, err := client.Accounts.ListSlackChannels(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(channels, func() {
				rows := make([][]string, 0, len(channels))
				for _, c := range channels {
					rows = append(rows, []string{c.ID, "#" + c.Name, boolLabel(c.IsPrivate, "yes", "no"), boolLabel(c.IsMember, "yes", "no"), boolLabel(c.IsCurrent, "yes", "no")})
				}
				printer.Table([]string{"id", "name", "private", "member", "current"}, rows)
			})
		},
	}
}

func newSlackMembersCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "members <account-id>",
		Short: "List the people in a Slack account's workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			members, err := client.Accounts.ListSlackMembers(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(members, func() {
				rows := make([][]string, 0, len(members))
				for _, m := range members {
					rows = append(rows, []string{m.ID, m.Name, output.Dash(deref(m.RealName)), output.Dash(deref(m.DisplayName)), boolLabel(m.IsBot, "yes", "no")})
				}
				printer.Table([]string{"id", "name", "real name", "display name", "bot"}, rows)
			})
		},
	}
}

func printSlackIdentity(state *State, identity *fopost.SlackIdentity) error {
	printer := state.Printer()
	return printer.Value(identity, func() {
		printer.Fields([][2]string{
			{"Username", output.Dash(deref(identity.Username))},
			{"Icon URL", output.Dash(deref(identity.IconURL))},
			{"Icon Emoji", output.Dash(deref(identity.IconEmoji))},
		})
	})
}

func newSlackIdentityCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "identity <account-id>",
		Short: "Show the name and icon a Slack account posts under",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			identity, err := client.Accounts.GetSlackIdentity(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printSlackIdentity(state, identity)
		},
	}
}

func newSlackSetIdentityCmd(state *State) *cobra.Command {
	var username, iconURL, iconEmoji string
	var clearUsername, clearIcon bool
	cmd := &cobra.Command{
		Use:   "set-identity <account-id>",
		Short: "Set the name and icon a Slack account posts under",
		Long:  "Sets the posting name and icon. Flags left out keep their value. Pass --icon-url or\n--icon-emoji, not both; setting one clears the other.",
		Example: strings.Join([]string{
			"  fopost accounts slack set-identity acc_1 --username \"Launch Bot\" --icon-emoji :rocket:",
			"  fopost accounts slack set-identity acc_1 --clear-username --clear-icon",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			if flags.Changed("icon-url") && flags.Changed("icon-emoji") {
				return usageErrorf("--icon-url and --icon-emoji are alternatives; pass one")
			}
			if clearUsername && flags.Changed("username") {
				return usageErrorf("--username and --clear-username are alternatives; pass one")
			}
			if clearIcon && (flags.Changed("icon-url") || flags.Changed("icon-emoji")) {
				return usageErrorf("--clear-icon cannot be combined with --icon-url or --icon-emoji")
			}
			body := &fopost.UpdateSlackIdentityRequest{}
			if flags.Changed("username") {
				if strings.TrimSpace(username) == "" {
					return usageErrorf("--username cannot be empty; use --clear-username")
				}
				body.Username = fopost.String(username)
			}
			if flags.Changed("icon-url") {
				if strings.TrimSpace(iconURL) == "" {
					return usageErrorf("--icon-url cannot be empty; use --clear-icon")
				}
				body.IconURL = fopost.String(iconURL)
			}
			if flags.Changed("icon-emoji") {
				if strings.TrimSpace(iconEmoji) == "" {
					return usageErrorf("--icon-emoji cannot be empty; use --clear-icon")
				}
				body.IconEmoji = fopost.String(iconEmoji)
			}
			if clearUsername {
				body.Username = fopost.String("")
			}
			if clearIcon {
				body.IconURL = fopost.String("")
				body.IconEmoji = fopost.String("")
			}
			if body.Username == nil && body.IconURL == nil && body.IconEmoji == nil {
				return usageErrorf("pass at least one of --username, --icon-url, --icon-emoji, --clear-username, --clear-icon")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			identity, err := client.Accounts.UpdateSlackIdentity(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			return printSlackIdentity(state, identity)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "the name posts appear under (1-80 characters)")
	cmd.Flags().StringVar(&iconURL, "icon-url", "", "an http(s) image URL posts appear with")
	cmd.Flags().StringVar(&iconEmoji, "icon-emoji", "", "an emoji code posts appear with, e.g. :rocket:")
	cmd.Flags().BoolVar(&clearUsername, "clear-username", false, "post under the app name again")
	cmd.Flags().BoolVar(&clearIcon, "clear-icon", false, "post with the app icon again")
	return cmd
}
