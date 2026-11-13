package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newVerifyServer mocks the API key check endpoint on the bridge domain.
// Only requests carrying validKey succeed with 204; everything else gets 401.
func newVerifyServer(t *testing.T, validKey string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/api-keys/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != validKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return httptest.NewServer(mux)
}

func TestVerifyAPIKeyValid(t *testing.T) {
	srv := newVerifyServer(t, "devbox_good-key")
	defer srv.Close()

	if err := VerifyAPIKey(context.Background(), srv.URL, "devbox_good-key"); err != nil {
		t.Fatalf("expected valid key to pass, got: %v", err)
	}
}

func TestVerifyAPIKeyInvalid(t *testing.T) {
	srv := newVerifyServer(t, "devbox_good-key")
	defer srv.Close()

	err := VerifyAPIKey(context.Background(), srv.URL, "devbox_bad-key")
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
