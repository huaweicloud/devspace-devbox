// Rendering helpers shared by the commands: local-time formatting and the
// e2b-compatible value styles.

package output

import (
	"encoding/json"
	"time"
)

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
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
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
