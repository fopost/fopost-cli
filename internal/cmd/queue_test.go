package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestQueueSlotsAndPostsScheduleQueue(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/queue/slots":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "slot_1", "workspace_id": "ws_1", "account_id": nil,
				"weekday": 2, "time": "09:00", "timezone": "Europe/Zagreb",
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/queue/slots/slot_1":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "slot_1", "weekday": 0, "time": "10:30", "timezone": "Europe/Zagreb",
				"moved": []map[string]any{{"id": "post_9", "from": "2026-10-20T07:00:00Z", "to": "2026-10-25T08:30:00Z"}},
			}})
		case r.Method == http.MethodDelete && r.URL.Path == "/queue/slots/slot_1":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"deleted": true, "moved": []any{}}})
		case r.URL.Path == "/queue/slots":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "slot_1", "weekday": 2, "time": "09:00", "timezone": "Europe/Zagreb", "account_id": "acc_1"},
			}})
		case r.URL.Path == "/queue/next":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"next": "2026-10-20T07:00:00Z"}})
		case r.URL.Path == "/queue/preview":
			json.NewEncoder(w).Encode(map[string]any{"data": []string{"2026-10-20T07:00:00Z", "2026-10-27T07:00:00Z"}})
		case r.Method == http.MethodPost && r.URL.Path == "/posts":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "post_1", "status": "scheduled", "schedule_at": "2026-10-20T07:00:00Z",
			}})
		default:
			http.NotFound(w, r)
		}
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "queue", "slots", "create", "--weekday", "2", "--time", "09:00", "--timezone", "Europe/Zagreb")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	created := api.find(t, "POST", "/queue/slots")
	if created.Body["workspace_id"] != "ws_1" || created.Body["weekday"] != float64(2) || created.Body["time"] != "09:00" {
		t.Fatalf("create sent %v", created.Body)
	}
	if _, present := created.Body["account_id"]; present {
		t.Fatalf("create sent account_id for an every-account slot: %v", created.Body)
	}
	if !strings.Contains(stdout, "slot_1") || !strings.Contains(stdout, "Tue") {
		t.Fatalf("create did not print the slot:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "queue", "slots", "list")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if got := api.find(t, "GET", "/queue/slots").Query.Get("workspace_id"); got != "ws_1" {
		t.Fatalf("list sent workspace_id=%q", got)
	}
	if !strings.Contains(stdout, "acc_1") {
		t.Fatalf("list did not print the slot's account:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "queue", "slots", "update", "slot_1", "--weekday", "0", "--time", "10:30")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	updated := api.find(t, "PUT", "/queue/slots/slot_1")
	// Sunday is 0, so the weekday must travel even though it is the zero value.
	if updated.Body["weekday"] != float64(0) || updated.Body["time"] != "10:30" {
		t.Fatalf("update sent %v", updated.Body)
	}
	if _, present := updated.Body["timezone"]; present {
		t.Fatalf("update sent a timezone that was not passed: %v", updated.Body)
	}
	if !strings.Contains(stdout, "post_9") {
		t.Fatalf("update did not print the moved post:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "queue", "slots", "delete", "slot_1", "--yes")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "DELETE", "/queue/slots/slot_1")

	stdout, stderr, code = run(t, "", "queue", "next", "--account", "acc_1")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if got := api.find(t, "GET", "/queue/next").Query.Get("account_id"); got != "acc_1" {
		t.Fatalf("next sent account_id=%q", got)
	}
	if !strings.Contains(stdout, "2026-10-20") {
		t.Fatalf("next did not print the time:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "queue", "preview", "--count", "2")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if got := api.find(t, "GET", "/queue/preview").Query.Get("count"); got != "2" {
		t.Fatalf("preview sent count=%q", got)
	}
	if !strings.Contains(stdout, "2026-10-27") {
		t.Fatalf("preview did not print the second time:\n%s", stdout)
	}

	stdout, stderr, code = run(t, "", "posts", "create", "--account", "acc_1", "--text", "Next free slot", "--schedule", "queue")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	post := api.find(t, "POST", "/posts")
	if post.Body["schedule"] != "queue" || post.Body["status"] != "scheduled" {
		t.Fatalf("posts create sent %v", post.Body)
	}
	if _, present := post.Body["schedule_at"]; present {
		t.Fatalf("posts create sent schedule_at alongside the queue: %v", post.Body)
	}
	if !strings.Contains(stdout, "Scheduled post_1") {
		t.Fatalf("posts create did not report the schedule:\n%s", stdout)
	}

	if _, _, code := run(t, "", "posts", "create", "--account", "acc_1", "--text", "x", "--schedule", "queue", "--publish"); code != ExitUsage {
		t.Fatalf("--schedule queue with --publish exited %d, want %d", code, ExitUsage)
	}
	if _, _, code := run(t, "", "posts", "create", "--account", "acc_1", "--text", "x", "--schedule", "tomorrow"); code != ExitUsage {
		t.Fatalf("--schedule tomorrow exited %d, want %d", code, ExitUsage)
	}
}
