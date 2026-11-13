// Package mask renders secret strings for safe display in logs.
package mask

import "strings"

// Key renders only the head and tail fragments of a secret, masking the
// middle; secrets of 8 characters or fewer are masked entirely.
func Key(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
