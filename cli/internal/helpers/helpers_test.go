/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

package helpers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/rest/manager"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
)

// TestTranslateSessionError verifies the arbitration of a failed session
// stream: a destroyed/paused sandbox is translated into a friendly notice and
// a clean end, while any other case keeps the original transport error
// (mirrors e2b's health-check arbitration).
func TestTranslateSessionError(t *testing.T) {
	transportErr := fmt.Errorf("internal: protocol error: promised 1633951847 bytes: unexpected EOF")
	probe := func(sb *manager.Sandbox, err error) SandboxProbe {
		return func(context.Context, string) (*manager.Sandbox, error) { return sb, err }
	}

	notFoundProbe := probe(nil, &client.APIError{StatusCode: http.StatusNotFound})
	cases := []struct {
		name       string
		err        error
		probe      SandboxProbe
		wantNil    bool
		wantNotice bool // msgs must contain the sandbox id (both languages interpolate %s)
	}{
		{"normal exit code is never translated", &ExitCodeError{ExitCode: 2}, probe(nil, nil), false, false},
		{"destroyed sandbox gets a notice", transportErr, notFoundProbe, true, true},
		{"paused sandbox gets a notice", transportErr, probe(&manager.Sandbox{State: manager.Paused}, nil), true, true},
		{"running sandbox keeps the error", transportErr, probe(&manager.Sandbox{State: manager.Running}, nil), false, false},
		{"failed probe keeps the error", transportErr, probe(nil, fmt.Errorf("dial tcp: timeout")), false, false},
		{"nil probe keeps the error", transportErr, nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var msgs bytes.Buffer
			got := translateSessionError(context.Background(), &msgs, "sbx-1", tc.err, tc.probe)
			if tc.wantNil && got != nil {
				t.Fatalf("translateSessionError() = %v, want nil", got)
			}
			if !tc.wantNil && got == nil {
				t.Fatal("translateSessionError() = nil, want the original error")
			}
			if tc.wantNotice && !strings.Contains(msgs.String(), "sbx-1") {
				t.Errorf("notice should carry the sandbox id, got: %q", msgs.String())
			}
		})
	}
}

func TestSandboxJSONRedactsSensitiveFields(t *testing.T) {
	token := "secret-token"
	expiration := int64(123)
	sb := &manager.Sandbox{
		SandboxID:       "s1",
		State:           manager.Running,
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
	if got["state"] != string(manager.Running) {
		t.Errorf("state = %v, want %q", got["state"], manager.Running)
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
