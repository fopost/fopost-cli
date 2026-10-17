package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func blogSettingsJSON(hasSecret bool) map[string]any {
	return map[string]any{
		"siteUrl": "https://yourbrand.com", "topic": "Running", "audience": nil,
		"goal": "signups", "focusTopics": "Trail gear", "avoidTopics": nil,
		"language": "en", "locationCode": 2840, "cadencePerWeek": 2, "reviewHoldDays": 3,
		"imageTier": "standard", "inBodyImages": 2,
		"minWords": 1200, "maxWords": 1800, "brandVoiceId": nil,
		"targetKind": "website", "targetAccountId": nil, "targetPlatform": nil,
		"websiteUrl": "https://yourbrand.com/hooks/fopost", "hasWebsiteSecret": hasSecret,
		"planRun":              map[string]any{"status": "done", "error": nil, "added": 14, "finishedAt": "2026-10-17T08:00:00.000Z"},
		"keywordDataAvailable": true, "updatedAt": "2026-10-11T10:00:00.000Z",
	}
}

func TestBlogSettingsSetSendsOnlyTheFlagsGivenAndPrintsTheSecretOnce(t *testing.T) {
	isolate(t)
	const secret = "whsec_9f2c4a7b1ee3d5g"
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": blogSettingsJSON(true), "websiteSecret": secret,
		})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "blog", "settings", "set",
		"--website-url", "https://yourbrand.com/hooks/fopost", "--review-hold-days", "0")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "PATCH", "/blog/settings")
	if sent.Body["workspaceId"] != "ws_1" {
		t.Fatalf("workspaceId = %v, want ws_1", sent.Body["workspaceId"])
	}
	if sent.Body["websiteUrl"] != "https://yourbrand.com/hooks/fopost" {
		t.Fatalf("websiteUrl = %v", sent.Body["websiteUrl"])
	}
	// An explicit zero still travels; an untouched field does not
	if sent.Body["reviewHoldDays"] != float64(0) {
		t.Fatalf("reviewHoldDays = %v, want 0", sent.Body["reviewHoldDays"])
	}
	if _, ok := sent.Body["cadencePerWeek"]; ok {
		t.Fatalf("set sent a flag that was not given: %v", sent.Body)
	}
	if !strings.Contains(stdout, secret) {
		t.Fatalf("set did not print the secret the API returned:\n%s", stdout)
	}
	if !strings.Contains(stderr, "not shown again") {
		t.Fatalf("set did not warn the secret is shown once:\n%s", stderr)
	}
}

func TestBlogSettingsSetWithoutASecretPrintsNone(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": blogSettingsJSON(true), "websiteSecret": nil})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "blog", "settings", "set", "--topic", "Trail running")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, "whsec") || strings.Contains(stderr, "not shown again") {
		t.Fatalf("set claimed a secret the API did not return:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "Trail running") && !strings.Contains(stdout, "Running") {
		t.Fatalf("set did not print the settings:\n%s", stdout)
	}
}

func TestBlogSettingsSetSendsTheGoalFields(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": blogSettingsJSON(false), "websiteSecret": nil})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "blog", "settings", "set",
		"--goal", "signups", "--focus-topics", "Trail gear", "--avoid-topics", "")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "PATCH", "/blog/settings")
	if sent.Body["goal"] != "signups" || sent.Body["focusTopics"] != "Trail gear" {
		t.Fatalf("body = %v", sent.Body)
	}
	// An explicit empty --avoid-topics clears the field rather than being dropped
	if avoid, ok := sent.Body["avoidTopics"]; !ok || avoid != "" {
		t.Fatalf("avoidTopics = %v (present %v), want an empty string", avoid, ok)
	}
	if _, ok := sent.Body["autoPublish"]; ok {
		t.Fatalf("set sent the removed autoPublish: %v", sent.Body)
	}
	if !strings.Contains(stdout, "signups") || !strings.Contains(stdout, "done, 14 added") {
		t.Fatalf("set did not print the goal and plan run:\n%s", stdout)
	}
}

func TestBlogSettingsSetRejectsTheRemovedAutoPublishFlag(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": blogSettingsJSON(false)})
	})
	configured(t, api.URL)

	if _, _, code := run(t, "", "blog", "settings", "set", "--auto-publish"); code == ExitOK {
		t.Fatalf("--auto-publish exit = %d, want a failure", code)
	}
	if len(api.paths()) != 0 {
		t.Fatalf("a removed flag reached the API: %v", api.paths())
	}
}

func TestBlogSettingsRotateSecretPrintsItWithTheNote(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"websiteSecret": "whsec_new"}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "blog", "settings", "rotate-secret", "--yes")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/blog/settings/website-secret")
	if sent.Body["workspaceId"] != "ws_1" {
		t.Fatalf("workspaceId = %v", sent.Body["workspaceId"])
	}
	if !strings.Contains(stdout, "whsec_new") || !strings.Contains(stderr, "not shown again") {
		t.Fatalf("rotate-secret output:\n%s%s", stdout, stderr)
	}
}

func TestBlogKeywordsPlanSendsTheSeeds(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "queued"}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "blog", "keywords", "plan", "--seed", "trail shoes", "--seed", "hydration", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/blog/keywords/plan")
	seeds, _ := sent.Body["seeds"].([]any)
	if len(seeds) != 2 || seeds[0] != "trail shoes" {
		t.Fatalf("seeds = %v", sent.Body["seeds"])
	}
	if !strings.Contains(stdout, `"queued"`) {
		t.Fatalf("plan --json did not report the queue:\n%s", stdout)
	}
}

func TestBlogKeywordsListFiltersByStatus(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
			"id": "kw_1", "keyword": "best trail shoes", "searchVolume": 4400, "difficulty": 38,
			"cpc": 1.25, "intent": "commercial", "source": "planner", "status": "planned",
			"plannedFor": "2026-10-20", "createdAt": "2026-10-11T10:00:00.000Z",
		}}})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "blog", "keywords", "list", "--status", "planned,suggested")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "GET", "/blog/keywords")
	if sent.Query.Get("workspaceId") != "ws_1" || sent.Query.Get("status") != "planned,suggested" {
		t.Fatalf("query = %v", sent.Query)
	}
	if !strings.Contains(stdout, "best trail shoes") || !strings.Contains(stdout, "4400") {
		t.Fatalf("list did not print the keyword:\n%s", stdout)
	}
}

func articleJSON(status string) map[string]any {
	return map[string]any{
		"id": "art_1", "title": "Best Trail Shoes", "slug": "best-trail-shoes",
		"keyword": "best trail shoes", "excerpt": nil, "metaTitle": nil, "metaDescription": nil,
		"featuredImage": nil, "tags": []string{"shoes"}, "wordCount": 1400, "status": status,
		"origin": "planner", "reviewUntil": nil, "publishAt": nil, "publishedAt": nil,
		"publishedUrl": nil, "error": nil,
		"createdAt": "2026-10-11T10:00:00.000Z", "updatedAt": "2026-10-11T10:00:00.000Z",
		"instructions": nil, "outline": []string{"Intro"}, "brief": map[string]any{},
		"bodyMarkdown": "# Best Trail Shoes\n\nThe body.", "bodyHtml": "<h1>Best Trail Shoes</h1>",
		"images": []any{},
	}
}

func TestBlogArticlesGetPrintsMetadataUnlessBodyIsAsked(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": articleJSON("draft")})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "", "blog", "articles", "get", "art_1")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "GET", "/blog/articles/art_1")
	if sent.Query.Get("workspaceId") != "ws_1" {
		t.Fatalf("query = %v", sent.Query)
	}
	if !strings.Contains(stdout, "best-trail-shoes") || strings.Contains(stdout, "The body.") {
		t.Fatalf("get printed the wrong view:\n%s", stdout)
	}

	stdout, _, code = run(t, "", "blog", "articles", "get", "art_1", "--body")
	if code != ExitOK || !strings.Contains(stdout, "# Best Trail Shoes\n\nThe body.") {
		t.Fatalf("get --body did not print the markdown (exit %d):\n%s", code, stdout)
	}
}

func TestBlogArticlesPublishPromptsUnlessYes(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{"data": articleJSON("publishing")})
	})
	configured(t, api.URL)

	// A declined prompt publishes nothing
	stdout, _, code := run(t, "n\n", "blog", "articles", "publish", "art_1")
	if code != ExitUsage {
		t.Fatalf("declined publish exit = %d, want %d", code, ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("a declined publish wrote to stdout: %q", stdout)
	}
	if len(api.paths()) != 0 {
		t.Fatalf("a declined publish reached the API: %v", api.paths())
	}

	stdout, stderr, code := run(t, "", "blog", "articles", "publish", "art_1", "--yes")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/blog/articles/art_1/publish")
	if sent.Query.Get("workspaceId") != "ws_1" {
		t.Fatalf("query = %v", sent.Query)
	}
	if !strings.Contains(stdout, "Publishing article art_1") {
		t.Fatalf("publish did not confirm:\n%s", stdout)
	}
}

func TestBlogArticlesEditReadsTheBodyFromStdin(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": articleJSON("draft")})
	})
	configured(t, api.URL)

	stdout, stderr, code := run(t, "# New body\n", "blog", "articles", "edit", "art_1",
		"--body-file", "-", "--tag", "shoes", "--tag", "trail", "--excerpt", "")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "PATCH", "/blog/articles/art_1")
	if sent.Body["bodyMarkdown"] != "# New body" {
		t.Fatalf("bodyMarkdown = %v", sent.Body["bodyMarkdown"])
	}
	tags, _ := sent.Body["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("tags = %v", sent.Body["tags"])
	}
	// An explicit empty --excerpt clears the field rather than being dropped
	if excerpt, ok := sent.Body["excerpt"]; !ok || excerpt != "" {
		t.Fatalf("excerpt = %v (present %v), want an empty string", excerpt, ok)
	}
}

func TestBlogArticlesRequestNeedsASubject(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{"data": articleJSON("queued")})
	})
	configured(t, api.URL)

	if _, _, code := run(t, "", "blog", "articles", "request"); code != ExitUsage {
		t.Fatalf("request with no subject exit = %d, want %d", code, ExitUsage)
	}
	stdout, stderr, code := run(t, "", "blog", "articles", "request", "--keyword-id", "kw_1", "--instructions", "Keep it short")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	sent := api.find(t, "POST", "/blog/articles")
	if sent.Body["workspaceId"] != "ws_1" || sent.Body["keywordId"] != "kw_1" {
		t.Fatalf("body = %v", sent.Body)
	}
}

func TestBlogDeleteCommandsPassTheWorkspaceInTheQuery(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	configured(t, api.URL)

	for _, args := range [][]string{
		{"blog", "keywords", "delete", "kw_1", "--yes", "--json"},
		{"blog", "articles", "delete", "art_1", "--yes", "--json"},
	} {
		stdout, stderr, code := run(t, "", args...)
		if code != ExitOK {
			t.Fatalf("%v exit = %d\n%s%s", args, code, stdout, stderr)
		}
		if !strings.Contains(stdout, `"deleted": true`) {
			t.Fatalf("%v --json did not acknowledge:\n%s", args, stdout)
		}
	}
	for _, path := range []string{"/blog/keywords/kw_1", "/blog/articles/art_1"} {
		if api.find(t, "DELETE", path).Query.Get("workspaceId") != "ws_1" {
			t.Fatalf("DELETE %s carried no workspaceId", path)
		}
	}
}
