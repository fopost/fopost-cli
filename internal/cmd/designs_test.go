package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDesignsCreateExportAndDelete(t *testing.T) {
	isolate(t)
	design := map[string]any{
		"id": "des_1", "name": "Launch", "kind": "carousel", "width": 1080, "height": 1350,
		"created_at": "2026-11-20T10:00:00Z", "updated_at": "2026-11-20T10:00:00Z",
	}
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/designs":
			json.NewEncoder(w).Encode(map[string]any{"data": design})
		case r.Method == http.MethodGet && r.URL.Path == "/designs":
			json.NewEncoder(w).Encode(map[string]any{"data": []any{design}})
		case r.Method == http.MethodPost && r.URL.Path == "/designs/des_1/export":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"format": "png",
				"files":  []any{map[string]any{"media_id": "med_9", "page_index": 1, "mime_type": "image/png", "preview_url": "/v1/media/med_9/file"}},
			}})
		case r.Method == http.MethodDelete && r.URL.Path == "/designs/des_1":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "des_1", "deleted": true}})
		default:
			http.NotFound(w, r)
		}
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, `{"pages":[]}`, "designs", "create", "--kind", "carousel", "--name", "Launch", "--document-file", "-")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/designs")
	if sent.Body["workspace_id"] != "ws_1" || sent.Body["kind"] != "carousel" || sent.Body["document"] == nil {
		t.Fatalf("create sent %v", sent.Body)
	}

	stdout, _, code = run(t, "", "designs", "list", "--kind", "carousel", "--json")
	if code != ExitOK || !strings.Contains(stdout, `"des_1"`) {
		t.Fatalf("list --json = %d\n%s", code, stdout)
	}
	if listed := api.find(t, "GET", "/designs"); listed.Query.Get("kind") != "carousel" {
		t.Fatalf("list sent %v", listed.Query)
	}

	stdout, stderr, code = run(t, "", "designs", "export", "des_1", "--format", "png", "--page", "1")
	if code != ExitOK || !strings.Contains(stdout, "med_9") {
		t.Fatalf("export = %d\n%s%s", code, stdout, stderr)
	}
	exported := api.find(t, "POST", "/designs/des_1/export")
	if pages, _ := exported.Body["page_indexes"].([]any); len(pages) != 1 || pages[0] != float64(1) {
		t.Fatalf("export sent %v", exported.Body)
	}

	if _, _, code = run(t, "n\n", "designs", "delete", "des_1"); code != ExitUsage {
		t.Fatalf("declined delete exit = %d", code)
	}
	stdout, _, code = run(t, "", "designs", "delete", "des_1", "--yes", "--json")
	if code != ExitOK || !strings.Contains(stdout, `"deleted": true`) {
		t.Fatalf("delete = %d\n%s", code, stdout)
	}
}
