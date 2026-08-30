package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestValueEmitsJSONWhenAsked(t *testing.T) {
	out := &bytes.Buffer{}
	printer := New(out, &bytes.Buffer{}, true, false, true)

	humanRan := false
	err := printer.Value(map[string]any{"id": "post_1", "status": "draft"}, func() { humanRan = true })
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if humanRan {
		t.Fatal("the table renderer ran under --json")
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if decoded["id"] != "post_1" || decoded["status"] != "draft" {
		t.Fatalf("decoded = %v, want the values passed in", decoded)
	}
	if !strings.HasSuffix(out.String(), "\n") {
		t.Fatal("JSON output should end in a newline so it pipes cleanly")
	}
}

func TestValueRendersTheHumanViewByDefault(t *testing.T) {
	out := &bytes.Buffer{}
	printer := New(out, &bytes.Buffer{}, false, false, true)

	humanRan := false
	if err := printer.Value(struct{}{}, func() { humanRan = true }); err != nil {
		t.Fatal(err)
	}
	if !humanRan {
		t.Fatal("the table renderer did not run")
	}
	if out.Len() != 0 {
		t.Fatalf("Value wrote %q on its own; the human view owns the output", out.String())
	}
}

func TestQuietSuppressesTheHumanViewButNotJSON(t *testing.T) {
	out := &bytes.Buffer{}
	printer := New(out, &bytes.Buffer{}, false, true, true)
	printer.Table([]string{"id"}, [][]string{{"post_1"}})
	printer.Line("chatter")
	printer.Success("done")
	if out.Len() != 0 {
		t.Fatalf("--quiet still wrote %q", out.String())
	}

	out.Reset()
	jsonPrinter := New(out, &bytes.Buffer{}, true, true, true)
	if err := jsonPrinter.Value(map[string]string{"id": "post_1"}, func() {}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "post_1") {
		t.Fatalf("--json --quiet dropped the machine output: %q", out.String())
	}
}

func TestNoColorEnvDisablesEscapes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := &bytes.Buffer{}
	printer := New(out, &bytes.Buffer{}, false, false, false)
	printer.Table([]string{"id"}, [][]string{{"post_1"}})
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("NO_COLOR was ignored: %q", out.String())
	}
}

func TestTableAlignsColumnsAndReportsEmptiness(t *testing.T) {
	out := &bytes.Buffer{}
	printer := New(out, &bytes.Buffer{}, false, false, true)
	printer.Table([]string{"id", "status"}, [][]string{{"a", "draft"}, {"bbbbbb", "published"}})
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want a header plus two rows:\n%s", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], "ID") {
		t.Fatalf("header = %q, want upper-case column names", lines[0])
	}

	out.Reset()
	printer.Table([]string{"id"}, nil)
	if !strings.Contains(out.String(), "No results.") {
		t.Fatalf("empty table said %q", out.String())
	}
}

func TestTruncateFlattensAndShortens(t *testing.T) {
	if got := Truncate("one\ntwo   three", 40); got != "one two three" {
		t.Fatalf("Truncate = %q", got)
	}
	if got := Truncate("abcdefghij", 5); got != "abcd…" {
		t.Fatalf("Truncate = %q, want an ellipsis at width 5", got)
	}
}

func TestDashFillsEmptyCells(t *testing.T) {
	if Dash("  ") != "-" || Dash("x") != "x" {
		t.Fatal("Dash should replace blank values with a dash and pass others through")
	}
}
