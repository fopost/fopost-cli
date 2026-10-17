package cmd

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/fopost/fopost-cli/internal/config"
)

// Business Profile management: one invocation per route, pinning the method,
// the path and the body the CLI actually sends.
func TestGBPCommandsHitTheirRoutes(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ok": true}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	attributes := filepath.Join(t.TempDir(), "attributes.json")
	if err := os.WriteFile(attributes, []byte(`[{"name":"attributes/has_wifi","values":[true]}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		args   []string
		method string
		path   string
	}{
		{"location", []string{"location", "acc_1"}, "GET", "/accounts/acc_1/gbp/location"},
		{"update-location", []string{"update-location", "acc_1", "--title", "Corner Bakery"},
			"PATCH", "/accounts/acc_1/gbp/location"},
		{"attributes", []string{"attributes", "acc_1"}, "GET", "/accounts/acc_1/gbp/attributes"},
		{"update-attributes", []string{"update-attributes", "acc_1", "--file", attributes},
			"PATCH", "/accounts/acc_1/gbp/attributes"},
		{"menus", []string{"menus", "acc_1"}, "GET", "/accounts/acc_1/gbp/menus"},
		{"services", []string{"services", "acc_1"}, "GET", "/accounts/acc_1/gbp/services"},
		{"media", []string{"media", "acc_1"}, "GET", "/accounts/acc_1/gbp/media"},
		{"add-media", []string{"add-media", "acc_1", "--media-id", "m_1", "--category", "INTERIOR"},
			"POST", "/accounts/acc_1/gbp/media"},
		{"delete-media", []string{"delete-media", "acc_1", "CAoSL"},
			"DELETE", "/accounts/acc_1/gbp/media/CAoSL"},
		{"place-actions", []string{"place-actions", "acc_1"},
			"GET", "/accounts/acc_1/gbp/place-actions"},
		{"add-place-action",
			[]string{"add-place-action", "acc_1", "--uri", "https://example.test/book", "--type", "APPOINTMENT"},
			"POST", "/accounts/acc_1/gbp/place-actions"},
		{"update-place-action", []string{"update-place-action", "acc_1", "links-1", "--preferred"},
			"PATCH", "/accounts/acc_1/gbp/place-actions/links-1"},
		{"delete-place-action", []string{"delete-place-action", "acc_1", "links-1"},
			"DELETE", "/accounts/acc_1/gbp/place-actions/links-1"},
		{"verification", []string{"verification", "acc_1"},
			"GET", "/accounts/acc_1/gbp/verification"},
		{"start-verification", []string{"start-verification", "acc_1", "--method", "sms"},
			"POST", "/accounts/acc_1/gbp/verification/start"},
		{"complete-verification", []string{"complete-verification", "acc_1", "v1", "--pin", "123456"},
			"POST", "/accounts/acc_1/gbp/verification/complete"},
		{"performance", []string{"performance", "acc_1", "--start", "2026-09-01", "--end", "2026-09-07"},
			"GET", "/accounts/acc_1/gbp/performance"},
		{"keywords", []string{"keywords", "acc_1", "--start", "2026-08-01", "--end", "2026-09-01"},
			"GET", "/accounts/acc_1/gbp/performance"},
		{"assign", []string{"assign", "acc_1", "--workspace", "ws_2"},
			"POST", "/accounts/acc_1/gbp/assign"},
	}

	for _, tc := range cases {
		args := append([]string{"accounts", "gbp"}, tc.args...)
		_, stderr, code := run(t, "", append(args, "--quiet")...)
		if code != ExitOK {
			t.Fatalf("%s: exit = %d: %s", tc.name, code, stderr)
		}
		last := api.Requests[len(api.Requests)-1]
		if last.Method != tc.method || last.Path != tc.path {
			t.Fatalf("%s sent %s %s, want %s %s", tc.name, last.Method, last.Path, tc.method, tc.path)
		}
	}
}

func TestGBPUpdateLocationSendsOnlyTheFlagsGiven(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	// An empty --description clears the field; the untouched flags stay out.
	_, stderr, code := run(t, "", "accounts", "gbp", "update-location", "acc_1",
		"--title", "Corner Bakery", "--description", "", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	body := api.Requests[len(api.Requests)-1].Body
	if len(body) != 2 || body["title"] != "Corner Bakery" || body["description"] != nil {
		t.Fatalf("body = %+v", body)
	}
}

func TestGBPAddMediaNamesTheLibraryAsset(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	if _, stderr, code := run(t, "", "accounts", "gbp", "add-media", "acc_1",
		"--media-id", "m_1", "--category", "MENU", "--quiet"); code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	body := api.Requests[len(api.Requests)-1].Body
	if body["media_id"] != "m_1" || body["category"] != "MENU" {
		t.Fatalf("body = %+v", body)
	}
}

func TestGBPPerformanceRepeatsTheMetricFlag(t *testing.T) {
	isolate(t)
	var query string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "", "accounts", "gbp", "performance", "acc_1",
		"--start", "2026-09-01", "--end", "2026-09-07",
		"--metric", "CALL_CLICKS", "--metric", "WEBSITE_CLICKS", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	// url.Values sorts the keys; what matters is that the metric repeats.
	parsed, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	metrics := parsed["daily_metrics"]
	if len(metrics) != 2 || metrics[0] != "CALL_CLICKS" || metrics[1] != "WEBSITE_CLICKS" {
		t.Fatalf("daily_metrics = %v", metrics)
	}
	if parsed.Get("start_date") != "2026-09-01" {
		t.Fatalf("start_date = %q", parsed.Get("start_date"))
	}
}

func TestGBPSurfacesAPendingAPIGrant(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "configuration_error", "message": "Not available yet",
		})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "", "accounts", "gbp", "location", "acc_1")
	if code == ExitOK {
		t.Fatal("expected a non-zero exit while the API grant is pending")
	}
	if stderr == "" {
		t.Fatal("expected the refusal to be explained on stderr")
	}
}
