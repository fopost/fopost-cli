package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestConnectSessionsCreateAndGet(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/connect-sessions":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "cs_1", "status": "pending", "external_user_id": "user-42",
				"platforms": []string{"linkedin", "bluesky"},
				"url":       "https://connect.yourbrand.com/cs_1", "expires_at": "2026-10-18T13:00:00Z",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/connect-sessions/cs_1":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "cs_1", "status": "used", "external_user_id": "user-42",
				"platforms": []string{"linkedin"}, "account_id": "acc_7",
				"expires_at": "2026-10-18T13:00:00Z", "used_at": "2026-10-18T12:10:00Z",
			}})
		default:
			http.NotFound(w, r)
		}
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "connect-sessions", "create",
		"--external-user-id", "user-42", "--platform", "linkedin", "--platform", "bluesky",
		"--redirect-url", "https://yourbrand.com/done", "--branding-name", "Your Brand",
		"--branding-logo-url", "https://yourbrand.com/logo.png")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/connect-sessions")
	if sent.Body["workspaceId"] != "ws_1" || sent.Body["externalUserId"] != "user-42" || sent.Body["redirectUrl"] != "https://yourbrand.com/done" {
		t.Fatalf("create sent %v", sent.Body)
	}
	if platforms, _ := sent.Body["platforms"].([]any); len(platforms) != 2 || platforms[1] != "bluesky" {
		t.Fatalf("create sent platforms %v", sent.Body["platforms"])
	}
	branding, _ := sent.Body["branding"].(map[string]any)
	if branding["name"] != "Your Brand" || branding["logoUrl"] != "https://yourbrand.com/logo.png" {
		t.Fatalf("create sent branding %v", sent.Body["branding"])
	}
	if !strings.Contains(stdout, "https://connect.yourbrand.com/cs_1") {
		t.Fatalf("create did not print the link:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "connect-sessions", "get", "cs_1")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "GET", "/connect-sessions/cs_1")
	if !strings.Contains(stdout, "used") || !strings.Contains(stdout, "acc_7") {
		t.Fatalf("get did not print the status and account:\n%s", stdout)
	}
}
