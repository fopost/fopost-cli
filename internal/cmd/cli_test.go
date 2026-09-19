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
