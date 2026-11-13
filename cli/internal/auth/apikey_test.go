package auth

import (
	"testing"
)

func TestReadValidAPIKey(t *testing.T) {
	t.Setenv("HW_DEVBOX_API_KEY", "env-key")
	if cred := ReadValidAPIKey(); cred == nil || cred.APIKey != "env-key" {
		t.Fatalf("expected env key, got %+v", cred)
	}

	SetOverrideAPIKey("override-key")
	if cred := ReadValidAPIKey(); cred == nil || cred.APIKey != "override-key" {
		t.Fatalf("expected override key to win, got %+v", cred)
	}
	SetOverrideAPIKey("")
}
