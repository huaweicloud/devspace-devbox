package helpers

import (
	"testing"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api"
)

func TestSandboxJSONRedactsSensitiveFields(t *testing.T) {
	token := "secret-token"
	expiration := int64(123)
	sb := &api.Sandbox{
		SandboxID:       "s1",
		State:           api.Running,
		TemplateID:      "tpl-1",
		ConnectToken:    &token,
		TokenExpiration: &expiration,
	}
	got := SandboxJSON(sb)
	if got == nil {
		t.Fatal("SandboxJSON returned nil")
	}
	if _, ok := got["connectToken"]; ok {
		t.Error("connectToken must be redacted from JSON output")
	}
	if _, ok := got["tokenExpiration"]; ok {
		t.Error("tokenExpiration must be redacted from JSON output")
	}
	if got["sandboxID"] != "s1" {
		t.Errorf("sandboxID = %v, want s1", got["sandboxID"])
	}
	if got["state"] != string(api.Running) {
		t.Errorf("state = %v, want %q", got["state"], api.Running)
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"abc", "abc"},
		{"a b", "'a b'"},
		{"it's", `'it'"'"'s'`},
		{"a@b%c+=:,./-9", "a@b%c+=:,./-9"},
		{"$HOME", "'$HOME'"},
	}
	for _, c := range cases {
		if got := ShellQuote(c.in); got != c.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildRemoteCommand(t *testing.T) {
	got, err := BuildRemoteCommand([]string{"--", "echo", "a b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "echo 'a b'"; got != want {
		t.Errorf("BuildRemoteCommand = %q, want %q", got, want)
	}
	if _, err := BuildRemoteCommand(nil); err == nil {
		t.Error("empty command must fail")
	}
	got, err = BuildRemoteCommand([]string{"ls -la"})
	if err != nil || got != "ls -la" {
		t.Errorf("single arg passthrough: got %q, err %v", got, err)
	}
}
