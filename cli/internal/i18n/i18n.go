// Package i18n provides minimal Chinese/English message selection for the
// CLI. The language comes from the DEVBOX_LANG environment variable (a value
// starting with "zh" selects Chinese); everything else defaults to English.
package i18n

import (
	"os"
	"strings"
)

type Lang string

const (
	ZH Lang = "zh"
	EN Lang = "en"
)

// Message carries one user-facing string in both supported languages.
type Message struct {
	ZH string
	EN string
}

var currentLang = detectLang()

// detectLang resolves the CLI language: DEVBOX_LANG wins (zh* -> Chinese),
// English is the default.
func detectLang() Lang {
	if env := os.Getenv("DEVBOX_LANG"); env != "" {
		if strings.HasPrefix(strings.ToLower(env), "zh") {
			return ZH
		}
		return EN
	}
	return EN
}

// T returns the message in the current language.
func T(msg Message) string {
	if currentLang == ZH {
		return msg.ZH
	}
	return msg.EN
}
