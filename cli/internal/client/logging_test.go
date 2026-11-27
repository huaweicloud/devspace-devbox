/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

package client

import (
	"strings"
	"testing"

	"github.com/huaweicloud/devspace-devbox/cli/internal/mask"
)

// TestMaskSensitiveKeys verifies that connectToken/envdAccessToken values are
// masked in logged bodies at any nesting depth, while non-sensitive fields
// and non-JSON bodies pass through unchanged.
func TestMaskSensitiveKeys(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiJ9.secret.signature"
	masked := mask.Key(token)

	// Object body: top-level token masked, other fields untouched.
	obj := maskSensitiveKeys(`{"sandboxID":"abc","connectToken":"` + token + `"}`)
	if strings.Contains(obj, token) {
		t.Fatal("object body must not contain the clear connect token")
	}
	if !strings.Contains(obj, masked) {
		t.Fatalf("object body must contain the masked token %q, got %s", masked, obj)
	}
	if !strings.Contains(obj, `"sandboxID":"abc"`) {
		t.Fatal("non-sensitive fields must stay unchanged")
	}

	// Array body: tokens nested in list items are masked too.
	arr := maskSensitiveKeys(`[{"connectToken":"` + token + `"},{"envdAccessToken":"` + token + `"}]`)
	if strings.Contains(arr, token) {
		t.Fatal("array body must not contain the clear token")
	}
	if strings.Count(arr, masked) != 2 {
		t.Fatalf("both array items must be masked with %q, got %s", masked, arr)
	}

	// Non-JSON body: returned unchanged.
	raw := `<html>service unavailable</html>`
	if got := maskSensitiveKeys(raw); got != raw {
		t.Fatalf("non-JSON body must be unchanged, got %s", got)
	}

	// Sensitive key with a non-string value: left as-is (no panic, no masking).
	num := maskSensitiveKeys(`{"connectToken":123}`)
	if num != `{"connectToken":123}` {
		t.Fatalf("non-string sensitive value must stay unchanged, got %s", num)
	}
}
