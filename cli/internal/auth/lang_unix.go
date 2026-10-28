//go:build !windows

package auth

import (
	"os"
	"strings"
)

// systemLang mirrors devbridge's language detection: env vars first, default
// to en-us.
func systemLang() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := os.Getenv(key); val != "" {
			lower := strings.ToLower(val)
			if strings.HasPrefix(lower, "zh") {
				return "zh-cn"
			}
			if strings.HasPrefix(lower, "en") {
				return "en-us"
			}
		}
	}
	return "en-us"
}
