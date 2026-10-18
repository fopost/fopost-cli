package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAutomationVersionsListGetRestore(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/automations/auto_1/versions":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"version": 3, "note": "Added the delay", "created_by": map[string]any{"name": "Ops"},
					"created_at": "2026-10-18T12:00:00Z", "node_count": 4,
					"nodes_added": []map[string]any{{"id": "n4", "kind": "delay", "label": nil}}, "nodes_removed": []any{},
				}},
				"meta": map[string]any{"page": 1, "per_page": 20, "total": 3, "total_pages": 1},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/automations/auto_1/versions/3":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"version": 3, "note": nil, "created_by": nil, "created_at": "2026-10-18T12:00:00Z",
				"graph": map[string]any{
					"nodes": []map[string]any{{"id": "n1", "kind": "trigger", "label": "Start", "config": map[string]any{}, "position": 0}},
					"edges": []any{},
				},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/automations/auto_1/versions/3/restore":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"version": 4, "graph": map[string]any{"nodes": []any{}, "edges": []any{}},
				"steps": []map[string]any{{"position": 0, "node_id": "n1", "action_type": "send_email", "action_config": map[string]any{}}},
			}})
		default:
			http.NotFound(w, r)
		}
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "automations", "versions", "list", "auto_1", "--per-page", "20")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if got := api.find(t, "GET", "/automations/auto_1/versions").Query.Get("per_page"); got != "20" {
		t.Fatalf("list sent per_page=%q", got)
	}
	if !strings.Contains(stdout, "Added the delay") || !strings.Contains(stdout, "Ops") || !strings.Contains(stdout, "+1 -0") {
		t.Fatalf("list did not print the version:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "automations", "versions", "get", "auto_1", "3")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "GET", "/automations/auto_1/versions/3")
	if !strings.Contains(stdout, "trigger") || !strings.Contains(stdout, "Start") {
		t.Fatalf("get did not print the nodes:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "automations", "versions", "restore", "auto_1", "3")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "POST", "/automations/auto_1/versions/3/restore")
	if !strings.Contains(stdout, "version 4") {
		t.Fatalf("restore did not print the new version:\n%s", stdout)
	}

	if _, _, code := run(t, "", "automations", "versions", "get", "auto_1", "latest"); code != ExitUsage {
		t.Fatalf("a non-numeric version exited %d, want %d", code, ExitUsage)
	}
}
