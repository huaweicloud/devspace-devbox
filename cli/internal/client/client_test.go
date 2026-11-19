package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// TestAPIErrorIsUnauthorized verifies that a 401 APIError matches the
// ErrUnauthorized sentinel via errors.Is, so all unauthorized failures are
// recognized uniformly no matter which code path produced them.
func TestAPIErrorIsUnauthorized(t *testing.T) {
	err := &APIError{StatusCode: http.StatusUnauthorized, Message: "bad key"}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatal("401 APIError must match ErrUnauthorized via errors.Is")
	}

	err = &APIError{StatusCode: http.StatusForbidden, Message: "no"}
	if errors.Is(err, ErrUnauthorized) {
		t.Fatal("non-401 APIError must not match ErrUnauthorized")
	}

	if !IsSentinelWrapped(ErrUnauthorized) {
		t.Fatal("ErrUnauthorized itself must match")
	}
}

// IsSentinelWrapped reports whether the sentinel matches itself (sanity check
// for errors.Is semantics).
func IsSentinelWrapped(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// TestUserAgentInjected verifies that the version set via UserAgent is sent
// as "devbox-cli/<version>" in the User-Agent header of the Manager API
// client, while the key check request stays header-free.
func TestUserAgentInjected(t *testing.T) {
	UserAgent = "devbox-cli/0.1.0-release"

	var gotAPI, gotCheck string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == DefaultCheckPath {
			gotCheck = r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		gotAPI = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c, err := New(srv.URL, "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.api.ListTemplatesWithResponse(context.Background()); err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if gotAPI != "devbox-cli/0.1.0-release" {
		t.Fatalf("API User-Agent = %q, want %q", gotAPI, "devbox-cli/0.1.0-release")
	}

	if err := VerifyAPIKey(context.Background(), srv.URL, "devbox_key"); err != nil {
		t.Fatalf("VerifyAPIKey: %v", err)
	}
	if strings.HasPrefix(gotCheck, "devbox-cli/") {
		t.Fatalf("check request must not carry the devbox-cli User-Agent, got %q", gotCheck)
	}
}

// TestCheckStatusUpgradeRequired verifies that a 426 response maps to
// UpgradeRequiredError, carrying the server message when present and falling
// back to the localized hint otherwise, both followed by the one-click
// upgrade commands.
func TestCheckStatusUpgradeRequired(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusUpgradeRequired}

	err := checkStatus(resp, []byte(`{"message":"minimum version 0.2.0"}`))
	var ue *UpgradeRequiredError
	if !errors.As(err, &ue) {
		t.Fatalf("426 must map to UpgradeRequiredError, got %T", err)
	}
	if ue.Message != "minimum version 0.2.0" {
		t.Fatalf("Message = %q, want the server message", ue.Message)
	}
	for _, want := range []string{"minimum version 0.2.0", "Update:", "OBS", "GitCode", "GitHub"} {
		if !strings.Contains(ue.Error(), want) {
			t.Fatalf("Error() = %q, want it to contain %q", ue.Error(), want)
		}
	}
	wantCmd := "curl -fsSL"
	if runtime.GOOS == "windows" {
		wantCmd = "irm "
	}
	if !strings.Contains(ue.Error(), wantCmd) {
		t.Fatalf("Error() = %q, want it to contain %q", ue.Error(), wantCmd)
	}

	err = checkStatus(resp, []byte("not json"))
	if !errors.As(err, &ue) {
		t.Fatalf("426 must map to UpgradeRequiredError, got %T", err)
	}
	if !strings.Contains(ue.Error(), i18n.T(i18n.Msg.Client.UpgradeRequired)) {
		t.Fatalf("Error() = %q, want the localized hint", ue.Error())
	}
}
