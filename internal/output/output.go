// Package output renders command results: aligned tables for a human, JSON
// for a script.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// Printer writes a command's result in the shape the caller asked for.
type Printer struct {
	Out   io.Writer
	Err   io.Writer
	JSON  bool
	Quiet bool
	Color bool
}

// New builds a printer for the given streams. Color is off whenever NO_COLOR
// is set to anything, per https://no-color.org, or the stream is not a terminal.
func New(out, errOut io.Writer, asJSON, quiet, noColor bool) *Printer {
	color := !noColor && !envSet("NO_COLOR") && isTerminal(out)
	return &Printer{Out: out, Err: errOut, JSON: asJSON, Quiet: quiet, Color: color}
}

func envSet(name string) bool {
	_, ok := os.LookupEnv(name)
	return ok
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Value prints v as indented JSON when --json is on, otherwise it calls human,
// which renders the table. A quiet run prints neither.
func (p *Printer) Value(v any, human func()) error {
	if p.JSON {
		return p.EmitJSON(v)
	}
	if p.Quiet {
		return nil
	}
	human()
	return nil
}

// EmitJSON writes v as indented JSON followed by a newline. It runs even under
// --quiet, since JSON is the machine's output, not chatter.
func (p *Printer) EmitJSON(v any) error {
	encoder := json.NewEncoder(p.Out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(v)
}

// Table writes rows under headers, padded into columns.
func (p *Printer) Table(headers []string, rows [][]string) {
	if p.Quiet {
		return
	}
	if len(rows) == 0 {
		p.Line("No results.")
		return
	}
	writer := tabwriter.NewWriter(p.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, strings.Join(p.headerCells(headers), "\t"))
	for _, row := range rows {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
	writer.Flush()
}

func (p *Printer) headerCells(headers []string) []string {
	cells := make([]string, 0, len(headers))
	for _, header := range headers {
		cells = append(cells, p.dim(strings.ToUpper(header)))
	}
	return cells
}

// Fields writes a label-and-value block, the detail view for a single resource.
func (p *Printer) Fields(pairs [][2]string) {
	if p.Quiet {
		return
	}
	writer := tabwriter.NewWriter(p.Out, 0, 0, 2, ' ', 0)
	for _, pair := range pairs {
		fmt.Fprintf(writer, "%s\t%s\n", p.dim(pair[0]), pair[1])
	}
	writer.Flush()
}

// Line writes one line of human-facing text, suppressed by --quiet and --json.
func (p *Printer) Line(format string, args ...any) {
	if p.Quiet || p.JSON {
		return
	}
	fmt.Fprintf(p.Out, format+"\n", args...)
}

// Success writes a confirmation line, suppressed by --quiet and --json.
func (p *Printer) Success(format string, args ...any) {
	if p.Quiet || p.JSON {
		return
	}
	fmt.Fprintf(p.Out, "%s %s\n", p.green("✓"), fmt.Sprintf(format, args...))
}

// Warn writes an advisory line to stderr, so it never pollutes piped output.
func (p *Printer) Warn(format string, args ...any) {
	if p.Quiet {
		return
	}
	fmt.Fprintf(p.Err, "%s %s\n", p.yellow("!"), fmt.Sprintf(format, args...))
}

func (p *Printer) dim(s string) string    { return p.paint("\x1b[2m", s) }
func (p *Printer) green(s string) string  { return p.paint("\x1b[32m", s) }
func (p *Printer) yellow(s string) string { return p.paint("\x1b[33m", s) }

func (p *Printer) paint(code, s string) string {
	if !p.Color {
		return s
	}
	return code + s + "\x1b[0m"
}

// Truncate shortens s to width runes, ending in an ellipsis, and flattens the
// newlines that would otherwise break a table row.
func Truncate(s string, width int) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// Dash renders an empty value as a dash, so a column never looks misaligned.
func Dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// Stamp renders a timestamp compactly, or a dash when there is none.
func Stamp(t time.Time, raw string) string {
	if !t.IsZero() {
		return t.UTC().Format("2006-01-02 15:04")
	}
	return Dash(raw)
}
