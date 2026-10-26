package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/huaweicloud/devspace-devbox/packages/cli/internal/config"
)

// newVerifyServer mocks the API key check endpoint on the relay controller.
// Only requests carrying validKey succeed with 204; everything else gets 401.
func newVerifyServer(t *testing.T, validKey string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(config.RelayControllerPath+"/auth/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != validKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return httptest.NewServer(mux)
}

func TestVerifyAPIKeyValid(t *testing.T) {
	srv := newVerifyServer(t, "devbridge_good-key")
	defer srv.Close()

	if err := VerifyAPIKey(context.Background(), srv.URL, "devbridge_good-key"); err != nil {
		t.Fatalf("expected valid key to pass, got: %v", err)
	}
}

func TestVerifyAPIKeyInvalid(t *testing.T) {
	srv := newVerifyServer(t, "devbridge_good-key")
	defer srv.Close()

	err := VerifyAPIKey(context.Background(), srv.URL, "devbridge_bad-key")
	if err == nil {
		t.Fatal("expected an error for an invalid key")
	}
	if !IsUnauthorized(err) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestVerifyAPIKeyDevboxKey(t *testing.T) {
	srv := newVerifyServer(t, "devbox_good-key")
	defer srv.Close()

	if err := VerifyAPIKey(context.Background(), srv.URL, "devbox_good-key"); err != nil {
		t.Fatalf("expected valid devbox key to pass, got: %v", err)
	}
	if err := VerifyAPIKey(context.Background(), srv.URL, "devbox_bad-key"); !IsUnauthorized(err) {
		t.Fatalf("expected ErrUnauthorized for invalid devbox key, got: %v", err)
	}
}

func TestVerifyAPIKeyEmptyFailsFast(t *testing.T) {
	if err := VerifyAPIKey(context.Background(), "http://127.0.0.1:1", ""); !IsUnauthorized(err) {
		t.Fatalf("expected ErrUnauthorized for empty key, got: %v", err)
	}
}

func TestStartLoginWithCallback(t *testing.T) {
	session, err := StartLogin("https://login.example.com/auth")
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	defer session.Close()

	if !strings.Contains(session.URL, "origin=devbox") {
		t.Errorf("login URL should carry the origin param: %s", session.URL)
	}

	u, err := url.Parse(session.URL)
	if err != nil {
		t.Fatalf("parse login URL: %v", err)
	}
	port := u.Query().Get("callback")
	if port == "" {
		t.Fatalf("login URL missing callback port: %s", session.URL)
	}

	callback := "http://127.0.0.1:" + port + "/"
	body := strings.NewReader(`{"apiKey":"callback-key-42"}`)
	resp, err := http.Post(callback, "application/json", body)
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()

	got := make(chan string, 1)
	go func() {
		key, err := session.Wait(context.Background())
		if err != nil {
			t.Errorf("wait: %v", err)
			return
		}
		got <- key
	}()

	key := <-got
	if key != "callback-key-42" {
		t.Fatalf("expected callback-key-42, got %q", key)
	}
}

func TestStartLoginRejectsMissingKey(t *testing.T) {
	session, err := StartLogin("https://login.example.com/auth")
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	defer session.Close()

	u, _ := url.Parse(session.URL)
	port := u.Query().Get("callback")

	resp, err := http.Post("http://127.0.0.1:"+port+"/", "application/json",
		strings.NewReader(`{"error_code":"E1"}`))
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing key, got %d", resp.StatusCode)
	}
}
