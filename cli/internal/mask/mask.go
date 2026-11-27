/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

// Package mask renders secret strings for safe display in logs.
package mask

import "strings"

// maskWholeThreshold is the maximum length at which a secret is masked
// entirely (no head/tail fragments shown).
const maskWholeThreshold = 8

// maskFragmentLen is the number of characters shown on each side of the
// "****" separator for longer secrets.
const maskFragmentLen = 4

// Key renders only the head and tail fragments of a secret, replacing the
// middle with a fixed "****" regardless of its length; secrets of
// maskWholeThreshold characters or fewer are masked entirely.
func Key(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= maskWholeThreshold {
		return strings.Repeat("*", len(key))
	}
	return key[:maskFragmentLen] + "****" + key[len(key)-maskFragmentLen:]
}
