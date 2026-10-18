package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAnalyticsInboxPrintsTheTotals(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"totals": map[string]any{"received": 40, "sent": 12, "answered": 10, "open": 5, "conversations": 8, "median_response_minutes": 95.4},
			"deltas": map[string]any{}, "series": []any{},
			"by_platform": []map[string]any{{"platform": "linkedin", "received": 40, "sent": 12}},
			"by_type":     []any{}, "by_account": []any{}, "response_time": []any{}, "heatmap": []any{},
			"time_zone": "UTC",
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "analytics", "inbox", "--days", "30", "--tz", "Europe/Zagreb")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "GET", "/analytics/inbox")
	if sent.Query.Get("workspace_id") != "ws_1" || sent.Query.Get("days") != "30" || sent.Query.Get("tz") != "Europe/Zagreb" {
		t.Fatalf("inbox sent query %v", sent.Query)
	}
	if !strings.Contains(stdout, "95 min") || !strings.Contains(stdout, "linkedin") {
		t.Fatalf("inbox did not print the totals:\n%s", stdout)
	}
}
