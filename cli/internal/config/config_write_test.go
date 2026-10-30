package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSetWritesUserInfo verifies the config.Set path persists a user_info map
// to the config file (isolated in a temp home, no side effects).
func TestSetWritesUserInfo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")

	if err := configSetForTest("user_info", map[string]any{
		"user_name": "hw067826022",
		"user_id":   "u-1",
	}); err != nil {
		t.Fatalf("set: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".huawei", "devbox", "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	t.Logf("config file content: %s", data)
	if !contains(string(data), "hw067826022") {
		t.Fatalf("user_name missing from config file: %s", data)
	}
}

// TestSetAfterEmptyFile verifies Set/Unset keep working after an empty config
// file (the state logout leaves behind) — an empty file must parse as no
// values, not as a parse error.
func TestSetAfterEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")

	if err := configSetForTest("user_info", map[string]any{"user_name": "a", "user_id": "b"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := configUnsetForTest("user_info"); err != nil {
		t.Fatalf("unset: %v", err)
	}

	// The file is now empty; Set must still work.
	if err := configSetForTest("user_info", map[string]any{"user_name": "hw067826022", "user_id": "u-1"}); err != nil {
		t.Fatalf("set after empty file: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".huawei", "devbox", "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	t.Logf("config file content: %s", data)
	if !contains(string(data), "hw067826022") {
		t.Fatalf("user_name missing from config file: %s", data)
	}
}

func configSetForTest(key string, value any) error {
	return Set(key, value)
}

func configUnsetForTest(key string) error {
	return Unset(key)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
