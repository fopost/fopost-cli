package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

// The platform owns a WhatsApp account's templates, flows, profile and commerce
// settings, so every command here is a live read or write. All of them answer
// 503 until WhatsApp is set up on the deployment.
func newAccountsWhatsappCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whatsapp",
		Short: "Read and write a WhatsApp Business number's templates, flows and profile",
	}
	cmd.AddCommand(
		newWhatsappProfileCmd(state),
		newWhatsappTemplatesCmd(state),
		newWhatsappFlowsCmd(state),
		newWhatsappGroupsCmd(state),
		newWhatsappBlockCmd(state),
		newWhatsappSandboxCmd(state),
	)
	return cmd
}

func newWhatsappProfileCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile <account-id>",
		Short: "Show the business profile and how the platform rates the number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			profile, err := client.WhatsApp.Profile(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(profile, func() {
				printer.Table([]string{"field", "value"}, [][]string{
					{"display name", output.Dash(deref(profile.DisplayName))},
					{"display name status", output.Dash(deref(profile.DisplayNameStatus))},
					{"username", output.Dash(deref(profile.Username))},
					{"about", output.Dash(deref(profile.About))},
					{"vertical", output.Dash(deref(profile.Vertical))},
					{"quality rating", output.Dash(deref(profile.QualityRating))},
					{"messaging limit", output.Dash(deref(profile.MessagingLimitTier))},
				})
			})
		},
	}
	return cmd
}

func newWhatsappTemplatesCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "List, file and remove message templates",
	}
	cmd.AddCommand(
		newWhatsappTemplatesListCmd(state),
		newWhatsappTemplatesCreateCmd(state),
		newWhatsappTemplatesDeleteCmd(state),
	)
	return cmd
}

func newWhatsappTemplatesListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list <account-id>",
		Short: "List templates with the review status the platform assigned",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			templates, err := client.WhatsApp.Templates(cmd.Context(), args[0], "")
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(templates, func() {
				rows := make([][]string, 0, len(templates))
				for _, t := range templates {
					rows = append(rows, []string{
						t.ID, t.Name, t.Language, t.Category, t.Status,
						output.Dash(deref(t.RejectedReason)),
					})
				}
				printer.Table([]string{"id", "name", "language", "category", "status", "reason"}, rows)
			})
		},
	}
}

func newWhatsappTemplatesCreateCmd(state *State) *cobra.Command {
	var name, language, category, body, componentsFile string
	cmd := &cobra.Command{
		Use:   "create <account-id>",
		Short: "File a template for review",
		Long: "Files a template for platform review and prints the status it was given, which is\n" +
			"PENDING on a normal submission. Pass --body for a simple text template, or\n" +
			"--components with a JSON file for anything richer.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			components, err := whatsappComponents(body, componentsFile)
			if err != nil {
				return err
			}
			template, err := client.WhatsApp.CreateTemplate(cmd.Context(), args[0], &fopost.CreateWhatsAppTemplateRequest{
				Name:       name,
				Language:   language,
				Category:   category,
				Components: components,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(template, func() {
				printer.Success("Filed %s for review: %s", template.Name, template.Status)
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "template name, lowercase with underscores (required)")
	cmd.Flags().StringVar(&language, "language", "en_US", "language code")
	cmd.Flags().StringVar(&category, "category", "UTILITY", "MARKETING, UTILITY or AUTHENTICATION")
	cmd.Flags().StringVar(&body, "body", "", "body text, for a simple text template")
	cmd.Flags().StringVar(&componentsFile, "components", "", "path to a JSON file of components")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// Either the shorthand --body or a components file, never both.
func whatsappComponents(body, file string) ([]map[string]any, error) {
	if file != "" {
		if body != "" {
			return nil, fmt.Errorf("pass --body or --components, not both")
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("could not read %s: %w", file, err)
		}
		var components []map[string]any
		if err := json.Unmarshal(raw, &components); err != nil {
			return nil, fmt.Errorf("%s is not a JSON array of components: %w", file, err)
		}
		return components, nil
	}
	if body == "" {
		return nil, fmt.Errorf("pass --body or --components")
	}
	return []map[string]any{{"type": "BODY", "text": body}}, nil
}

func newWhatsappTemplatesDeleteCmd(state *State) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "delete <account-id> <template-id>",
		Short: "Remove a template from the account",
		Long:  "The --name is required: it is what the platform deletes by.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.WhatsApp.DeleteTemplate(cmd.Context(), args[0], args[1], name); err != nil {
				return err
			}
			state.Printer().Success("Deleted %s", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "template name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newWhatsappFlowsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flows",
		Short: "List, publish and read the answers to in-chat forms",
	}
	cmd.AddCommand(
		newWhatsappFlowsListCmd(state),
		newWhatsappFlowsPublishCmd(state),
		newWhatsappFlowResponsesCmd(state),
	)
	return cmd
}

func newWhatsappFlowsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list <account-id>",
		Short: "List flows with their status and validation errors",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			flows, err := client.WhatsApp.Flows(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(flows, func() {
				rows := make([][]string, 0, len(flows))
				for _, f := range flows {
					problem := "-"
					if len(f.ValidationErrors) > 0 {
						problem = f.ValidationErrors[0].Message
					}
					rows = append(rows, []string{f.ID, f.Name, f.Status, problem})
				}
				printer.Table([]string{"id", "name", "status", "first problem"}, rows)
			})
		},
	}
}

func newWhatsappFlowsPublishCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "publish <account-id> <flow-id>",
		Short: "Make a draft flow sendable",
		Long:  "A published flow can no longer be deleted; it is deprecated instead.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			flow, err := client.WhatsApp.PublishFlow(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(flow, func() {
				printer.Success("%s is %s", flow.Name, flow.Status)
			})
		},
	}
}

func newWhatsappFlowResponsesCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "responses <account-id>",
		Short: "What people submitted through this account's flows",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			answers, err := client.WhatsApp.FlowResponses(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(answers, func() {
				rows := make([][]string, 0, len(answers))
				for _, a := range answers {
					fields := make([]string, 0, len(a.Answers))
					for key := range a.Answers {
						fields = append(fields, key)
					}
					rows = append(rows, []string{
						a.MessageID, output.Dash(deref(a.WaID)), fmt.Sprint(len(fields)),
					})
				}
				printer.Table([]string{"message", "from", "answers"}, rows)
			})
		},
	}
}

func newWhatsappGroupsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups <account-id>",
		Short: "List the groups this number created",
		Long:  "Participation is invite-only: no endpoint adds someone, so you send the invite link.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			groups, err := client.WhatsApp.Groups(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(groups, func() {
				rows := make([][]string, 0, len(groups))
				for _, g := range groups {
					count := "-"
					if g.ParticipantCount != nil {
						count = fmt.Sprint(*g.ParticipantCount)
					}
					rows = append(rows, []string{g.ID, g.Subject, count, output.Dash(deref(g.InviteLink))})
				}
				printer.Table([]string{"id", "subject", "people", "invite link"}, rows)
			})
		},
	}
	return cmd
}

func newWhatsappBlockCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block <account-id> <number>...",
		Short: "Block numbers from messaging this account",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.WhatsApp.BlockUsers(cmd.Context(), args[0], args[1:])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Success("Blocked %d, refused %d", len(result.Blocked), len(result.Failed))
			})
		},
	}
	return cmd
}

func newWhatsappSandboxCmd(state *State) *cobra.Command {
	var workspace, number string
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Invite a tester to the platform-owned WhatsApp test number",
		Long: "Sends an approved template from the test number to one tester; their reply opens the\n" +
			"messaging window. This is a send, so the key needs the publish scope. Only the last\n" +
			"four digits of the tester's number are ever stored.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			ws, err := state.WorkspaceID(workspace)
			if err != nil {
				return err
			}
			session, err := client.WhatsApp.CreateSandboxSession(cmd.Context(), ws, number)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(session, func() {
				printer.Success("Invited ****%s: %s", session.PhoneNumberLast4, session.Status)
			})
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace id")
	cmd.Flags().StringVar(&number, "number", "", "the tester's number in E.164, e.g. +15551234567 (required)")
	_ = cmd.MarkFlagRequired("number")
	return cmd
}
