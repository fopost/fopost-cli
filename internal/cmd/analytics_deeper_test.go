package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fopost/fopost-cli/internal/config"
)

// configured points the CLI at the fake API with a workspace already chosen,
// so the analytics commands need no flags beyond their own.
func configured(t *testing.T, baseURL string) {
	t.Helper()
	if err := config.Save(&config.File{APIKey: "fp_k", BaseURL: baseURL, Workspace: "ws_1"}); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyticsDecayPrintsTheBandsAndHalfLife(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"days":           30,
			"postsMeasured":  2,
			"halfLifeBucket": "1h_3h",
			"bands": []map[string]any{
				{"bucket": "under_1h", "label": "First hour", "posts": 2,
					"avgEngagements": 25, "avgImpressions": 300, "shareOfFinal": 0.3},
				{"bucket": "6h_12h", "label": "6-12 hours", "posts": 0,
					"avgEngagements": 0, "avgImpressions": 0, "shareOfFinal": nil},
			},
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "analytics", "decay", "--days", "30")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "GET", "/analytics/decay")
	if !strings.Contains(stdout, "1h_3h") {
		t.Fatalf("decay did not print the half life:\n%s", stdout)
	}
	if !strings.Contains(stdout, "First hour") {
		t.Fatalf("decay did not print the measured band:\n%s", stdout)
	}
	// A band nothing was measured in is left out rather than printed as zeroes
	if strings.Contains(stdout, "6-12 hours") {
		t.Fatalf("decay printed an empty band:\n%s", stdout)
	}
}

func TestAnalyticsFrequencyPrintsTheBestCadence(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"days": 90,
			"weeks": []map[string]any{
				{"weekStart": "2026-03-02", "posts": 2, "engagements": 240, "avgEngagementsPerPost": 120},
			},
			"bands": []map[string]any{
				{"band": "under_3", "label": "1-2 a week", "weeks": 1, "posts": 2,
					"avgPostsPerWeek": 2, "avgEngagementsPerPost": 120, "engagementRate": 0.12},
			},
			"best": map[string]any{"band": "under_3", "label": "1-2 a week", "avgEngagementsPerPost": 120},
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "analytics", "frequency", "--days", "90")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "GET", "/analytics/frequency")
	if !strings.Contains(stdout, "1-2 a week") {
		t.Fatalf("frequency did not print the best cadence:\n%s", stdout)
	}
}

func TestAnalyticsTimelineAcceptsAPermalink(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"postId": nil,
			"deliveries": []map[string]any{{
				"accountId": "acc_1", "platform": "twitter", "username": "acme",
				"externalPostId": "1", "postedAt": "2026-03-02T00:00:00.000Z",
				"points": []map[string]any{{
					"at": "2026-03-02T00:30:00.000Z", "ageMinutes": 30, "engagements": 40,
					"impressions": 400, "reach": nil, "likes": 30, "comments": nil,
					"shares": nil, "videoViews": nil,
					"delta": map[string]any{"impressions": 400, "reach": 0, "engagements": 40,
						"likes": 30, "comments": 0, "shares": 0},
				}},
			}},
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "analytics", "timeline", "https://x.com/acme/status/1")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	// The permalink travels as one escaped path segment
	api.find(t, "GET", "/analytics/posts/https://x.com/acme/status/1/timeline")
	if !strings.Contains(stdout, "30m") {
		t.Fatalf("timeline did not print the reading age:\n%s", stdout)
	}
}

func TestAnalyticsChangesPrintsTheCursor(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"since": "2026-03-02T00:00:00.000Z", "cursor": "2026-03-02T06:00:00.000Z",
			"hasMore": true,
			"changes": []map[string]any{{
				"accountId": "acc_1", "platform": "twitter", "externalPostId": "1",
				"postId": "post_1", "postedAt": "2026-03-02T00:00:00.000Z",
				"fetchedAt": "2026-03-02T06:00:00.000Z", "impressions": 900,
				"reach": nil, "engagements": 90, "likes": 70, "comments": 10, "shares": 10,
			}},
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "analytics", "changes", "--since", "2026-03-02T00:00:00Z")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "GET", "/analytics/changes")
	if !strings.Contains(stdout, "post_1") {
		t.Fatalf("changes did not print the reading:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2026-03-02T06:00:00Z") {
		t.Fatalf("changes did not print the cursor:\n%s", stdout)
	}
}

func TestAnalyticsCollectPostReportsEachDelivery(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"collected": 1,
			"deliveries": []map[string]any{{
				"accountId": "acc_1", "platform": "twitter", "externalPostId": "1",
				"collected": true, "fetchedAt": "2026-03-02T00:30:00.000Z", "message": nil,
			}},
		}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "analytics", "collect-post", "post_1")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "POST", "/posts/post_1/analytics/collect")
	if !strings.Contains(stdout, "true") {
		t.Fatalf("collect-post did not report the refresh:\n%s", stdout)
	}
}

func TestAnalyticsNativePostsListsPostsMadeOutsideFoPost(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"externalPostId": "1", "text": "Posted by hand",
				"permalink": "https://x.com/acme/status/1", "thumbnailUrl": nil,
				"mediaType": nil, "postedAt": "2026-03-02T00:00:00.000Z",
				"fetchedAt": "2026-03-02T06:00:00.000Z",
				"metrics": map[string]any{"impressions": 900, "reach": nil, "engagements": 90,
					"likes": 70, "comments": 10, "shares": 10, "videoViews": nil},
			}},
			"meta": map[string]any{"page": 1, "perPage": 20, "total": 1},
		})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "analytics", "native-posts", "--account", "acc_1")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "GET", "/accounts/acc_1/native-posts")
	if !strings.Contains(stdout, "Posted by hand") {
		t.Fatalf("native-posts did not print the post:\n%s", stdout)
	}
}
