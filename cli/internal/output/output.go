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
	"time"

	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// Format selects the rendering style.
type Format string

const (
	// FormatText renders tables or key/value pairs (human friendly).
	FormatText Format = "text"
	// FormatJSON renders the raw API payloads.
	FormatJSON Format = "json"
)

// ParseFormat validates/normalizes --format values. "pretty" is an alias of
// text (e2b-compatible flag values).
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "text", "table", "pretty":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf(i18n.T(i18n.Msg.Output.UnknownFormat), s)
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
		return fmt.Errorf(i18n.T(i18n.Msg.Output.EncodeFailed), err)
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
	for _, r := range rows {
		if len(r) != len(headers) {
			return fmt.Errorf(i18n.T(i18n.Msg.Output.RowLengthMismatch), len(r), len(headers))
		}
	}
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

// FormatTime renders t in local time as "2006-01-02 15:04:05"; a zero time
// renders as "-".
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// FormatTimePtr renders a nullable time field; nil renders as "-".
func FormatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return FormatTime(*t)
}

// JoinList joins string items with ", ".
func JoinList(items []string) string {
	return strings.Join(items, ", ")
}

// FormatMetadata renders sandbox metadata as JSON (e2b-compatible); nil or
// empty renders as "{}".
func FormatMetadata(m *map[string]string) string {
	if m == nil {
		return "{}"
	}
	return JSONString(*m)
}

// JSONString renders v as compact JSON; failures render as "{}".
func JSONString(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
