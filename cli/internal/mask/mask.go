// Package mask renders secret strings for safe display in logs.
package mask

import "strings"

// Key renders only the head and tail fragments of a secret, replacing the
// middle with a fixed "****" regardless of its length; secrets of 8
// characters or fewer are masked entirely.
func Key(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + "****" + key[len(key)-4:]
}
