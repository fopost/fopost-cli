package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func newAdsTrackingCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tracking",
		Short: "Pixels, the Conversions API and custom conversions on an ad account",
		Long: "Every command names the ad connection and the ad account the pixel sits on.\n" +
			"Needs the ads scope only.",
	}
	cmd.AddCommand(
		newTrackingPixelsCmd(state),
		newTrackingCustomConversionsCmd(state),
		newTrackingConversionsCmd(state),
	)
	return cmd
}

// trackingFlags are on every pixel-scoped command: the connection and the ad
// account that owns the pixel.
type trackingFlags struct {
	connection string
	adAccount  string
}

func (f *trackingFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.connection, "connection", "", "ad connection id (required)")
	cmd.Flags().StringVar(&f.adAccount, "ad-account", "", "ad account id (required)")
}

func (f *trackingFlags) params(state *State) (*fopost.PixelParams, error) {
	if f.connection == "" || f.adAccount == "" {
		return nil, usageErrorf("--connection and --ad-account are required")
	}
	resolved, err := state.Resolved()
	if err != nil {
		return nil, err
	}
	return &fopost.PixelParams{
		WorkspaceID:  resolved.Workspace,
		ConnectionID: f.connection,
		AdAccountID:  f.adAccount,
	}, nil
}

// ─── Pixels ────────────────────────────────────────────────────────

func newTrackingPixelsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pixels",
		Short: "List, read and create pixels, and read what they fired",
	}
	cmd.AddCommand(
		newPixelsListCmd(state),
		newPixelsGetCmd(state),
		newPixelsCreateCmd(state),
		newPixelsStatsCmd(state),
		newPixelsMatchQualityCmd(state),
	)
	return cmd
}

func newPixelsListCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the pixels on an ad account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			pixels, err := client.Ads.ListPixels(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(pixels, func() {
				rows := make([][]string, 0, len(pixels))
				for _, pixel := range pixels {
					rows = append(rows, []string{pixel.ID, output.Truncate(pixel.Name, 40)})
				}
				printer.Table([]string{"id", "name"}, rows)
			})
		},
	}
	flags.bind(cmd)
	return cmd
}

func printPixel(printer *output.Printer, pixel *fopost.AdPixelDetail, showCode bool) {
	printer.Fields([][2]string{
		{"ID", pixel.ID},
		{"Name", pixel.Name},
		{"Data Use", strOrDash(pixel.DataUseSetting)},
		{"Created", strOrDash(pixel.CreatedAt)},
		{"Last Fired", strOrDash(pixel.LastFiredAt)},
		{"Available", boolLabel(!pixel.IsUnavailable, "yes", "no")},
	})
	if showCode && pixel.Code != nil {
		printer.Line("%s", *pixel.Code)
	}
}

func newPixelsGetCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	var code bool
	cmd := &cobra.Command{
		Use:   "get <pixel-id>",
		Short: "Show one pixel; --code prints the install script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			pixel, err := client.Ads.GetPixel(cmd.Context(), args[0], params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(pixel, func() { printPixel(printer, pixel, code) })
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVar(&code, "code", false, "print the install script after the fields")
	return cmd
}

func newPixelsCreateCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Add a pixel to an ad account",
		Long:  "Prints the pixel with its install script.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return usageErrorf("--name is required")
			}
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			pixel, err := client.Ads.CreatePixel(cmd.Context(), &fopost.CreatePixelRequest{
				WorkspaceID:  params.WorkspaceID,
				ConnectionID: params.ConnectionID,
				AdAccountID:  params.AdAccountID,
				Name:         name,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(pixel, func() { printPixel(printer, pixel, true) })
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVar(&name, "name", "", "pixel name (required)")
	return cmd
}

func newPixelsStatsCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	cmd := &cobra.Command{
		Use:   "stats <pixel-id>",
		Short: "Show what a pixel fired over the last 28 days, by event",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			stats, err := client.Ads.GetPixelStats(cmd.Context(), args[0], params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(stats, func() {
				rows := make([][]string, 0, len(stats.Events))
				for _, event := range stats.Events {
					rows = append(rows, []string{event.EventName, itoa(event.Count)})
				}
				printer.Table([]string{"event", "count"}, rows)
				printer.Line("%s to %s", stats.Since, stats.Until)
			})
		},
	}
	flags.bind(cmd)
	return cmd
}

func newPixelsMatchQualityCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	cmd := &cobra.Command{
		Use:   "match-quality <pixel-id>",
		Short: "Show how well each pixel event's identity fields match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			quality, err := client.Ads.GetEventMatchQuality(cmd.Context(), args[0], params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(quality, func() {
				rows := make([][]string, 0, len(quality))
				for _, event := range quality {
					score := "-"
					if event.Score != nil {
						score = fmt.Sprintf("%.1f", *event.Score)
					}
					keys := make([]string, 0, len(event.MatchKeys))
					for _, key := range event.MatchKeys {
						keys = append(keys, fmt.Sprintf("%s %.0f%%", key.Identifier, key.CoveragePercent))
					}
					rows = append(rows, []string{event.EventName, score, output.Truncate(strings.Join(keys, ", "), 56)})
				}
				printer.Table([]string{"event", "score", "match keys"}, rows)
			})
		},
	}
	flags.bind(cmd)
	return cmd
}

// ─── Custom conversions ────────────────────────────────────────────

func newTrackingCustomConversionsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-conversions",
		Short: "List, create and delete the custom conversions on a pixel",
	}
	cmd.AddCommand(
		newCustomConversionsListCmd(state),
		newCustomConversionsCreateCmd(state),
		newCustomConversionsDeleteCmd(state),
	)
	return cmd
}

func customConversionRow(c fopost.CustomConversion) []string {
	rule := "-"
	if c.Rule != nil {
		switch {
		case c.Rule.URLContains != "":
			rule = "url contains " + c.Rule.URLContains
		case c.Rule.URLEquals != "":
			rule = "url equals " + c.Rule.URLEquals
		default:
			rule = output.Dash(c.Rule.EventName)
		}
	}
	return []string{
		c.ID,
		output.Truncate(c.Name, 32),
		c.EventType,
		strOrDash(c.PixelID),
		output.Truncate(rule, 36),
		boolLabel(c.IsArchived, "archived", "active"),
	}
}

func newCustomConversionsListCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the custom conversions on an ad account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			conversions, err := client.Ads.ListCustomConversions(cmd.Context(), params)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(conversions, func() {
				rows := make([][]string, 0, len(conversions))
				for _, c := range conversions {
					rows = append(rows, customConversionRow(c))
				}
				printer.Table([]string{"id", "name", "event type", "pixel", "rule", "state"}, rows)
			})
		},
	}
	flags.bind(cmd)
	return cmd
}

func newCustomConversionsCreateCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	var pixel, name, eventType, ruleEvent, urlContains, urlEquals, description string
	var defaultValue int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Add a custom conversion on a pixel",
		Example: strings.Join([]string{
			"  fopost ads tracking custom-conversions create --connection conn_1 --ad-account act_1 \\",
			"    --pixel 123 --name \"Signed up\" --event-type COMPLETE_REGISTRATION --url-contains /thanks",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if pixel == "" || name == "" || eventType == "" {
				return usageErrorf("--pixel, --name and --event-type are required")
			}
			if urlContains != "" && urlEquals != "" {
				return usageErrorf("--url-contains and --url-equals are alternatives; pass one")
			}
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			body := &fopost.CreateCustomConversionRequest{
				WorkspaceID:  params.WorkspaceID,
				ConnectionID: params.ConnectionID,
				AdAccountID:  params.AdAccountID,
				PixelID:      pixel,
				Name:         name,
				EventType:    strings.ToUpper(eventType),
				Rule: fopost.CustomConversionRule{
					EventName:   ruleEvent,
					URLContains: urlContains,
					URLEquals:   urlEquals,
				},
				Description: description,
			}
			if cmd.Flags().Changed("default-value") {
				body.DefaultValueMinor = &defaultValue
			}
			conversion, err := client.Ads.CreateCustomConversion(cmd.Context(), body)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(conversion, func() {
				printer.Table([]string{"id", "name", "event type", "pixel", "rule", "state"},
					[][]string{customConversionRow(*conversion)})
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVar(&pixel, "pixel", "", "the pixel the rule reads (required)")
	cmd.Flags().StringVar(&name, "name", "", "conversion name (required)")
	cmd.Flags().StringVar(&eventType, "event-type", "", "standard event it counts as, e.g. PURCHASE or LEAD (required)")
	cmd.Flags().StringVar(&ruleEvent, "rule-event", "", "pixel event the rule reads; PageView when empty")
	cmd.Flags().StringVar(&urlContains, "url-contains", "", "count when the page URL contains this")
	cmd.Flags().StringVar(&urlEquals, "url-equals", "", "count when the page URL equals this")
	cmd.Flags().StringVar(&description, "description", "", "what the conversion means")
	cmd.Flags().IntVar(&defaultValue, "default-value", 0, "default value in minor units of the account currency")
	return cmd
}

func newCustomConversionsDeleteCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <custom-conversion-id>",
		Short: "Remove a custom conversion from the network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete custom conversion %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			err = client.Ads.DeleteCustomConversion(cmd.Context(), args[0], &fopost.DeleteCustomConversionRequest{
				WorkspaceID:  params.WorkspaceID,
				ConnectionID: params.ConnectionID,
				AdAccountID:  params.AdAccountID,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted %s.", args[0])
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation")
	return cmd
}

// ─── Conversions ───────────────────────────────────────────────────

func newTrackingConversionsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversions",
		Short: "Send conversion events to a pixel exactly once, and list the batches sent",
	}
	cmd.AddCommand(
		newConversionsListCmd(state),
		newConversionsSendCmd(state),
	)
	return cmd
}

func newConversionsListCmd(state *State) *cobra.Command {
	var connection string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the conversion batches sent from this workspace, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit < 0 || limit > 100 {
				return usageErrorf("--limit must be between 1 and 100")
			}
			client, resolved, err := state.clientAndWorkspace()
			if err != nil {
				return err
			}
			batches, err := client.Ads.ListConversionEvents(cmd.Context(), &fopost.ListConversionEventsParams{
				WorkspaceID:  resolved,
				ConnectionID: connection,
				Limit:        limit,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(batches, func() {
				rows := make([][]string, 0, len(batches))
				for _, batch := range batches {
					rows = append(rows, []string{
						batch.ID,
						batch.CreatedAt,
						batch.PixelID,
						itoa(batch.EventCount),
						itoa(batch.Accepted),
						output.Truncate(strOrDash(batch.IdempotencyKey), 24),
					})
				}
				printer.Table([]string{"id", "sent", "pixel", "events", "accepted", "idempotency key"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&connection, "connection", "", "only batches sent through this ad connection")
	cmd.Flags().IntVar(&limit, "limit", 0, "batches to return, 1 to 100")
	return cmd
}

// readConversionEvents reads a JSON array of events from a path, or stdin for "-".
func readConversionEvents(state *State, path string) ([]fopost.ConversionEvent, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = readAll(state)
	} else {
		raw, err = os.ReadFile(path)
		if err != nil {
			err = fmt.Errorf("reading %s: %w", path, err)
		}
	}
	if err != nil {
		return nil, err
	}
	var events []fopost.ConversionEvent
	if err := json.Unmarshal(raw, &events); err != nil {
		return nil, usageErrorf("--events must be a JSON array of events: %v", err)
	}
	if len(events) == 0 {
		return nil, usageErrorf("--events holds no events")
	}
	return events, nil
}

func newConversionsSendCmd(state *State) *cobra.Command {
	flags := &trackingFlags{}
	var pixel, key, events string
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send conversion events to a pixel, exactly once per idempotency key",
		Long: "Reads a JSON array of events from --events (a path, or - for stdin). Each event carries\n" +
			"eventName and occurredAt, and may carry eventId, actionSource, sourceUrl, email, phone,\n" +
			"externalId, clientIpAddress, clientUserAgent, clickId, browserId, valueMinor, currency,\n" +
			"orderId and contentIds. Identity fields are hashed by the API and never stored.\n" +
			"A key sent before answers replayed and sends nothing.",
		Example: strings.Join([]string{
			"  fopost ads tracking conversions send --connection conn_1 --ad-account act_1 \\",
			"    --pixel 123 --idempotency-key order-981 --events events.json",
			"  cat events.json | fopost ads tracking conversions send --connection conn_1 \\",
			"    --ad-account act_1 --pixel 123 --idempotency-key order-981 --events -",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if pixel == "" || key == "" || events == "" {
				return usageErrorf("--pixel, --idempotency-key and --events are required")
			}
			params, err := flags.params(state)
			if err != nil {
				return err
			}
			list, err := readConversionEvents(state, events)
			if err != nil {
				return err
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Ads.SendConversions(cmd.Context(), key, &fopost.UploadConversionsRequest{
				WorkspaceID:  params.WorkspaceID,
				ConnectionID: params.ConnectionID,
				AdAccountID:  params.AdAccountID,
				PixelID:      pixel,
				Events:       list,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				if result.Replayed {
					printer.Warn("Replayed batch %s; nothing was sent.", result.ID)
					return
				}
				printer.Success("Sent batch %s: %d of %d events accepted.", result.ID, result.Accepted, result.EventCount)
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVar(&pixel, "pixel", "", "the pixel the events belong to (required)")
	cmd.Flags().StringVar(&key, "idempotency-key", "", "your own key for this batch; a repeat sends nothing (required)")
	cmd.Flags().StringVar(&events, "events", "", "path to a JSON array of events, or - for stdin (required)")
	return cmd
}
