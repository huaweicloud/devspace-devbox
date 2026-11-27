//go:build windows

/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

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

// systemLang resolves the login page language: env vars first, then the
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
	r, _, callErr := procGetUserDefaultUILanguage.Call()
	if callErr != nil {
		return "en-us"
	}
	langID := uint16(r)
	// LANGID layout: the low 9 bits hold the primary language id; 0x04 is
	// Chinese. Mask with 0x3FF to ignore the sublanguage (simplified/
	// traditional) and match any Chinese UI language.
	const langIDPrimaryMask = 0x3FF
	const langIDChinese = 0x04
	if langID&langIDPrimaryMask == langIDChinese {
		return "zh-cn"
	}
	return "en-us"
}
