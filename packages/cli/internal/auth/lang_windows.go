//go:build windows

package auth

import (
	"os"
	"strings"
	"syscall"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")
)

// systemLang mirrors devbridge's language detection: env vars first, then the
// Windows default UI language.
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
	r, _, _ := procGetUserDefaultUILanguage.Call()
	langID := uint16(r)
	if langID&0x3FF == 0x04 {
		return "zh-cn"
	}
	return "en-us"
}
