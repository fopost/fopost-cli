package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFontsUploadAndDelete(t *testing.T) {
	isolate(t)
	var family, workspace, filename string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/fonts":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("upload was not multipart: %v", err)
			}
			family, workspace = r.FormValue("family"), r.FormValue("workspace_id")
			if _, header, err := r.FormFile("file"); err == nil {
				filename = header.Filename
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "font_1", "family": "Brand Sans", "style": "normal", "weight": 700, "format": "woff2",
			}})
		case r.Method == http.MethodDelete && r.URL.Path == "/fonts/font_1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	configured(t, api.URL)

	path := filepath.Join(t.TempDir(), "brand-sans.woff2")
	if err := os.WriteFile(path, []byte("wOF2"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := run(t, "", "fonts", "upload", path, "--family", "Brand Sans", "--weight", "700")
	if code != ExitOK || !strings.Contains(stdout, "font_1") {
		t.Fatalf("upload = %d\n%s%s", code, stdout, stderr)
	}
	if family != "Brand Sans" || workspace != "ws_1" || filename != "brand-sans.woff2" {
		t.Fatalf("upload sent family=%q workspace=%q file=%q", family, workspace, filename)
	}

	stdout, stderr, code = run(t, "", "fonts", "delete", "font_1", "-y", "--json")
	if code != ExitOK || !strings.Contains(stdout, `"font_1"`) {
		t.Fatalf("delete = %d\n%s%s", code, stdout, stderr)
	}
	api.find(t, "DELETE", "/fonts/font_1")
}
