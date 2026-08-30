package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newWebhooksCmd) }

func newWebhooksCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "webhooks",
		Aliases: []string{"webhook"},
		Short:   "Manage outbound webhook subscriptions",
	}
	cmd.AddCommand(
		newWebhooksListCmd(state),
		newWebhooksCreateCmd(state),
		newWebhooksTestCmd(state),
		newWebhooksDeleteCmd(state),
	)
	return cmd
}

func newWebhooksListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List webhook subscriptions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			webhooks, err := client.Webhooks.List(cmd.Context())
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(webhooks, func() {
				rows := make([][]string, 0, len(webhooks))
				for _, webhook := range webhooks {
					rows = append(rows, []string{
						webhook.ID,
						output.Truncate(webhook.URL, 40),
						output.Truncate(strings.Join(webhook.Events, ","), 40),
						boolLabel(webhook.Active, "on", "off"),
						itoa(webhook.FailureCount),
					})
				}
				printer.Table([]string{"id", "url", "events", "state", "failures"}, rows)
			})
		},
	}
}

func newWebhooksCreateCmd(state *State) *cobra.Command {
	var (
		url    string
		events []string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Subscribe an endpoint to a workspace's events",
		Long: "Subscribes an endpoint. The signing secret is returned once, at creation, and\n" +
			"never again — store it now.\n\n" +
			"Events: post.published, post.failed, post.partially_failed, delivery.published,\n" +
			"delivery.failed, delivery.delayed, account.health_changed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if url == "" {
				return usageErrorf("--url is required")
			}
			if len(events) == 0 {
				return usageErrorf("at least one --event is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			webhook, err := client.Webhooks.Create(cmd.Context(), &fopost.CreateWebhookRequest{
				WorkspaceID: workspaceID,
				URL:         url,
				Events:      events,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(webhook, func() {
				printer.Success("Created webhook %s.", webhook.ID)
				printer.Fields([][2]string{
					{"URL", webhook.URL},
					{"Events", strings.Join(webhook.Events, ", ")},
					{"Secret", webhook.Secret},
				})
				printer.Warn("The secret is shown once. Store it now.")
			})
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "endpoint to deliver events to (required)")
	cmd.Flags().StringArrayVar(&events, "event", nil, "event to subscribe to (repeatable, at least one required)")
	return cmd
}

func newWebhooksTestCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "test <webhook-id>",
		Short: "Send a sample event to the subscribed endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Webhooks.Test(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				message := result.Message
				if message == "" {
					message = "Test event sent."
				}
				printer.Success("%s", message)
			})
		},
	}
}

func newWebhooksDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <webhook-id>",
		Short: "Remove a webhook subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete webhook %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Webhooks.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted webhook %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
