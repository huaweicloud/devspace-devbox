// Package output renders command results as either aligned tables, key/value
// lines or raw JSON.
//
// Text rendering is delegated to github.com/jedib0t/go-pretty/v6/table while
// JSON output uses encoding/json, keeping command code free of formatting
// concerns.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

// Format selects the rendering style.
type Format string

const (
	// FormatText renders tables or key/value pairs (human friendly).
	FormatText Format = "text"
	// FormatJSON renders the raw API payloads.
	FormatJSON Format = "json"
)

// ParseFormat validates/normalises --output values.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "text", "table":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unknown output format %q (supported: text, json)", s)
	}
}

// KV is a single key/value line used for single-entity renders.
type KV struct {
	Key   string
	Value string
}

// Spec describes one render. Exactly one of the render variants applies:
//
//   - KV is used for single entities in text mode
//   - Headers+Rows build a table in text mode
//   - JSON carries the raw payload for JSON mode
type Spec struct {
	KV      []KV
	Headers []string
	Rows    [][]string
	JSON    any
}

// Print renders spec to w using format.
func Print(w io.Writer, format Format, spec Spec) error {
	if format == FormatJSON {
		return PrintJSON(w, spec.JSON)
	}
	if spec.KV != nil {
		return printKV(w, spec.KV)
	}
	return printTable(w, spec.Headers, spec.Rows)
}

// PrintJSON writes v as indented JSON followed by a newline.
func PrintJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		return err
	}
	return nil
}

func printKV(w io.Writer, kv []KV) error {
	width := 0
	for _, item := range kv {
		if n := len(item.Key); n > width {
			width = n
		}
	}
	for _, item := range kv {
		if _, err := fmt.Fprintf(w, "%-*s  %s\n", width, item.Key, item.Value); err != nil {
			return err
		}
	}
	return nil
}

func printTable(w io.Writer, headers []string, rows [][]string) error {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false
	t.Style().Options.SeparateHeader = false
	t.Style().Options.SeparateRows = false

	headerRow := make(table.Row, len(headers))
	for i, h := range headers {
		headerRow[i] = strings.ToUpper(h)
	}
	t.AppendHeader(headerRow)

	for _, r := range rows {
		row := make(table.Row, len(r))
		for i, cell := range r {
			row[i] = cell
		}
		t.AppendRow(row)
	}
	t.Render()
	return nil
}

// Dash renders empty strings as "-" for table cells.
func Dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
