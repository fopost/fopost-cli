package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestTrackingConversionsSendCarriesTheIdempotencyKeyAndReadsStdin(t *testing.T) {
	isolate(t)
	var header string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "batch_1", "accepted": 1, "eventCount": 1, "replayed": false,
		}})
	})
	configured(t, api.URL)

	events := `[{"eventName":"Purchase","occurredAt":"2026-10-18T10:00:00Z","email":"a@b.c","valueMinor":1999,"currency":"USD"}]`
	stdout, stderr, code := run(t, events, "ads", "tracking", "conversions", "send",
		"--connection", "conn_1", "--ad-account", "act_1", "--pixel", "px_1",
		"--idempotency-key", "order-981", "--events", "-")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/ads/conversions/events")
	if header != "order-981" {
		t.Fatalf("Idempotency-Key = %q, want order-981", header)
	}
	if sent.Body["pixelId"] != "px_1" || sent.Body["workspaceId"] != "ws_1" {
		t.Fatalf("body = %v", sent.Body)
	}
	list, _ := sent.Body["events"].([]any)
	if len(list) != 1 {
		t.Fatalf("events = %v, want one", sent.Body["events"])
	}
	if event := list[0].(map[string]any); event["valueMinor"] != float64(1999) {
		t.Fatalf("event = %v", event)
	}
	if !strings.Contains(stdout, "batch_1") {
		t.Fatalf("send did not print the batch:\n%s", stdout)
	}
}

func TestTrackingConversionsSendRequiresTheKey(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the CLI reached the API without a key")
	})
	configured(t, api.URL)

	_, _, code := run(t, "[]", "ads", "tracking", "conversions", "send",
		"--connection", "conn_1", "--ad-account", "act_1", "--pixel", "px_1", "--events", "-")
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
}

func TestTrackingPixelsGetScopesTheReadAndPrintsTheCodeOnRequest(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "px_1", "name": "Site", "code": "<script>pixel</script>",
			"createdAt": "2026-10-01T00:00:00.000Z", "lastFiredAt": nil,
			"isUnavailable": false, "dataUseSetting": "advertising_and_analytics",
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "ads", "tracking", "pixels", "get", "px_1",
		"--connection", "conn_1", "--ad-account", "act_1", "--code")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "GET", "/ads/pixels/px_1")
	for key, want := range map[string]string{"workspace_id": "ws_1", "connection_id": "conn_1", "ad_account_id": "act_1"} {
		if got := sent.Query.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if !strings.Contains(stdout, "<script>pixel</script>") {
		t.Fatalf("get --code did not print the install script:\n%s", stdout)
	}
}

func TestTrackingCustomConversionsCreateSendsTheRule(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "cc_1", "name": "Signed up", "pixelId": "px_1", "eventType": "COMPLETE_REGISTRATION",
			"rule":        map[string]any{"eventName": "PageView", "urlContains": "/thanks"},
			"description": nil, "defaultValueMinor": 500, "isArchived": false, "isUnavailable": false,
			"createdAt": nil, "lastFiredAt": nil,
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "ads", "tracking", "custom-conversions", "create",
		"--connection", "conn_1", "--ad-account", "act_1", "--pixel", "px_1",
		"--name", "Signed up", "--event-type", "complete_registration",
		"--url-contains", "/thanks", "--default-value", "500")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/ads/custom-conversions")
	if sent.Body["eventType"] != "COMPLETE_REGISTRATION" || sent.Body["pixelId"] != "px_1" {
		t.Fatalf("body = %v", sent.Body)
	}
	rule, _ := sent.Body["rule"].(map[string]any)
	if rule["urlContains"] != "/thanks" {
		t.Fatalf("rule = %v", rule)
	}
	if sent.Body["defaultValueMinor"] != float64(500) {
		t.Fatalf("defaultValueMinor = %v", sent.Body["defaultValueMinor"])
	}
	if !strings.Contains(stdout, "cc_1") {
		t.Fatalf("create did not print the conversion:\n%s", stdout)
	}
}
