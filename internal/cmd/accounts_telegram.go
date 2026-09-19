package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAccountsTelegramCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telegram",
		Short: "Connect Telegram chats and manage the bot's command menu",
	}
	cmd.AddCommand(
		newTelegramConnectCodeCmd(state),
		newTelegramConnectStatusCmd(state),
		newTelegramCommandsCmd(state),
	)
	return cmd
}

func newTelegramConnectCodeCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "connect-code",
		Short: "Mint a one-time code that connects a Telegram chat",
		Long: "Mints a code valid for 15 minutes. Send the printed command to the bot in a chat\n" +
			"to connect that chat, then check the outcome with connect-status.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			code, err := client.Accounts.CreateTelegramConnectCode(cmd.Context(), resolved.Workspace)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(code, func() {
				printer.Fields([][2]string{
					{"Code", code.Code},
					{"Command", code.Command},
					{"Bot", output.Dash(deref(code.BotUsername))},
					{"Private Chat", output.Dash(deref(code.DeepLink))},
					{"Group", output.Dash(deref(code.GroupLink))},
					{"Expires", output.Stamp(code.ExpiresAt.Time, code.ExpiresAt.Raw)},
				})
			})
		},
	}
}

func newTelegramConnectStatusCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "connect-status <code>",
		Short: "Show whether a connect code has connected a chat",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			status, err := client.Accounts.GetTelegramConnectStatus(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(status, func() {
				printer.Fields([][2]string{
					{"Status", status.Status},
					{"Account", output.Dash(deref(status.AccountID))},
					{"Reason", output.Dash(deref(status.Reason))},
				})
			})
		},
	}
}

func newTelegramCommandsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commands",
		Short: "Read or replace the command menu the bot shows in a connected chat",
	}
	cmd.AddCommand(
		newTelegramCommandsGetCmd(state),
		newTelegramCommandsSetCmd(state),
		newTelegramCommandsClearCmd(state),
	)
	return cmd
}

func printTelegramCommands(state *State, menu *fopost.TelegramBotCommands) error {
	printer := state.Printer()
	return printer.Value(menu, func() {
		rows := make([][]string, 0, len(menu.Commands))
		for _, c := range menu.Commands {
			rows = append(rows, []string{"/" + c.Command, c.Description})
		}
		printer.Table([]string{"command", "description"}, rows)
	})
}

func newTelegramCommandsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <account-id>",
		Short: "Show the bot's command menu for a chat",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			menu, err := client.Accounts.GetTelegramBotCommands(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printTelegramCommands(state, menu)
		},
	}
}

func newTelegramCommandsSetCmd(state *State) *cobra.Command {
	var entries []string
	cmd := &cobra.Command{
		Use:   "set <account-id>",
		Short: "Replace the bot's command menu for a chat",
		Example: strings.Join([]string{
			"  fopost accounts telegram commands set acc_1 --command \"start=Start here\" --command \"help=Get help\"",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(entries) == 0 {
				return usageErrorf("at least one --command is required")
			}
			commands := make([]fopost.TelegramBotCommand, 0, len(entries))
			for _, entry := range entries {
				name, description, ok := strings.Cut(entry, "=")
				name = strings.TrimPrefix(strings.TrimSpace(name), "/")
				description = strings.TrimSpace(description)
				if !ok || name == "" || description == "" {
					return usageErrorf("--command %q must look like name=description", entry)
				}
				commands = append(commands, fopost.TelegramBotCommand{Command: name, Description: description})
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			menu, err := client.Accounts.SetTelegramBotCommands(cmd.Context(), args[0], commands)
			if err != nil {
				return err
			}
			return printTelegramCommands(state, menu)
		},
	}
	cmd.Flags().StringArrayVar(&entries, "command", nil, "a menu entry as name=description (repeatable)")
	return cmd
}

func newTelegramCommandsClearCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "clear <account-id>",
		Short: "Remove the bot's command menu for a chat",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Clear the bot's commands for account %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			menu, err := client.Accounts.DeleteTelegramBotCommands(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(menu, func() {
				printer.Success("Cleared the bot's commands for account %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
