package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	fopost "github.com/fopost/fopost-go"
)

func itoa(n int) string { return strconv.Itoa(n) }

// scheduleLayouts are the forms --schedule-at accepts, most specific first.
var scheduleLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// parseSchedule reads a timestamp in local time unless it carries an offset.
func parseSchedule(value string) (*fopost.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range scheduleLayouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return fopost.NewTime(parsed), nil
		}
	}
	return nil, usageErrorf("could not read %q as a time. Use RFC 3339 (2026-09-01T14:30:00Z) or \"2026-09-01 14:30\"", value)
}

// readText resolves the text of a post from --text or --text-file, where a
// path of "-" means standard input.
func readText(text, textFile string, in io.Reader) (string, error) {
	switch {
	case text != "" && textFile != "":
		return "", usageErrorf("--text and --text-file are alternatives; pass one")
	case text != "":
		return text, nil
	case textFile == "":
		return "", nil
	case textFile == "-":
		raw, err := io.ReadAll(in)
		if err != nil {
			return "", fmt.Errorf("reading the post text from stdin: %w", err)
		}
		return strings.TrimRight(string(raw), "\n"), nil
	default:
		raw, err := os.ReadFile(textFile)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", textFile, err)
		}
		return strings.TrimRight(string(raw), "\n"), nil
	}
}

// readAll drains standard input.
func readAll(state *State) ([]byte, error) {
	raw, err := io.ReadAll(state.In)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return raw, nil
}

// confirm asks before a destructive action. A non-interactive run must pass
// --yes rather than be prompted into a hang.
func confirm(state *State, prompt string) error {
	fmt.Fprintf(state.Err, "%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(state.In)
	if !scanner.Scan() {
		return usageErrorf("cancelled")
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return usageErrorf("cancelled")
	}
	return nil
}

// deleted is the acknowledgement a delete prints under --json.
type deleted struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}
