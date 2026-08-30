package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrefersFlagThenEnvThenFile(t *testing.T) {
	file := &File{APIKey: "fp_from_file", BaseURL: "https://file.example/v1", Workspace: "ws_file"}

	t.Run("flag wins", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "fp_from_env")
		resolved := Resolve(Flags{APIKey: "fp_from_flag"}, file)
		if resolved.APIKey != "fp_from_flag" {
			t.Fatalf("APIKey = %q, want the flag", resolved.APIKey)
		}
		if resolved.APIKeySource != SourceFlag {
			t.Fatalf("APIKeySource = %q, want %q", resolved.APIKeySource, SourceFlag)
		}
	})

	t.Run("environment beats the file", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "fp_from_env")
		resolved := Resolve(Flags{}, file)
		if resolved.APIKey != "fp_from_env" {
			t.Fatalf("APIKey = %q, want the environment", resolved.APIKey)
		}
		if resolved.APIKeySource != SourceEnv {
			t.Fatalf("APIKeySource = %q, want %q", resolved.APIKeySource, SourceEnv)
		}
	})

	t.Run("the file is the fallback", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "")
		resolved := Resolve(Flags{}, file)
		if resolved.APIKey != "fp_from_file" {
			t.Fatalf("APIKey = %q, want the file", resolved.APIKey)
		}
		if resolved.APIKeySource != SourceFile {
			t.Fatalf("APIKeySource = %q, want %q", resolved.APIKeySource, SourceFile)
		}
	})

	t.Run("nothing set", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "")
		resolved := Resolve(Flags{}, &File{})
		if resolved.APIKey != "" || resolved.APIKeySource != SourceNone {
			t.Fatalf("resolved = %+v, want an empty key from %q", resolved, SourceNone)
		}
	})

	t.Run("base URL and workspace follow the same order", func(t *testing.T) {
		t.Setenv(EnvBaseURL, "https://env.example/v1")
		resolved := Resolve(Flags{BaseURL: "https://flag.example/v1"}, file)
		if resolved.BaseURL != "https://flag.example/v1" {
			t.Fatalf("BaseURL = %q, want the flag", resolved.BaseURL)
		}
		resolved = Resolve(Flags{}, file)
		if resolved.BaseURL != "https://env.example/v1" {
			t.Fatalf("BaseURL = %q, want the environment", resolved.BaseURL)
		}
		if resolved.Workspace != "ws_file" {
			t.Fatalf("Workspace = %q, want the file", resolved.Workspace)
		}
	})
}

func TestSaveWritesOwnerOnlyAndReloads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	if err := Save(&File{APIKey: "fp_live_secret_value", Workspace: "ws_1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(dir, "fopost", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 600 — the file holds an API key", perm)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIKey != "fp_live_secret_value" || loaded.Workspace != "ws_1" {
		t.Fatalf("loaded = %+v, want the saved values", loaded)
	}
}

func TestSaveTightensPermissionsOnAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigHome, dir)

	path := filepath.Join(dir, "fopost", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(&File{APIKey: "fp_key"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 600", perm)
	}
}

func TestLoadOnAMissingFileIsNotAnError(t *testing.T) {
	t.Setenv(EnvConfigHome, t.TempDir())
	file, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty", file.APIKey)
	}
}

func TestPathHonoursXDGConfigHome(t *testing.T) {
	t.Setenv(EnvConfigHome, "/somewhere/config")
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/somewhere/config", "fopost", "config.json"); path != want {
		t.Fatalf("Path() = %q, want %q", path, want)
	}
}

func TestMaskNeverRevealsTheKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want string
	}{
		{"empty", "", ""},
		{"prefixed", "fp_live_9f2c4a7b1ee3d", "fp_" + "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" + "ee3d"},
		{"too short to split", "fp_abc", "fp_" + "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"},
		{"no prefix", "9f2c4a7b1e3d5g", "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" + "3d5g"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Mask(tc.key)
			if got != tc.want {
				t.Fatalf("Mask(%q) = %q, want %q", tc.key, got, tc.want)
			}
			if tc.key != "" && got == tc.key {
				t.Fatal("Mask returned the key unchanged")
			}
		})
	}
}

func TestMaskLeaksAtMostTheLastFourCharacters(t *testing.T) {
	key := "fp_live_abcdefghijklmnop"
	masked := Mask(key)
	secret := key[len("fp_"):]
	for length := 5; length <= len(secret); length++ {
		if contains(masked, secret[len(secret)-length:]) {
			t.Fatalf("Mask(%q) = %q leaks %d characters of the key", key, masked, length)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
