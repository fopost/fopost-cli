package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func videoEditJob(status string, extra map[string]any) map[string]any {
	job := map[string]any{
		"id": "ve_1", "status": status, "progress": 0, "source_media_id": "med_1",
		"operations": []any{map[string]any{"op": "trim"}},
		"created_at": "2026-11-20T10:00:00Z", "updated_at": "2026-11-20T10:00:00Z",
	}
	for key, value := range extra {
		job[key] = value
	}
	return job
}

func TestVideoEditsCreateWaitsForTheResult(t *testing.T) {
	isolate(t)
	var polls atomic.Int32
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/video-edits":
			json.NewEncoder(w).Encode(map[string]any{"data": videoEditJob("queued", nil)})
		case r.Method == http.MethodGet && r.URL.Path == "/video-edits/ve_1":
			if polls.Add(1) < 2 {
				json.NewEncoder(w).Encode(map[string]any{"data": videoEditJob("running", nil)})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": videoEditJob("succeeded", map[string]any{
				"progress": 1, "result_media_id": "med_2",
			})})
		default:
			http.NotFound(w, r)
		}
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "video-edits", "create", "--media", "med_1",
		"--op", `trim={"start":2,"end":12}`, "--op", "silence", "--wait", "--poll-interval", "1ms", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, `"succeeded"`) || !strings.Contains(stdout, `"med_2"`) {
		t.Fatalf("--wait did not print the finished job:\n%s", stdout)
	}
	sent := api.find(t, "POST", "/video-edits")
	operations, _ := sent.Body["operations"].([]any)
	if sent.Body["media_id"] != "med_1" || len(operations) != 2 {
		t.Fatalf("create sent %v", sent.Body)
	}
	trim, _ := operations[0].(map[string]any)
	params, _ := trim["params"].(map[string]any)
	if trim["op"] != "trim" || params["end"] != float64(12) {
		t.Fatalf("create sent operation %v", operations[0])
	}
}

func TestVideoEditsGetWaitExitsNonZeroOnFailure(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": videoEditJob("failed", map[string]any{"error": "unreadable video"})})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "video-edit", "get", "ve_1", "--wait", "--json")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\n%s%s", code, ExitError, stdout, stderr)
	}
	if stdout != "" || !strings.Contains(stderr, "unreadable video") {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
}
