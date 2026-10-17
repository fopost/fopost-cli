package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func articleBody() map[string]any {
	return map[string]any{
		"id": "99", "blog_id": "11", "title": "Spring drop",
		"body_html": "<p>Hello</p>", "excerpt": "A short summary",
		"status": "published", "author_name": "Store Owner",
		"tags": []string{"news"}, "image_url": nil,
		"url":          "https://demo.myshopify.com/blogs/article/spring-drop",
		"published_at": "2026-09-01T10:00:00.000Z", "updated_at": nil,
	}
}

func TestBlogsListShowsEveryBlogOnTheSite(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "11", "title": "News", "handle": "news", "url": nil},
		}})
	})

	stdout, stderr, code := run(t, "", "blogs", "list", "acc_1",
		"--api-key", "fp_k", "--base-url", api.URL)
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if got := api.paths(); len(got) != 1 || got[0] != "GET /accounts/acc_1/blogs" {
		t.Fatalf("paths = %v", got)
	}
	if !strings.Contains(stdout, "News") {
		t.Fatalf("stdout did not list the blog:\n%s", stdout)
	}
}

func TestArticlesListSendsTheFilters(t *testing.T) {
	isolate(t)
	var query string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Encode()
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{articleBody()}})
	})

	stdout, stderr, code := run(t, "", "blogs", "articles", "list", "acc_1", "11",
		"--limit", "5", "--status", "draft", "--query", "spring",
		"--api-key", "fp_k", "--base-url", api.URL)
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}
	if query != "limit=5&q=spring&status=draft" {
		t.Fatalf("query = %q", query)
	}
	if !strings.Contains(stdout, "Spring drop") {
		t.Fatalf("stdout did not list the article:\n%s", stdout)
	}
}

// The article id is in the path and only the flags that were passed travel,
// which is what stops an edit from creating a second post on the site.
func TestArticlesUpdateChangesTheLiveArticleInPlace(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": articleBody()})
	})

	stdout, stderr, code := run(t, "", "blogs", "articles", "update", "acc_1", "11", "99",
		"--title", "Spring drop, restocked",
		"--api-key", "fp_k", "--base-url", api.URL)
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}

	sent := api.find(t, http.MethodPatch, "/accounts/acc_1/blogs/11/articles/99")
	if len(sent.Body) != 1 || sent.Body["title"] != "Spring drop, restocked" {
		t.Fatalf("body = %v", sent.Body)
	}
}

func TestArticlesUpdateRefusesAnEmptyChangeBeforeCallingTheAPI(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the CLI called the API with nothing to change")
	})

	_, stderr, code := run(t, "", "blogs", "articles", "update", "acc_1", "11", "99",
		"--api-key", "fp_k", "--base-url", api.URL)
	if code == ExitOK {
		t.Fatalf("an empty update succeeded:\n%s", stderr)
	}
	if !strings.Contains(stderr, "at least one field") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestArticlesDeleteAsksBeforeRemovingFromTheSite(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"deleted": true}})
	})

	// Answering "n" leaves the site alone.
	_, _, code := run(t, "n\n", "blogs", "articles", "delete", "acc_1", "11", "99",
		"--api-key", "fp_k", "--base-url", api.URL)
	if code == ExitOK {
		t.Fatal("declining the prompt still deleted the article")
	}
	if got := api.paths(); len(got) != 0 {
		t.Fatalf("the CLI called the API after a declined prompt: %v", got)
	}

	if _, _, code = run(t, "", "blogs", "articles", "delete", "acc_1", "11", "99", "--yes",
		"--api-key", "fp_k", "--base-url", api.URL); code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	api.find(t, http.MethodDelete, "/accounts/acc_1/blogs/11/articles/99")
}

func TestProductsUpdateSendsOnlyWhatChanged(t *testing.T) {
	isolate(t)
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "7", "title": "Mug XL", "handle": "mug", "status": "draft",
			"product_type": "Drinkware", "tags": []string{},
			"price": "12.00", "currency": "USD",
		}})
	})

	stdout, stderr, code := run(t, "", "blogs", "products", "update", "acc_1", "7",
		"--title", "Mug XL", "--product-type", "Drinkware",
		"--api-key", "fp_k", "--base-url", api.URL)
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
	}

	sent := api.find(t, http.MethodPatch, "/accounts/acc_1/products/7")
	if len(sent.Body) != 2 || sent.Body["title"] != "Mug XL" || sent.Body["product_type"] != "Drinkware" {
		t.Fatalf("body = %v", sent.Body)
	}
}
