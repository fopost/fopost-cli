package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestWebhookDeliveriesListGetRedeliver(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/webhooks/wh_1/deliveries":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id": "del_1", "webhook_id": "wh_1", "event": "post.published", "delivery_key": "key_1",
				"attempt": 2, "status": "failed", "response_status": 502, "response_snippet": "bad gateway",
				"duration_ms": 340, "payload_hash": "abc", "created_at": "2026-10-18T12:00:00Z",
			}}})
		case r.Method == http.MethodGet && r.URL.Path == "/webhooks/wh_1/deliveries/del_1":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "del_1", "webhook_id": "wh_1", "event": "post.published", "delivery_key": "key_1",
				"attempt": 2, "status": "failed", "response_status": nil, "response_snippet": "no answer",
				"duration_ms": 340, "payload_hash": "abc", "created_at": "2026-10-18T12:00:00Z",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/webhooks/wh_1/deliveries/del_1/redeliver":
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "webhooks", "deliveries", "list", "wh_1", "--status", "failed", "--limit", "10")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	listed := api.find(t, "GET", "/webhooks/wh_1/deliveries")
	if listed.Query.Get("status") != "failed" || listed.Query.Get("limit") != "10" {
		t.Fatalf("list sent query %v", listed.Query)
	}
	if !strings.Contains(stdout, "del_1") || !strings.Contains(stdout, "502") {
		t.Fatalf("list did not print the delivery:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "webhooks", "deliveries", "get", "wh_1", "del_1")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "GET", "/webhooks/wh_1/deliveries/del_1")
	if !strings.Contains(stdout, "key_1") || !strings.Contains(stdout, "no answer") {
		t.Fatalf("get did not print the delivery:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "webhooks", "deliveries", "redeliver", "wh_1", "del_1", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "POST", "/webhooks/wh_1/deliveries/del_1/redeliver")
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || result["redelivered"] != true || result["delivery_id"] != "del_1" {
		t.Fatalf("--json output is not the acknowledgement: %v\n%s", err, stdout)
	}
}
