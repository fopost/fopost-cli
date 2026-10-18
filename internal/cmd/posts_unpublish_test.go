package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestPostsUnpublishConfirmsAndPrintsEachNetwork(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"post_status": "unpublished",
			"deliveries": []map[string]any{
				{"account_id": "acc_1", "platform": "linkedin", "removed": true},
				{"account_id": "acc_2", "platform": "bluesky", "removed": false},
			},
		}})
	})
	configured(t, api.URL)

	if _, _, code := run(t, "n\n", "posts", "unpublish", "post_1"); code != ExitUsage {
		t.Fatalf("a declined prompt exited %d, want %d", code, ExitUsage)
	}
	if len(api.paths()) != 0 {
		t.Fatalf("a declined prompt still sent %v", api.paths())
	}

	stdout, stderr, code := run(t, "", "posts", "unpublish", "post_1", "--yes")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "POST", "/posts/post_1/unpublish")
	if !strings.Contains(stdout, "unpublished") || !strings.Contains(stdout, "linkedin") || !strings.Contains(stdout, "kept") {
		t.Fatalf("unpublish did not print the outcome:\n%s", stdout)
	}

	stdout, _, code = run(t, "", "posts", "unpublish", "post_1", "--yes", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || result["post_status"] != "unpublished" {
		t.Fatalf("--json output is not the result: %v\n%s", err, stdout)
	}
}
