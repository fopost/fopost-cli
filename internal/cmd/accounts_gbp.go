package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"
)

// Business Profile management for a connected Google location.
//
// Google grants Business Profile API access per project. Until that grant
// lands every command here fails with a 503 configuration_error.
//
// Responses relay Google's own shape, so the human view is the JSON itself
// rather than a table that would go stale the moment Google adds a field.
func newAccountsGBPCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gbp",
		Aliases: []string{"google-business"},
		Short:   "Manage a connected Google Business Profile location",
	}
	cmd.AddCommand(
		newGBPLocationCmd(state),
		newGBPUpdateLocationCmd(state),
		newGBPAttributesCmd(state),
		newGBPUpdateAttributesCmd(state),
		newGBPMenusCmd(state),
		newGBPReplaceMenusCmd(state),
		newGBPServicesCmd(state),
		newGBPReplaceServicesCmd(state),
		newGBPMediaCmd(state),
		newGBPAddMediaCmd(state),
		newGBPDeleteMediaCmd(state),
		newGBPPlaceActionsCmd(state),
		newGBPAddPlaceActionCmd(state),
		newGBPUpdatePlaceActionCmd(state),
		newGBPDeletePlaceActionCmd(state),
		newGBPVerificationCmd(state),
		newGBPStartVerificationCmd(state),
		newGBPCompleteVerificationCmd(state),
		newGBPPerformanceCmd(state),
		newGBPKeywordsCmd(state),
		newGBPAssignCmd(state),
	)
	return cmd
}

/* ── Location ──────────────────────────────────────────────────── */

func newGBPLocationCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "location <account-id>",
		Short: "Show the connected location",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.GetLocation(cmd.Context(), args[0])
		}),
	}
}

func newGBPUpdateLocationCmd(state *State) *cobra.Command {
	var title, description, website, phone, storeCode string
	cmd := &cobra.Command{
		Use:   "update-location <account-id>",
		Short: "Update the location's name, description, website, phone or store code",
		Long: "Updates the location. Only the flags you pass change; pass an empty string\n" +
			"to clear a field.",
		Args: cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			body := &fopost.UpdateGoogleBusinessLocationRequest{}
			if cmd.Flags().Changed("title") {
				body.Title = &title
			}
			if cmd.Flags().Changed("description") {
				body.Description = &description
			}
			if cmd.Flags().Changed("website") {
				body.WebsiteURI = &website
			}
			if cmd.Flags().Changed("phone") {
				body.PrimaryPhone = &phone
			}
			if cmd.Flags().Changed("store-code") {
				body.StoreCode = &storeCode
			}
			return c.GoogleBusiness.UpdateLocation(cmd.Context(), args[0], body)
		}),
	}
	cmd.Flags().StringVar(&title, "title", "", "Business name")
	cmd.Flags().StringVar(&description, "description", "", "Profile description")
	cmd.Flags().StringVar(&website, "website", "", "Website the listing links to")
	cmd.Flags().StringVar(&phone, "phone", "", "Phone number on the listing")
	cmd.Flags().StringVar(&storeCode, "store-code", "", "Your own code for this location")
	return cmd
}

/* ── Attributes, menus, services ───────────────────────────────── */

func newGBPAttributesCmd(state *State) *cobra.Command {
	var available bool
	var categoryName, regionCode, languageCode string
	cmd := &cobra.Command{
		Use:   "attributes <account-id>",
		Short: "Show the attributes set on the location",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.GetAttributes(cmd.Context(), args[0],
				&fopost.GoogleBusinessAttributesOptions{
					Available:    available,
					CategoryName: categoryName,
					RegionCode:   regionCode,
					LanguageCode: languageCode,
				})
		}),
	}
	cmd.Flags().BoolVar(&available, "available", false, "List what Google offers instead of what is set")
	cmd.Flags().StringVar(&categoryName, "category", "", "Category to list attributes for")
	cmd.Flags().StringVar(&regionCode, "region", "", "Region code to list attributes for")
	cmd.Flags().StringVar(&languageCode, "language", "", "Language for the attribute labels")
	return cmd
}

func newGBPUpdateAttributesCmd(state *State) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update-attributes <account-id>",
		Short: "Set attributes from a JSON file",
		Long: "Sets attributes from a JSON array, e.g.\n" +
			`  [{"name":"attributes/has_wifi","values":[true]}]` + "\n" +
			"Only the named attributes change; every other one is left alone.",
		Args: cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			attributes, err := readJSONList(file)
			if err != nil {
				return nil, err
			}
			return c.GoogleBusiness.UpdateAttributes(cmd.Context(), args[0], attributes)
		}),
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON file holding the attribute array, or - for stdin")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newGBPMenusCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "menus <account-id>",
		Short: "Show the location's food menus",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.GetMenus(cmd.Context(), args[0])
		}),
	}
}

func newGBPReplaceMenusCmd(state *State) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "replace-menus <account-id>",
		Short: "Replace the food menus from a JSON file",
		Long:  "Google has no per-section patch, so the whole menu set is replaced.",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			menus, err := readJSONList(file)
			if err != nil {
				return nil, err
			}
			return c.GoogleBusiness.ReplaceMenus(cmd.Context(), args[0], menus)
		}),
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON file holding the menu array, or - for stdin")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newGBPServicesCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "services <account-id>",
		Short: "Show the location's service list",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.GetServices(cmd.Context(), args[0])
		}),
	}
}

func newGBPReplaceServicesCmd(state *State) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "replace-services <account-id>",
		Short: "Replace the service list from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			items, err := readJSONList(file)
			if err != nil {
				return nil, err
			}
			return c.GoogleBusiness.ReplaceServices(cmd.Context(), args[0], items)
		}),
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON file holding the service array, or - for stdin")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

/* ── Photos ────────────────────────────────────────────────────── */

func newGBPMediaCmd(state *State) *cobra.Command {
	var pageSize int
	var pageToken string
	cmd := &cobra.Command{
		Use:   "media <account-id>",
		Short: "List the location's photos",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.ListMedia(cmd.Context(), args[0], pageSize, pageToken)
		}),
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Photos per page (1-100)")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Page token from a previous listing")
	return cmd
}

func newGBPAddMediaCmd(state *State) *cobra.Command {
	var mediaID, category, description string
	cmd := &cobra.Command{
		Use:   "add-media <account-id>",
		Short: "Add a photo from your media library",
		Long:  "The photo is a media-library asset in the same workspace, JPEG or PNG.",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.AddMedia(cmd.Context(), args[0], &fopost.AddGoogleBusinessMediaRequest{
				MediaID:     mediaID,
				Category:    category,
				Description: description,
			})
		}),
	}
	cmd.Flags().StringVar(&mediaID, "media-id", "", "Media library asset id")
	cmd.Flags().StringVar(&category, "category", "ADDITIONAL", "COVER, INTERIOR, PRODUCT, MENU and the rest")
	cmd.Flags().StringVar(&description, "description", "", "Caption Google shows with the photo")
	_ = cmd.MarkFlagRequired("media-id")
	return cmd
}

func newGBPDeleteMediaCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-media <account-id> <media-key>",
		Short: "Remove a photo from the location",
		Args:  cobra.ExactArgs(2),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.DeleteMedia(cmd.Context(), args[0], args[1])
		}),
	}
}

/* ── Place action links ────────────────────────────────────────── */

func newGBPPlaceActionsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "place-actions <account-id>",
		Short: "List the Book, Order and Reserve links on the listing",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.ListPlaceActions(cmd.Context(), args[0])
		}),
	}
}

func newGBPAddPlaceActionCmd(state *State) *cobra.Command {
	var uri, actionType string
	var preferred bool
	cmd := &cobra.Command{
		Use:   "add-place-action <account-id>",
		Short: "Add an action link to the listing",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			body := &fopost.CreateGoogleBusinessPlaceActionRequest{URI: uri, PlaceActionType: actionType}
			if cmd.Flags().Changed("preferred") {
				body.IsPreferred = &preferred
			}
			return c.GoogleBusiness.CreatePlaceAction(cmd.Context(), args[0], body)
		}),
	}
	cmd.Flags().StringVar(&uri, "uri", "", "Where the button sends the visitor")
	cmd.Flags().StringVar(&actionType, "type", "", "APPOINTMENT, FOOD_ORDERING, SHOP_ONLINE and the rest")
	cmd.Flags().BoolVar(&preferred, "preferred", false, "Prefer this link over the others of its type")
	_ = cmd.MarkFlagRequired("uri")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func newGBPUpdatePlaceActionCmd(state *State) *cobra.Command {
	var uri string
	var preferred bool
	cmd := &cobra.Command{
		Use:   "update-place-action <account-id> <link-id>",
		Short: "Change an action link's URL or preference",
		Args:  cobra.ExactArgs(2),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			body := &fopost.UpdateGoogleBusinessPlaceActionRequest{}
			if cmd.Flags().Changed("uri") {
				body.URI = &uri
			}
			if cmd.Flags().Changed("preferred") {
				body.IsPreferred = &preferred
			}
			return c.GoogleBusiness.UpdatePlaceAction(cmd.Context(), args[0], args[1], body)
		}),
	}
	cmd.Flags().StringVar(&uri, "uri", "", "Where the button sends the visitor")
	cmd.Flags().BoolVar(&preferred, "preferred", false, "Prefer this link over the others of its type")
	return cmd
}

func newGBPDeletePlaceActionCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-place-action <account-id> <link-id>",
		Short: "Remove an action link from the listing",
		Args:  cobra.ExactArgs(2),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.DeletePlaceAction(cmd.Context(), args[0], args[1])
		}),
	}
}

/* ── Verification ──────────────────────────────────────────────── */

func newGBPVerificationCmd(state *State) *cobra.Command {
	var languageCode string
	cmd := &cobra.Command{
		Use:   "verification <account-id>",
		Short: "List the ways Google will let this location be verified",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.GetVerificationOptions(cmd.Context(), args[0], languageCode)
		}),
	}
	cmd.Flags().StringVar(&languageCode, "language", "", "Language Google should verify in")
	return cmd
}

func newGBPStartVerificationCmd(state *State) *cobra.Command {
	var method, languageCode, phone, email, mailer string
	cmd := &cobra.Command{
		Use:   "start-verification <account-id>",
		Short: "Start verifying the location",
		Long:  "The response names the pending verification to finish with complete-verification.",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.StartVerification(cmd.Context(), args[0],
				&fopost.StartGoogleBusinessVerificationRequest{
					Method:            strings.ToUpper(method),
					LanguageCode:      languageCode,
					PhoneNumber:       phone,
					EmailAddress:      email,
					MailerContactName: mailer,
				})
		}),
	}
	cmd.Flags().StringVar(&method, "method", "", "ADDRESS, EMAIL, PHONE_CALL, SMS, AUTO or VETTED_PARTNER")
	cmd.Flags().StringVar(&languageCode, "language", "", "Language Google should verify in")
	cmd.Flags().StringVar(&phone, "phone", "", "Number to call or text")
	cmd.Flags().StringVar(&email, "email", "", "Address to mail")
	cmd.Flags().StringVar(&mailer, "mailer-contact", "", "Who the postcard is addressed to")
	_ = cmd.MarkFlagRequired("method")
	return cmd
}

func newGBPCompleteVerificationCmd(state *State) *cobra.Command {
	var pin string
	cmd := &cobra.Command{
		Use:   "complete-verification <account-id> <verification-name>",
		Short: "Finish a verification with the PIN Google sent",
		Args:  cobra.ExactArgs(2),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.CompleteVerification(cmd.Context(), args[0], args[1], pin)
		}),
	}
	cmd.Flags().StringVar(&pin, "pin", "", "The PIN Google sent")
	_ = cmd.MarkFlagRequired("pin")
	return cmd
}

/* ── Performance ───────────────────────────────────────────────── */

func newGBPPerformanceCmd(state *State) *cobra.Command {
	var start, end string
	var metrics []string
	cmd := &cobra.Command{
		Use:   "performance <account-id>",
		Short: "Show daily impressions, calls, directions and clicks",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.GetPerformance(cmd.Context(), args[0], start, end, metrics)
		}),
	}
	cmd.Flags().StringVar(&start, "start", "", "First day, as 2026-09-01")
	cmd.Flags().StringVar(&end, "end", "", "Last day, as 2026-09-30")
	cmd.Flags().StringSliceVar(&metrics, "metric", nil, "Metric to fetch; repeat for more")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")
	return cmd
}

func newGBPKeywordsCmd(state *State) *cobra.Command {
	var start, end, pageToken string
	cmd := &cobra.Command{
		Use:   "keywords <account-id>",
		Short: "Show the search terms people used to find the listing",
		Args:  cobra.ExactArgs(1),
		RunE: gbpRun(state, func(cmd *cobra.Command, c *fopost.Client, args []string) (any, error) {
			return c.GoogleBusiness.GetSearchKeywords(cmd.Context(), args[0], start, end, pageToken)
		}),
	}
	cmd.Flags().StringVar(&start, "start", "", "First month, as 2026-08-01")
	cmd.Flags().StringVar(&end, "end", "", "Last month, as 2026-09-01")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Page token from a previous listing")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")
	return cmd
}

/* ── Workspace assignment ──────────────────────────────────────── */

func newGBPAssignCmd(state *State) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "assign <account-id>",
		Short: "Hand the location to another workspace you own",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			moved, err := client.GoogleBusiness.Assign(cmd.Context(), args[0], workspace)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(moved, func() {
				printer.Table([]string{"id", "workspace"}, [][]string{{moved.ID, moved.WorkspaceID}})
			})
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace to move the location into")
	_ = cmd.MarkFlagRequired("workspace")
	return cmd
}

/* ── Shared plumbing ───────────────────────────────────────────── */

// gbpRun wires a Business Profile call to the printer. The payload is Google's
// own shape, so the human view is the JSON rather than a table.
func gbpRun(
	state *State,
	call func(*cobra.Command, *fopost.Client, []string) (any, error),
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		client, err := state.Client()
		if err != nil {
			return err
		}
		payload, err := call(cmd, client, args)
		if err != nil {
			return err
		}
		printer := state.Printer()
		return printer.Value(payload, func() {
			encoded, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return
			}
			printer.Line(string(encoded))
		})
	}
}

// readJSONList reads a JSON array of objects from a file, or stdin for "-".
func readJSONList(path string) ([]map[string]any, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("reading %s: expected a JSON array of objects: %w", path, err)
	}
	return out, nil
}
