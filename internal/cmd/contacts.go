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

func init() { register(newContactsCmd) }

func newContactsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contacts",
		Aliases: []string{"contact"},
		Short:   "List, inspect, import and delete the people behind the inbox",
	}
	cmd.AddCommand(
		newContactsListCmd(state),
		newContactsGetCmd(state),
		newContactsConversationsCmd(state),
		newContactsImportCmd(state),
		newContactsDeleteCmd(state),
		newContactsFieldsCmd(state),
	)
	return cmd
}

func newContactsListCmd(state *State) *cobra.Command {
	var search, platform, source string
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contacts, most recently active first",
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
			list, err := client.Contacts.List(cmd.Context(), &fopost.ListContactsParams{
				WorkspaceID: workspaceID,
				Search:      search,
				Platform:    platform,
				Source:      source,
				Page:        page,
				PerPage:     perPage,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(list, func() {
				rows := make([][]string, 0, len(list.Data))
				for _, contact := range list.Data {
					rows = append(rows, []string{
						contact.ID,
						output.Truncate(output.Dash(contact.DisplayName), 24),
						output.Truncate(handlesOf(contact), 32),
						contact.Source,
						contact.LastSeenAt.String(),
					})
				}
				printer.Table([]string{"id", "name", "handles", "source", "last seen"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "match a display name or any handle")
	cmd.Flags().StringVar(&platform, "platform", "", "only contacts with a handle on this network")
	cmd.Flags().StringVar(&source, "source", "", "inbox, radar, or import")
	cmd.Flags().IntVar(&page, "page", 0, "page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "results per page, up to 100")
	return cmd
}

// "instagram:@ada, x:@ada_writes" — one line, in the order the API returned.
func handlesOf(contact fopost.Contact) string {
	parts := make([]string, 0, len(contact.Channels))
	for _, channel := range contact.Channels {
		parts = append(parts, channel.Platform+":@"+channel.Handle)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func newContactsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <contact-id>",
		Short: "Show one contact, its handles and its custom fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			contact, err := client.Contacts.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(contact, func() {
				rows := [][]string{
					{"name", output.Dash(contact.DisplayName)},
					{"handles", handlesOf(*contact)},
					{"source", contact.Source},
					{"first seen", contact.FirstSeenAt.String()},
					{"last seen", contact.LastSeenAt.String()},
				}
				for key, value := range contact.Fields {
					rows = append(rows, []string{key, value})
				}
				printer.Table([]string{"field", "value"}, rows)
			})
		},
	}
}

func newContactsConversationsCmd(state *State) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "conversations <contact-id>",
		Short: "List the inbox threads one contact appears in",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			rows, err := client.Contacts.Conversations(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(rows, func() {
				table := make([][]string, 0, len(rows))
				for _, row := range rows {
					table = append(table, []string{
						row.Platform,
						output.Dash(row.AccountUsername),
						strconv.Itoa(row.Messages),
						strconv.Itoa(row.Received),
						strconv.Itoa(row.Sent),
						row.LastMessageAt.String(),
					})
				}
				printer.Table([]string{"platform", "account", "messages", "in", "out", "last"}, table)
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "how many threads to return, up to 100")
	return cmd
}

func newContactsImportCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file.csv>",
		Short: "Import contacts from a CSV",
		Long: "Import contacts from a CSV. `platform` and `handle` are required columns; " +
			"`external_id`, `display_name` and `note` are optional, and every other column " +
			"is read as a custom field key. A column matching no field is reported back " +
			"rather than stored.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			csv, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			result, err := client.Contacts.Import(cmd.Context(), workspaceID, string(csv))
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Success("Imported %d new and merged %d into a contact already on file.",
					result.Created, result.Merged)
				for _, skip := range result.Skipped {
					printer.Warn("Row %d skipped: %s.", skip.Row, skip.Reason)
				}
				if len(result.UnknownColumns) > 0 {
					printer.Warn("Columns with no field: %s.", strings.Join(result.UnknownColumns, ", "))
				}
			})
		},
	}
	return cmd
}

func newContactsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <contact-id>",
		Short: "Delete a contact; the messages stay in the inbox",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf("Delete contact %s? This cannot be undone.", args[0])
				if err := confirm(state, prompt); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Contacts.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted contact %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newContactsFieldsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fields",
		Short: "The columns this workspace keeps about a contact",
	}
	cmd.AddCommand(newContactsFieldsListCmd(state), newContactsFieldsDeleteCmd(state))
	return cmd
}

func newContactsFieldsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List custom fields, in display order",
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
			fields, err := client.Contacts.ListFields(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(fields, func() {
				rows := make([][]string, 0, len(fields))
				for _, field := range fields {
					rows = append(rows, []string{
						field.ID,
						field.Key,
						output.Truncate(field.Name, 24),
						field.Type,
						output.Dash(strings.Join(field.Options, ", ")),
					})
				}
				printer.Table([]string{"id", "key", "name", "type", "options"}, rows)
			})
		},
	}
}

func newContactsFieldsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <field-id>",
		Short: "Delete a custom field and every contact answer to it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				prompt := fmt.Sprintf(
					"Delete field %s? Every contact's answer to it goes with it. This cannot be undone.",
					args[0],
				)
				if err := confirm(state, prompt); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Contacts.DeleteField(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted field %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
