package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fopost/fopost-cli/internal/config"
)

// run executes the CLI in-process against an isolated config directory.
func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	code = Execute(args, strings.NewReader(stdin), outBuf, errBuf)
	return outBuf.String(), errBuf.String(), code
}

// isolate points the config at a temp directory and clears the environment the
// CLI reads, so a developer's own key can never leak into a test.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.EnvConfigHome, dir)
	t.Setenv(config.EnvAPIKey, "")
	t.Setenv(config.EnvBaseURL, "")
	t.Setenv("NO_COLOR", "1")
	return dir
}

// fakeAPI is a stand-in for the FoPost API: enough of the surface for the
// flows under test, and a record of what the CLI actually sent.
type fakeAPI struct {
	*httptest.Server
	mu       chan struct{}
	Keys     []string
	Requests []recordedRequest
}

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

func newFakeAPI(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *fakeAPI {
	t.Helper()
	api := &fakeAPI{mu: make(chan struct{}, 1)}
	api.mu <- struct{}{}
	api.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := map[string]any{}
		if len(raw) > 0 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			_ = json.Unmarshal(raw, &body)
		}
		<-api.mu
		api.Keys = append(api.Keys, r.Header.Get("X-API-Key"))
		api.Requests = append(api.Requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		api.mu <- struct{}{}

		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(api.Close)
	return api
}

func (a *fakeAPI) paths() []string {
	<-a.mu
	defer func() { a.mu <- struct{}{} }()
	paths := make([]string, 0, len(a.Requests))
	for _, request := range a.Requests {
		paths = append(paths, request.Method+" "+request.Path)
	}
	return paths
}

func (a *fakeAPI) find(t *testing.T, method, path string) recordedRequest {
	t.Helper()
	<-a.mu
	defer func() { a.mu <- struct{}{} }()
	for _, request := range a.Requests {
		if request.Method == method && request.Path == path {
			return request
		}
	}
	t.Fatalf("the CLI never sent %s %s; it sent %v", method, path, a.Requests)
	return recordedRequest{}
}

func TestAuthLoginStoresTheKeyWithoutPrintingIt(t *testing.T) {
	dir := isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "ws_1", "name": "Studio"}}})
	})

	const key = "fp_live_9f2c4a7b1ee3d"
	stdout, stderr, code := run(t, key+"\n", "auth", "login", "--base-url", api.URL)
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, key) {
		t.Fatalf("login printed the API key:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout, config.Mask(key)) {
		t.Fatalf("login did not show the masked key:\n%s", stdout)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "fopost", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	saved := &config.File{}
	if err := json.Unmarshal(raw, saved); err != nil {
		t.Fatal(err)
	}
	if saved.APIKey != key {
		t.Fatalf("saved key = %q, want the one given", saved.APIKey)
	}
	// A lone workspace becomes the default, so later commands need no flag.
	if saved.Workspace != "ws_1" {
		t.Fatalf("saved workspace = %q, want ws_1", saved.Workspace)
	}
	if api.Keys[0] != key {
		t.Fatalf("the API saw key %q", api.Keys[0])
	}
}

func TestAuthStatusMasksTheKeyAndNamesItsSource(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "ws_1"}}})
	})

	const key = "fp_live_abcdefghijkl"
	t.Setenv(config.EnvAPIKey, key)

	stdout, stderr, code := run(t, "", "auth", "status", "--base-url", api.URL, "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	result := statusResult{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("status --json is not valid JSON: %v\n%s", err, stdout)
	}
	if result.Key == key || strings.Contains(stdout, key) {
		t.Fatalf("status leaked the key:\n%s", stdout)
	}
	if result.Key != config.Mask(key) {
		t.Fatalf("Key = %q, want the mask", result.Key)
	}
	if result.Source != string(config.SourceEnv) {
		t.Fatalf("Source = %q, want %q", result.Source, config.SourceEnv)
	}
	if !result.Authenticated {
		t.Fatal("Authenticated = false with a key set")
	}
}

func TestAuthLogoutRemovesTheKey(t *testing.T) {
	dir := isolate(t)
	if err := config.Save(&config.File{APIKey: "fp_key", Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}
	if _, _, code := run(t, "", "auth", "logout"); code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "fopost", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	saved := &config.File{}
	json.Unmarshal(raw, saved)
	if saved.APIKey != "" {
		t.Fatalf("the key survived logout: %q", saved.APIKey)
	}
	if saved.Workspace != "ws_1" {
		t.Fatal("logout cleared the workspace without --all")
	}
}

func TestMissingKeyIsAUsageError(t *testing.T) {
	isolate(t)
	_, stderr, code := run(t, "", "workspaces", "list")
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "auth login") {
		t.Fatalf("stderr = %q, want the login hint", stderr)
	}
}

func TestFlagKeyBeatsTheEnvironmentAndTheFile(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	})
	if err := config.Save(&config.File{APIKey: "fp_from_file", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvAPIKey, "fp_from_env")

	if _, stderr, code := run(t, "", "workspaces", "list", "--api-key", "fp_from_flag"); code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if api.Keys[0] != "fp_from_flag" {
		t.Fatalf("the API saw %q, want the flag's key", api.Keys[0])
	}
}

func TestListJSONIsTheDecodedResourceNotATable(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "ws_1", "name": "Studio", "slug": "studio", "type": "TEAM"},
		}})
	})

	stdout, stderr, code := run(t, "", "workspaces", "list", "--api-key", "fp_k", "--base-url", api.URL, "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var workspaces []map[string]any
	if err := json.Unmarshal([]byte(stdout), &workspaces); err != nil {
		t.Fatalf("--json output is not a JSON array: %v\n%s", err, stdout)
	}
	if len(workspaces) != 1 || workspaces[0]["id"] != "ws_1" || workspaces[0]["name"] != "Studio" {
		t.Fatalf("decoded = %v, want the workspace the API sent", workspaces)
	}

	stdout, _, code = run(t, "", "workspaces", "list", "--api-key", "fp_k", "--base-url", api.URL)
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "ws_1") {
		t.Fatalf("the table view is missing its header or row:\n%s", stdout)
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Fatalf("the default view printed JSON:\n%s", stdout)
	}
}

func TestAPIErrorsSurfaceAsOneLineAndAnExitCode(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]any{
			"error":       "subscription_required",
			"message":     "Your plan does not include the API.",
			"upgrade_url": "https://fopost.com/billing",
		})
	})

	stdout, stderr, code := run(t, "", "workspaces", "list", "--api-key", "fp_k", "--base-url", api.URL)
	if code != ExitPaymentRequired {
		t.Fatalf("exit = %d, want %d", code, ExitPaymentRequired)
	}
	if stdout != "" {
		t.Fatalf("a failure wrote to stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "Your plan does not include the API.") {
		t.Fatalf("stderr = %q, want the API's message", stderr)
	}
	if !strings.Contains(stderr, "https://fopost.com/billing") {
		t.Fatalf("stderr = %q, want the upgrade URL", stderr)
	}
}

func TestPostsCreatePublishUploadsSchedulesAndPublishesInOneRun(t *testing.T) {
	isolate(t)

	mediaFile := filepath.Join(t.TempDir(), "card.png")
	if err := os.WriteFile(mediaFile, []byte("\x89PNG\r\n\x1a\n fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/media/upload":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "med_1", "type": "image", "name": "card.png", "url": "https://cdn.example/card.png", "size": 11},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/posts":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "post_1", "workspace_id": "ws_1", "status": "draft",
				"content": []map[string]any{{"text": "Shipping today"}},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/posts/post_1/publish":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"post_status": "publishing",
				"deliveries": []map[string]any{
					{"id": "del_1", "accountId": "acc_1", "status": "queued"},
				},
			}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not_found","message":"no"}`))
		}
	})

	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "",
		"posts", "create",
		"--account", "acc_1",
		"--text", "Shipping today",
		"--media", mediaFile,
		"--publish",
		"--json",
	)
	if code != ExitOK {
		t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	want := []string{"POST /media/upload", "POST /posts", "POST /posts/post_1/publish"}
	got := api.paths()
	if len(got) != len(want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	for index, path := range want {
		if got[index] != path {
			t.Fatalf("request %d = %q, want %q (full order: %v)", index, got[index], path, got)
		}
	}

	created := api.find(t, http.MethodPost, "/posts")
	if created.Body["workspace_id"] != "ws_1" {
		t.Fatalf("workspace_id = %v, want the saved default", created.Body["workspace_id"])
	}
	if created.Body["status"] != "draft" {
		t.Fatalf("status = %v; --publish creates a draft, then publishes it", created.Body["status"])
	}
	accounts, _ := created.Body["accounts"].([]any)
	if len(accounts) != 1 || accounts[0] != "acc_1" {
		t.Fatalf("accounts = %v, want [acc_1]", created.Body["accounts"])
	}
	content, _ := created.Body["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content = %v, want one block", created.Body["content"])
	}
	block, _ := content[0].(map[string]any)
	if block["text"] != "Shipping today" {
		t.Fatalf("text = %v", block["text"])
	}
	media, _ := block["media"].([]any)
	if len(media) != 1 {
		t.Fatalf("the uploaded file was not attached: %v", block["media"])
	}
	if attached, _ := media[0].(map[string]any); attached["url"] != "https://cdn.example/card.png" {
		t.Fatalf("attached media = %v, want the uploaded URL", media[0])
	}

	result := struct {
		Post    map[string]any `json:"post"`
		Media   []any          `json:"media"`
		Publish map[string]any `json:"publish"`
	}{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, stdout)
	}
	if result.Post["id"] != "post_1" {
		t.Fatalf("post id = %v", result.Post["id"])
	}
	if len(result.Media) != 1 {
		t.Fatalf("media = %v, want the uploaded asset", result.Media)
	}
	if result.Publish == nil || result.Publish["post_status"] != "publishing" {
		t.Fatalf("publish = %v, want the publish result", result.Publish)
	}
}

func TestPostsCreateRejectsContradictoryFlagsBeforeCallingTheAPI(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the CLI called %s %s despite a usage error", r.Method, r.URL.Path)
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	cases := [][]string{
		{"posts", "create", "--text", "hi"},
		{"posts", "create", "--account", "acc_1"},
		{"posts", "create", "--account", "acc_1", "--text", "hi", "--publish", "--schedule-at", "2026-09-01T09:00:00Z"},
		{"posts", "create", "--account", "acc_1", "--text", "hi", "--publish", "--draft"},
		{"posts", "create", "--account", "acc_1", "--text", "hi", "--schedule-at", "next tuesday"},
	}
	for _, args := range cases {
		_, stderr, code := run(t, "", args...)
		if code != ExitUsage {
			t.Errorf("%v: exit = %d, want %d (%s)", args, code, ExitUsage, stderr)
		}
	}
}

func TestPostsCreateReadsTheTextFromStdin(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "post_2", "status": "scheduled"}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "from stdin\n",
		"posts", "create", "--account", "acc_1", "--text-file", "-",
		"--schedule-at", "2026-09-01T09:00:00Z", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	created := api.find(t, http.MethodPost, "/posts")
	if created.Body["status"] != "scheduled" {
		t.Fatalf("status = %v, want scheduled", created.Body["status"])
	}
	if created.Body["schedule_at"] != "2026-09-01T09:00:00Z" {
		t.Fatalf("schedule_at = %v", created.Body["schedule_at"])
	}
	content, _ := created.Body["content"].([]any)
	block, _ := content[0].(map[string]any)
	if block["text"] != "from stdin" {
		t.Fatalf("text = %v, want the piped text", block["text"])
	}
}

func TestQuietSuppressesOutputButKeepsTheExitCode(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "ws_1", "name": "Studio"}}})
	})
	stdout, _, code := run(t, "", "workspaces", "list", "--api-key", "fp_k", "--base-url", api.URL, "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if stdout != "" {
		t.Fatalf("--quiet printed %q", stdout)
	}
}

func TestVersionReportsTheCLIAndSDK(t *testing.T) {
	isolate(t)
	stdout, _, code := run(t, "", "version", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	info := versionInfo{}
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("version --json is not valid JSON: %v\n%s", err, stdout)
	}
	if info.Version == "" || info.SDK == "" {
		t.Fatalf("version = %+v, want both the CLI and SDK versions", info)
	}
}

func TestMediaUploadDirectPresignsPutsAndCompletes(t *testing.T) {
	isolate(t)

	mediaFile := filepath.Join(t.TempDir(), "card.png")
	if err := os.WriteFile(mediaFile, []byte("\x89PNG\r\n\x1a\n fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	var api *fakeAPI
	var putKey, putType string
	var putLength int64
	api = newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/media/presign":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"uploadId":  "up_1",
				"uploadUrl": api.URL + "/blob/up_1",
				"method":    "PUT",
				"headers":   map[string]string{"Content-Type": "image/png"},
				"expiresAt": "2026-09-19T12:00:00Z",
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/blob/up_1":
			putKey, putType, putLength = r.Header.Get("X-API-Key"), r.Header.Get("Content-Type"), r.ContentLength
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/media/presign/up_1/complete":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "med_1", "type": "image", "name": "card.png", "url": "https://cdn.example/card.png", "size": 13,
			}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not_found","message":"no"}`))
		}
	})

	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "media", "upload", "--direct", mediaFile, "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	want := []string{"POST /media/presign", "PUT /blob/up_1", "POST /media/presign/up_1/complete"}
	got := api.paths()
	if len(got) != len(want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	for index, path := range want {
		if got[index] != path {
			t.Fatalf("request %d = %q, want %q (full order: %v)", index, got[index], path, got)
		}
	}

	presign := api.find(t, http.MethodPost, "/media/presign")
	if presign.Body["workspaceId"] != "ws_1" || presign.Body["filename"] != "card.png" {
		t.Fatalf("presign body = %v", presign.Body)
	}
	if presign.Body["mimeType"] != "image/png" || presign.Body["size"] != float64(13) {
		t.Fatalf("presign body = %v, want the detected type and exact size", presign.Body)
	}
	if putKey != "" {
		t.Fatalf("the PUT carried the API key")
	}
	if putType != "image/png" || putLength != 13 {
		t.Fatalf("PUT type = %q length = %d, want image/png and the file's 13 bytes", putType, putLength)
	}

	var result []map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, stdout)
	}
	if len(result) != 1 || result[0]["id"] != "med_1" {
		t.Fatalf("result = %v, want the completed asset", result)
	}
}

func TestPostsCreateWithAGroupSendsTheGroupAndNoAccounts(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "post_3", "status": "draft"}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "", "posts", "create", "--group", "grp_1", "--text", "hi", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	created := api.find(t, http.MethodPost, "/posts")
	if created.Body["account_group_id"] != "grp_1" {
		t.Fatalf("account_group_id = %v", created.Body["account_group_id"])
	}
	if _, sent := created.Body["accounts"]; sent {
		t.Fatalf("accounts = %v, want it omitted", created.Body["accounts"])
	}
}

func TestAccountsListPassesTheGroupFilter(t *testing.T) {
	isolate(t)
	var query string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "", "accounts", "list", "--group", "grp_1", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if query != "group_id=grp_1" {
		t.Fatalf("query = %q", query)
	}
}

func TestAccountGroupsSetMembersNeedsAccountsOrClear(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "grp_1", "account_ids": []any{}}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	if _, _, code := run(t, "", "account-groups", "set-members", "grp_1"); code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if len(api.paths()) != 0 {
		t.Fatalf("the CLI called %v despite a usage error", api.paths())
	}

	_, stderr, code := run(t, "", "account-groups", "set-members", "grp_1", "--clear", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	sent := api.find(t, http.MethodPut, "/account-groups/grp_1/members")
	if members, ok := sent.Body["account_ids"].([]any); !ok || len(members) != 0 {
		t.Fatalf("account_ids = %v, want an empty list", sent.Body["account_ids"])
	}
}

func TestAccountsMoveBlockedExitsWithAnError(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]any{"error": "move_blocked", "message": "Account has records in its workspace", "blocking_tables": []string{"posts"}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := run(t, "", "accounts", "move", "acc_1", "--to", "ws_2")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing on failure", stdout)
	}
	if moved := api.find(t, http.MethodPost, "/accounts/acc_1/move"); moved.Body["workspace_id"] != "ws_2" {
		t.Fatalf("workspace_id = %v", moved.Body["workspace_id"])
	}
}

func TestAdsTreeSendsTheConnectionAndPrintsEveryLevel(t *testing.T) {
	isolate(t)
	var query string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"adAccountId": "act_1",
			"campaigns": []any{map[string]any{
				"id": "c_1", "name": "Launch", "status": "ACTIVE", "budgetMinor": nil,
				"adSets": []any{map[string]any{
					"id": "s_1", "name": "US", "status": "ACTIVE", "budgetMinor": 5000,
					"ads": []any{map[string]any{"id": "a_1", "name": "Hero", "status": "PAUSED"}},
				}},
			}},
		}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	if _, _, code := run(t, "", "ads", "tree", "act_1"); code != ExitUsage {
		t.Fatalf("exit = %d without --connection, want %d", code, ExitUsage)
	}
	stdout, stderr, code := run(t, "", "ads", "tree", "act_1", "--connection", "conn_1")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	api.find(t, http.MethodGet, "/ads/accounts/act_1/tree")
	if query != "connection_id=conn_1&workspace_id=ws_1" {
		t.Fatalf("query = %q", query)
	}
	for _, want := range []string{"c_1", "s_1", "a_1", "50.00"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("tree output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestAdsPauseSendsEveryObjectWithItsLevel(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{
			map[string]any{"id": "c_1", "level": "campaign", "ok": true},
			map[string]any{"id": "a_1", "level": "ad", "ok": true},
		}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	if _, _, code := run(t, "", "ads", "pause", "--connection", "conn_1"); code != ExitUsage {
		t.Fatalf("exit = %d with no objects, want %d", code, ExitUsage)
	}
	if len(api.paths()) != 0 {
		t.Fatalf("the CLI called %v despite a usage error", api.paths())
	}

	_, stderr, code := run(t, "", "ads", "pause", "--connection", "conn_1", "--campaign", "c_1", "--ad", "a_1", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	sent := api.find(t, http.MethodPost, "/ads/status")
	objects, _ := sent.Body["objects"].([]any)
	if sent.Body["status"] != "paused" || sent.Body["workspaceId"] != "ws_1" || len(objects) != 2 {
		t.Fatalf("body = %v", sent.Body)
	}
	if first, _ := objects[0].(map[string]any); first["level"] != "campaign" {
		t.Fatalf("objects = %v", objects)
	}
}

func TestAdsInsightsPassesTheRangeAndBreakdown(t *testing.T) {
	isolate(t)
	var query string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"objectId": "c_1", "totals": map[string]any{"impressions": 10}}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	if _, _, code := run(t, "", "ads", "insights", "c_1", "--connection", "conn_1", "--since", "2026-09-01", "--until", "2026-09-07", "--breakdown", "weekday"); code != ExitUsage {
		t.Fatalf("exit = %d for an unknown breakdown, want %d", code, ExitUsage)
	}
	_, stderr, code := run(t, "", "ads", "insights", "c_1", "--connection", "conn_1", "--since", "2026-09-01", "--until", "2026-09-07", "--breakdown", "age", "--daily", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if query != "breakdown=age&connection_id=conn_1&daily=true&object_id=c_1&since=2026-09-01&until=2026-09-07" {
		t.Fatalf("query = %q", query)
	}
}

func TestAdsLeadsPassesTheCursorAndPrintsTheNextOne(t *testing.T) {
	isolate(t)
	var query string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"leads":      []any{map[string]any{"id": "l_1", "fields": []any{map[string]any{"name": "full_name", "values": []string{"Morgan Lee"}}}}},
			"nextCursor": "cur_2",
		}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "ads", "leads", "--cursor", "cur_1", "--limit", "25")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if query != "cursor=cur_1&limit=25" {
		t.Fatalf("query = %q", query)
	}
	if !strings.Contains(stdout, "cur_2") {
		t.Fatalf("output is missing the next cursor:\n%s", stdout)
	}
}

func TestAccountsTelegramConnectCodeSendsTheWorkspace(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"code": "abc123", "command": "/connect abc123", "expires_at": "2026-09-19T12:15:00Z"}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "accounts", "telegram", "connect-code", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if sent := api.find(t, http.MethodPost, "/accounts/telegram/connect-code"); sent.Body["workspaceId"] != "ws_1" {
		t.Fatalf("workspaceId = %v", sent.Body["workspaceId"])
	}
	if !strings.Contains(stdout, `"code": "abc123"`) {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestAccountsTelegramCommandsSetParsesEntries(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"commands": []any{}}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	if _, _, code := run(t, "", "accounts", "telegram", "commands", "set", "acc_1", "--command", "start"); code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if len(api.paths()) != 0 {
		t.Fatalf("the CLI called %v despite a usage error", api.paths())
	}

	_, stderr, code := run(t, "", "accounts", "telegram", "commands", "set", "acc_1", "--command", "/start=Start here", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	sent := api.find(t, http.MethodPut, "/accounts/acc_1/telegram/commands")
	commands, _ := sent.Body["commands"].([]any)
	if len(commands) != 1 {
		t.Fatalf("commands = %v", sent.Body["commands"])
	}
	if entry, _ := commands[0].(map[string]any); entry["command"] != "start" || entry["description"] != "Start here" {
		t.Fatalf("command = %v", commands[0])
	}
}

func TestAccountsTelegramCommandsClearSkipsPromptWithYes(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"commands": []any{}}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "", "accounts", "telegram", "commands", "clear", "acc_1", "--yes", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	api.find(t, http.MethodDelete, "/accounts/acc_1/telegram/commands")
}

func TestAccountsSlackChannelsEmitsJSON(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "C1", "name": "general", "is_private": false, "is_member": true, "is_current": true}}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "accounts", "slack", "channels", "acc_1", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	api.find(t, http.MethodGet, "/accounts/acc_1/slack/channels")
	if !strings.Contains(stdout, `"is_current": true`) {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestAccountsSlackSetIdentitySendsOnlyPassedFields(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"username": nil, "icon_url": nil, "icon_emoji": ":rocket:"}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	if _, _, code := run(t, "", "accounts", "slack", "set-identity", "acc_1", "--icon-url", "https://example.com/a.png", "--icon-emoji", ":rocket:"); code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if len(api.paths()) != 0 {
		t.Fatalf("the CLI called %v despite a usage error", api.paths())
	}

	_, stderr, code := run(t, "", "accounts", "slack", "set-identity", "acc_1", "--clear-username", "--icon-emoji", ":rocket:", "--quiet")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	sent := api.find(t, http.MethodPatch, "/accounts/acc_1/slack/identity")
	if v, ok := sent.Body["username"]; !ok || v != nil {
		t.Fatalf("username = %v (present %v), want null", v, ok)
	}
	if sent.Body["icon_emoji"] != ":rocket:" {
		t.Fatalf("icon_emoji = %v", sent.Body["icon_emoji"])
	}
	if _, ok := sent.Body["icon_url"]; ok {
		t.Fatalf("icon_url was sent: %v", sent.Body)
	}
}
