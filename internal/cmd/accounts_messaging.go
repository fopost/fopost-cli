package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAccountsMessagingCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messaging",
		Short: "Read and set the Meta messaging profile on a Facebook Page or Instagram account",
	}
	cmd.AddCommand(
		newIceBreakersCmd(state),
		newPersistentMenuCmd(state),
		newGreetingCmd(state),
	)
	return cmd
}

// ─── Ice breakers ────────────────────────────────────────────────

func printIceBreakers(state *State, result *fopost.MetaIceBreakers) error {
	printer := state.Printer()
	return printer.Value(result, func() {
		rows := make([][]string, 0, len(result.IceBreakers))
		for _, b := range result.IceBreakers {
			rows = append(rows, []string{b.Question, b.Payload})
		}
		printer.Table([]string{"question", "payload"}, rows)
	})
}

func newIceBreakersCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ice-breakers <account-id>",
		Short: "Show the prompts shown before the first message",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.GetIceBreakers(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printIceBreakers(state, result)
		},
	}
	cmd.AddCommand(newSetIceBreakersCmd(state), newClearIceBreakersCmd(state))
	return cmd
}

func newSetIceBreakersCmd(state *State) *cobra.Command {
	var prompts []string
	cmd := &cobra.Command{
		Use:   "set <account-id>",
		Short: "Replace the ice breakers, up to four",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			breakers := make([]fopost.MetaIceBreaker, 0, len(prompts))
			for _, raw := range prompts {
				question, payload, found := strings.Cut(raw, "=")
				if !found {
					return fmt.Errorf("--prompt %q must be \"question=PAYLOAD\"", raw)
				}
				breakers = append(breakers, fopost.MetaIceBreaker{Question: question, Payload: payload})
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.SetIceBreakers(cmd.Context(), args[0], breakers)
			if err != nil {
				return err
			}
			return printIceBreakers(state, result)
		},
	}
	cmd.Flags().StringArrayVar(&prompts, "prompt", nil, "An ice breaker as \"question=PAYLOAD\"; repeat for up to four")
	_ = cmd.MarkFlagRequired("prompt")
	return cmd
}

func newClearIceBreakersCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <account-id>",
		Short: "Clear the ice breakers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.DeleteIceBreakers(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printIceBreakers(state, result)
		},
	}
}

// ─── Persistent menu ─────────────────────────────────────────────

func printPersistentMenu(state *State, result *fopost.MetaPersistentMenu) error {
	printer := state.Printer()
	return printer.Value(result, func() {
		rows := make([][]string, 0)
		for _, entry := range result.PersistentMenu {
			for _, item := range entry.CallToActions {
				target := item.Payload
				if item.Type == "web_url" {
					target = item.URL
				}
				rows = append(rows, []string{entry.Locale, item.Type, item.Title, output.Dash(target)})
			}
		}
		printer.Table([]string{"locale", "type", "title", "target"}, rows)
	})
}

func newPersistentMenuCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "persistent-menu <account-id>",
		Short: "Show the always-visible Messenger menu (Facebook Pages only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.GetPersistentMenu(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printPersistentMenu(state, result)
		},
	}
	cmd.AddCommand(newSetPersistentMenuCmd(state), newClearPersistentMenuCmd(state))
	return cmd
}

func newSetPersistentMenuCmd(state *State) *cobra.Command {
	var postbacks, links []string
	var locale string
	cmd := &cobra.Command{
		Use:   "set <account-id>",
		Short: "Replace the menu, up to three items",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			items := make([]fopost.MetaMenuItem, 0, len(postbacks)+len(links))
			for _, raw := range postbacks {
				title, payload, found := strings.Cut(raw, "=")
				if !found {
					return fmt.Errorf("--postback %q must be \"Title=PAYLOAD\"", raw)
				}
				items = append(items, fopost.MetaMenuItem{Type: "postback", Title: title, Payload: payload})
			}
			for _, raw := range links {
				title, url, found := strings.Cut(raw, "=")
				if !found {
					return fmt.Errorf("--link %q must be \"Title=https://…\"", raw)
				}
				items = append(items, fopost.MetaMenuItem{Type: "web_url", Title: title, URL: url})
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			menu := []fopost.MetaPersistentMenuEntry{{Locale: locale, CallToActions: items}}
			result, err := client.Accounts.SetPersistentMenu(cmd.Context(), args[0], menu)
			if err != nil {
				return err
			}
			return printPersistentMenu(state, result)
		},
	}
	cmd.Flags().StringArrayVar(&postbacks, "postback", nil, "A menu item as \"Title=PAYLOAD\"")
	cmd.Flags().StringArrayVar(&links, "link", nil, "A menu item as \"Title=https://…\"")
	cmd.Flags().StringVar(&locale, "locale", "default", "The locale this menu applies to")
	return cmd
}

func newClearPersistentMenuCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <account-id>",
		Short: "Clear the menu",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.DeletePersistentMenu(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printPersistentMenu(state, result)
		},
	}
}

// ─── Greeting ────────────────────────────────────────────────────

func printGreeting(state *State, result *fopost.MetaGreeting) error {
	printer := state.Printer()
	return printer.Value(result, func() {
		rows := make([][]string, 0, len(result.Greeting))
		for _, g := range result.Greeting {
			rows = append(rows, []string{g.Locale, g.Text})
		}
		printer.Table([]string{"locale", "text"}, rows)
	})
}

func newGreetingCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "greeting <account-id>",
		Short: "Show the text shown before a Messenger conversation starts (Facebook Pages only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.GetGreeting(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printGreeting(state, result)
		},
	}
	cmd.AddCommand(newSetGreetingCmd(state), newClearGreetingCmd(state))
	return cmd
}

func newSetGreetingCmd(state *State) *cobra.Command {
	var text, locale string
	cmd := &cobra.Command{
		Use:   "set <account-id>",
		Short: "Replace the greeting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			greeting := []fopost.MetaGreetingText{{Locale: locale, Text: text}}
			result, err := client.Accounts.SetGreeting(cmd.Context(), args[0], greeting)
			if err != nil {
				return err
			}
			return printGreeting(state, result)
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "The greeting, up to 160 characters")
	cmd.Flags().StringVar(&locale, "locale", "default", "The locale this greeting applies to")
	_ = cmd.MarkFlagRequired("text")
	return cmd
}

func newClearGreetingCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <account-id>",
		Short: "Clear the greeting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.DeleteGreeting(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printGreeting(state, result)
		},
	}
}

// ─── Webhook subscription ────────────────────────────────────────

func printWebhookSubscription(state *State, result *fopost.WebhookSubscription) error {
	printer := state.Printer()
	return printer.Value(result, func() {
		printer.Table(
			[]string{"subscribed", "fields", "missing"},
			[][]string{{
				boolLabel(result.Subscribed, "yes", "no"),
				output.Dash(strings.Join(result.Fields, ", ")),
				output.Dash(strings.Join(result.MissingFields, ", ")),
			}},
		)
	})
}

func newAccountsWebhookCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook-subscription <account-id>",
		Short: "Show what the network is delivering to the FoPost webhook for an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.GetWebhookSubscription(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printWebhookSubscription(state, result)
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "resubscribe <account-id>",
		Short: "Subscribe to every field this account needs, lapsed or not",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.ResubscribeWebhook(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printWebhookSubscription(state, result)
		},
	})
	return cmd
}
