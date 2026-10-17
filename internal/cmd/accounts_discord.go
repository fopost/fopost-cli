package cmd

import (
	"strconv"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAccountsDiscordCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discord",
		Short: "Manage a Discord bot connection: its channel, identity, events, members and roles",
		Long:  "Works on a Discord account connected with the bot. A connection made with a\nwebhook answers 409 webhook_connection; upgrade it to the bot first.",
	}
	cmd.AddCommand(
		newDiscordChannelsCmd(state),
		newDiscordSwitchChannelCmd(state),
		newDiscordIdentityCmd(state),
		newDiscordSetIdentityCmd(state),
		newDiscordEventsCmd(state),
		newDiscordCreateEventCmd(state),
		newDiscordDeleteEventCmd(state),
		newDiscordMembersCmd(state),
		newDiscordRolesCmd(state),
		newDiscordAssignRoleCmd(state),
		newDiscordUnassignRoleCmd(state),
		newDiscordDmCmd(state),
	)
	return cmd
}

func newDiscordChannelsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "channels <account-id>",
		Short: "List the channels the bot can post to",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			channels, err := client.Accounts.ListDiscordChannels(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(channels, func() {
				rows := make([][]string, 0, len(channels))
				for _, c := range channels {
					rows = append(rows, []string{c.ID, "#" + c.Name, strconv.Itoa(c.Type), boolLabel(c.CanPost, "yes", "no"), boolLabel(c.IsCurrent, "yes", "no")})
				}
				printer.Table([]string{"id", "name", "type", "can post", "current"}, rows)
			})
		},
	}
}

func newDiscordSwitchChannelCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "switch-channel <account-id> <channel-id>",
		Short: "Move the account to another channel in the same server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			channel, err := client.Accounts.SwitchDiscordChannel(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(channel, func() {
				printer.Fields([][2]string{{"ID", channel.ID}, {"Name", "#" + channel.Name}})
			})
		},
	}
}

func printDiscordIdentity(state *State, identity *fopost.DiscordIdentity) error {
	printer := state.Printer()
	return printer.Value(identity, func() {
		printer.Fields([][2]string{
			{"Nickname", output.Dash(deref(identity.Username))},
			{"Avatar URL", output.Dash(deref(identity.AvatarURL))},
		})
	})
}

func newDiscordIdentityCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "identity <account-id>",
		Short: "Show the nickname and avatar the bot wears in the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			identity, err := client.Accounts.GetDiscordIdentity(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printDiscordIdentity(state, identity)
		},
	}
}

func newDiscordSetIdentityCmd(state *State) *cobra.Command {
	var username, avatarURL string
	var clearUsername, clearAvatar bool
	cmd := &cobra.Command{
		Use:   "set-identity <account-id>",
		Short: "Set the nickname and avatar the bot wears in the server",
		Long:  "Flags left out keep their value.",
		Example: strings.Join([]string{
			"  fopost accounts discord set-identity acc_1 --username \"Release Bot\"",
			"  fopost accounts discord set-identity acc_1 --clear-username --clear-avatar",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			if clearUsername && flags.Changed("username") {
				return usageErrorf("--username and --clear-username are alternatives; pass one")
			}
			if clearAvatar && flags.Changed("avatar-url") {
				return usageErrorf("--avatar-url and --clear-avatar are alternatives; pass one")
			}
			body := &fopost.UpdateDiscordIdentityRequest{}
			if flags.Changed("username") {
				if strings.TrimSpace(username) == "" {
					return usageErrorf("--username cannot be empty; use --clear-username")
				}
				body.Username = fopost.String(username)
			}
			if flags.Changed("avatar-url") {
				if strings.TrimSpace(avatarURL) == "" {
					return usageErrorf("--avatar-url cannot be empty; use --clear-avatar")
				}
				body.AvatarURL = fopost.String(avatarURL)
			}
			if clearUsername {
				body.Username = fopost.String("")
			}
			if clearAvatar {
				body.AvatarURL = fopost.String("")
			}
			if body.Username == nil && body.AvatarURL == nil {
				return usageErrorf("pass at least one of --username, --avatar-url, --clear-username, --clear-avatar")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			identity, err := client.Accounts.UpdateDiscordIdentity(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			return printDiscordIdentity(state, identity)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "the bot's nickname in the server (1-32 characters)")
	cmd.Flags().StringVar(&avatarURL, "avatar-url", "", "an http(s) image URL the bot wears")
	cmd.Flags().BoolVar(&clearUsername, "clear-username", false, "wear the application's own name again")
	cmd.Flags().BoolVar(&clearAvatar, "clear-avatar", false, "wear the application's own avatar again")
	return cmd
}

func newDiscordEventsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "events <account-id>",
		Short: "List the server's scheduled events",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			events, err := client.Accounts.ListDiscordEvents(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(events, func() {
				rows := make([][]string, 0, len(events))
				for _, e := range events {
					where := output.Dash(deref(e.Location))
					if e.ChannelID != nil {
						where = "#" + *e.ChannelID
					}
					rows = append(rows, []string{e.ID, e.Name, e.StartTime, where, e.Status})
				}
				printer.Table([]string{"id", "name", "starts", "where", "status"}, rows)
			})
		},
	}
}

func newDiscordCreateEventCmd(state *State) *cobra.Command {
	var name, description, startTime, endTime, channelID, location string
	cmd := &cobra.Command{
		Use:   "create-event <account-id>",
		Short: "Add an event to the server's calendar",
		Long:  "Give --channel-id for an event in a voice or stage channel, or --location with an\n--end-time for one somewhere else.",
		Example: strings.Join([]string{
			"  fopost accounts discord create-event acc_1 --name \"Launch stream\" \\",
			"    --start-time 2026-10-01T18:00:00Z --end-time 2026-10-01T19:00:00Z \\",
			"    --location https://yourbrand.com/live",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(startTime) == "" {
				return usageErrorf("--name and --start-time are required")
			}
			if channelID == "" && location == "" {
				return usageErrorf("pass --channel-id or --location")
			}
			if channelID == "" && endTime == "" {
				return usageErrorf("an event at a location needs --end-time")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			event, err := client.Accounts.CreateDiscordEvent(cmd.Context(), args[0], &fopost.DiscordEventRequest{
				Name:        name,
				Description: description,
				StartTime:   startTime,
				EndTime:     endTime,
				ChannelID:   channelID,
				Location:    location,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(event, func() {
				printer.Fields([][2]string{
					{"ID", event.ID},
					{"Name", event.Name},
					{"Starts", event.StartTime},
					{"Status", event.Status},
				})
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "the event name")
	cmd.Flags().StringVar(&description, "description", "", "what the event is about")
	cmd.Flags().StringVar(&startTime, "start-time", "", "when it starts, RFC 3339")
	cmd.Flags().StringVar(&endTime, "end-time", "", "when it ends, RFC 3339")
	cmd.Flags().StringVar(&channelID, "channel-id", "", "a voice or stage channel to hold it in")
	cmd.Flags().StringVar(&location, "location", "", "where it happens, when it is not in a channel")
	return cmd
}

func newDiscordDeleteEventCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-event <account-id> <event-id>",
		Short: "Remove a scheduled event",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Accounts.DeleteDiscordEvent(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]bool{"deleted": true}, func() {
				printer.Success("Event %s deleted.", args[1])
			})
		},
	}
}

func newDiscordMembersCmd(state *State) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{
		Use:   "members <account-id>",
		Short: "List or search the server's members",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			members, err := client.Accounts.ListDiscordMembers(cmd.Context(), args[0], &fopost.ListDiscordMembersOptions{
				Query: query,
				Limit: limit,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(members, func() {
				rows := make([][]string, 0, len(members))
				for _, m := range members {
					rows = append(rows, []string{m.ID, m.Username, output.Dash(deref(m.Nick)), boolLabel(m.IsBot, "yes", "no")})
				}
				printer.Table([]string{"id", "username", "nickname", "bot"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "search by username or nickname prefix")
	cmd.Flags().IntVar(&limit, "limit", 0, "how many members to return")
	return cmd
}

func newDiscordRolesCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "roles <account-id>",
		Short: "List the server's roles",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			roles, err := client.Accounts.ListDiscordRoles(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(roles, func() {
				rows := make([][]string, 0, len(roles))
				for _, r := range roles {
					rows = append(rows, []string{r.ID, r.Name, strconv.Itoa(r.Position), boolLabel(r.Managed, "yes", "no")})
				}
				printer.Table([]string{"id", "name", "position", "managed"}, rows)
			})
		},
	}
}

func newDiscordAssignRoleCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "assign-role <account-id> <role-id> <member-id>",
		Short: "Give a member a role",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Accounts.AddDiscordMemberRole(cmd.Context(), args[0], args[1], args[2]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]bool{"assigned": true}, func() {
				printer.Success("Member %s now holds role %s.", args[2], args[1])
			})
		},
	}
}

func newDiscordUnassignRoleCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "unassign-role <account-id> <role-id> <member-id>",
		Short: "Take a role from a member",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Accounts.RemoveDiscordMemberRole(cmd.Context(), args[0], args[1], args[2]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]bool{"assigned": false}, func() {
				printer.Success("Role %s taken from member %s.", args[1], args[2])
			})
		},
	}
}

func newDiscordDmCmd(state *State) *cobra.Command {
	var content string
	cmd := &cobra.Command{
		Use:   "dm <account-id> <member-id>",
		Short: "Send one message to a member of the server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(content) == "" {
				return usageErrorf("--content is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			sent, err := client.Accounts.SendDiscordDM(cmd.Context(), args[0], args[1], content)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(sent, func() {
				printer.Fields([][2]string{{"ID", sent.ID}, {"Channel", sent.ChannelID}})
			})
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "the message text")
	return cmd
}
