// Package config loads and stores the CLI's credentials and defaults.
//
// The file lives at $XDG_CONFIG_HOME/fopost/config.json, falling back to
// ~/.config/fopost/config.json, and is written with mode 0600 because it
// holds an API key.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileMode is the permission the config file is written with. The key inside
// it is a bearer credential, so nothing but the owner may read it.
const FileMode os.FileMode = 0o600

// Env var names the CLI reads.
const (
	EnvAPIKey  = "FOPOST_API_KEY"
	EnvBaseURL = "FOPOST_BASE_URL"
	// EnvConfigHome overrides where the config directory is looked up.
	EnvConfigHome = "XDG_CONFIG_HOME"
)

// Source says where a resolved value came from, so `auth status` can explain
// which one is winning.
type Source string

// The sources a value can come from, in precedence order.
const (
	SourceFlag Source = "flag"
	SourceEnv  Source = "environment"
	SourceFile Source = "config file"
	SourceNone Source = "unset"
)

// File is the on-disk config.
type File struct {
	APIKey    string `json:"api_key,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

// Path is where the config file lives.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Dir is the directory the config file lives in.
func Dir() (string, error) {
	if home := strings.TrimSpace(os.Getenv(EnvConfigHome)); home != "" {
		return filepath.Join(home, "fopost"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating your home directory: %w", err)
	}
	return filepath.Join(home, ".config", "fopost"), nil
}

// Load reads the config file. A missing file is not an error — it yields an
// empty config, which is what a first run looks like.
func Load() (*File, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	file := &File{}
	if err := json.Unmarshal(raw, file); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	return file, nil
}

// Save writes the config file, creating its directory and forcing mode 0600
// even when the file already existed with looser permissions.
func Save(file *File) error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, FileMode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	// WriteFile honours the mode only when it creates the file.
	if err := os.Chmod(path, FileMode); err != nil {
		return fmt.Errorf("securing %s: %w", path, err)
	}
	return nil
}

// Resolved is the effective configuration, with the origin of each value.
type Resolved struct {
	APIKey       string
	APIKeySource Source
	BaseURL      string
	Workspace    string
}

// Flags are the values a command-line flag supplied, empty when it did not.
type Flags struct {
	APIKey    string
	BaseURL   string
	Workspace string
}

// Resolve applies the precedence rule: flag beats environment, environment
// beats the config file.
func Resolve(flags Flags, file *File) Resolved {
	if file == nil {
		file = &File{}
	}
	out := Resolved{APIKeySource: SourceNone}

	switch {
	case strings.TrimSpace(flags.APIKey) != "":
		out.APIKey, out.APIKeySource = strings.TrimSpace(flags.APIKey), SourceFlag
	case strings.TrimSpace(os.Getenv(EnvAPIKey)) != "":
		out.APIKey, out.APIKeySource = strings.TrimSpace(os.Getenv(EnvAPIKey)), SourceEnv
	case strings.TrimSpace(file.APIKey) != "":
		out.APIKey, out.APIKeySource = strings.TrimSpace(file.APIKey), SourceFile
	}

	out.BaseURL = firstNonEmpty(flags.BaseURL, os.Getenv(EnvBaseURL), file.BaseURL)
	out.Workspace = firstNonEmpty(flags.Workspace, file.Workspace)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// Mask renders a key safely for display: the prefix, then a fixed run of dots,
// then the last four characters. A key too short to split is fully hidden, so
// masking never leaks more of a short key than of a long one.
func Mask(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	prefix := ""
	rest := key
	if idx := strings.Index(key, "_"); idx > 0 && idx < len(key)-1 {
		prefix, rest = key[:idx+1], key[idx+1:]
	}
	if len(rest) < 12 {
		return prefix + "••••••••"
	}
	return prefix + "••••••••" + rest[len(rest)-4:]
}
