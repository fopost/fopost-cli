package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fopost/fopost-cli/internal/config"
)

func TestXFollowSendsTheUserID(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"account_id": "acc_1", "user_id": "12345", "following": true,
		}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "accounts", "x", "follow", "acc_1", "--user-id", "12345", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	last := api.find(t, "POST", "/accounts/acc_1/x/follow")
	if last.Body["user_id"] != "12345" {
		t.Fatalf("body = %+v", last.Body)
	}
	var parsed struct {
		Following bool `json:"following"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil || !parsed.Following {
		t.Fatalf("follow --json = %q (%v)", stdout, err)
	}

	if _, stderr, code := run(t, "", "accounts", "x", "unfollow", "acc_1", "--user-id", "12345", "--quiet"); code != ExitOK {
		t.Fatalf("unfollow exit = %d: %s", code, stderr)
	}
	api.find(t, "POST", "/accounts/acc_1/x/unfollow")
}

func TestXFollowRequiresAUserID(t *testing.T) {
	isolate(t)
	if err := config.Save(&config.File{APIKey: "fp_k"}); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := run(t, "", "accounts", "x", "follow", "acc_1")
	if code == ExitOK || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q; want a usage failure with nothing on stdout", code, stdout)
	}
}

func TestXPostReadsWithTheAccountToken(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "987", "text": "hello", "author_handle": "acme", "likes": 3,
		}})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "x", "post", "987", "--account", "acc_1", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	req := api.find(t, "GET", "/x/posts/987")
	if req.Query.Get("account_id") != "acc_1" {
		t.Fatalf("query = %v", req.Query)
	}
	if !strings.Contains(stdout, `"author_handle": "acme"`) {
		t.Fatalf("post --json = %q", stdout)
	}
}

func TestXSearchSendsQueryAndCursor(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "1", "text": "one"}},
			"meta": map[string]any{"next_token": "tok_2"},
		})
	})
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: api.URL}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "x", "search", "--account", "acc_1", "--q", "fopost",
		"--max-results", "20", "--next-token", "tok_1")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	req := api.find(t, "GET", "/x/search")
	for key, want := range map[string]string{
		"account_id": "acc_1", "q": "fopost", "max_results": "20", "next_token": "tok_1",
	} {
		if got := req.Query.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if !strings.Contains(stdout, "tok_2") {
		t.Fatalf("the next cursor was not shown:\n%s", stdout)
	}

	stdout, _, code = run(t, "", "x", "search", "--account", "acc_1", "--q", "fopost", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	var page struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Meta struct {
			NextToken *string `json:"next_token"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &page); err != nil || len(page.Data) != 1 || page.Meta.NextToken == nil {
		t.Fatalf("search --json = %q (%v)", stdout, err)
	}
}
