package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetWritesUserInfo verifies the config.Set path persists a user_info map
// to the config file (isolated in a temp home, no side effects).
func TestSetWritesUserInfo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	if err := Set("user_info", map[string]any{
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
	if !strings.Contains(string(data), "hw067826022") {
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

	if err := Set("user_info", map[string]any{"user_name": "a", "user_id": "b"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := Unset("user_info"); err != nil {
		t.Fatalf("unset: %v", err)
	}

	// The file is now empty; Set must still work.
	if err := Set("user_info", map[string]any{"user_name": "hw067826022", "user_id": "u-1"}); err != nil {
		t.Fatalf("set after empty file: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".huawei", "devbox", "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	t.Logf("config file content: %s", data)
	if !strings.Contains(string(data), "hw067826022") {
		t.Fatalf("user_name missing from config file: %s", data)
	}
}
