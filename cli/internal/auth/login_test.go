package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartLoginWithCallback(t *testing.T) {
	session, err := StartLogin("https://devstation.connect.huaweicloud.com")
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
	body := strings.NewReader(`{"apiKey":"callback-key-42","userName":"tester","userId":"u-1"}`)
	req, err := http.NewRequest(http.MethodPost, callback, body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://devstation.connect.huaweicloud.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()

	cred, ui, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if cred.APIKey != "callback-key-42" {
		t.Fatalf("expected callback-key-42, got %q", cred.APIKey)
	}
	if ui == nil || ui.UserName != "tester" || ui.UserID != "u-1" {
		t.Fatalf("expected user info from the callback, got %+v", ui)
	}
}

func TestStartLoginRejectsMissingKey(t *testing.T) {
	session, err := StartLogin("https://devstation.connect.huaweicloud.com")
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	defer session.Close()

	u, _ := url.Parse(session.URL)
	port := u.Query().Get("callback")

	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+port+"/",
		strings.NewReader(`{"error_code":"E1"}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://devstation.connect.huaweicloud.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing key, got %d", resp.StatusCode)
	}
}

// postCallback posts body with the given Origin header ("" omits it) and
// returns the response status.
func postCallback(t *testing.T, port, origin, body string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+port+"/",
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// TestStartLoginRejectsForeignOrigin verifies the callback fails closed: only
// the login page origin is accepted; foreign or missing Origin headers get 403
// and never satisfy the wait (a planted-key CSRF is rejected).
func TestStartLoginRejectsForeignOrigin(t *testing.T) {
	cases := []struct {
		name   string
		origin string
	}{
		{"mismatched origin", "https://evil.example.com"},
		{"missing origin", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, err := StartLogin("https://devstation.connect.huaweicloud.com")
			if err != nil {
				t.Fatalf("start login: %v", err)
			}
			defer session.Close()

			u, _ := url.Parse(session.URL)
			port := u.Query().Get("callback")
			status := postCallback(t, port, tc.origin,
				`{"apiKey":"planted-key","userName":"evil","userId":"e-1"}`)
			if status != http.StatusForbidden {
				t.Fatalf("expected 403, got %d", status)
			}

			// The wait must not be satisfied by the rejected callback.
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			if _, _, err := session.Wait(ctx); err == nil {
				t.Fatal("Wait must not succeed for a rejected callback")
			}
		})
	}
}

// The devstation login page nests the payload under "result".
func TestStartLoginWithNestedResult(t *testing.T) {
	session, err := StartLogin("https://devstation.connect.huaweicloud.com")
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	defer session.Close()

	u, _ := url.Parse(session.URL)
	port := u.Query().Get("callback")
	body := strings.NewReader(`{"error_code":"0000","error_msg":"success","result":{"apiKey":"nested-key-9","userName":"hw067826022","userId":"u-2"}}`)
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+port+"/", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://devstation.connect.huaweicloud.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()

	cred, ui, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if cred.APIKey != "nested-key-9" {
		t.Fatalf("expected nested-key-9, got %q", cred.APIKey)
	}
	if ui == nil || ui.UserName != "hw067826022" || ui.UserID != "u-2" {
		t.Fatalf("expected user info lifted from result, got %+v", ui)
	}
}

func TestCallbackDebugLogging(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	session, err := StartLogin("https://devstation.connect.huaweicloud.com")
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	defer session.Close()

	u, _ := url.Parse(session.URL)
	port := u.Query().Get("callback")
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+port+"/",
		strings.NewReader(`{"apiKey":"callback-key-42","userName":"tester","userId":"u-1"}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://devstation.connect.huaweicloud.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()

	cred, ui, err := session.Wait(context.Background())
	if err != nil || cred.APIKey != "callback-key-42" {
		t.Fatalf("wait: key=%q err=%v", cred.APIKey, err)
	}
	if ui == nil || ui.UserName != "tester" {
		t.Fatalf("expected user info from the callback, got %+v", ui)
	}
}

func TestLoginPageURL(t *testing.T) {
	got := loginPageURL("https://devstation.connect.huaweicloud.com/", 12345)
	for _, want := range []string{
		"https://devstation.connect.huaweicloud.com/space/devbridge/redirect",
		"origin=devbox",
		"callback=12345",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("loginPageURL = %q, want to contain %q", got, want)
		}
	}
	if !strings.Contains(got, "language=") {
		t.Fatalf("loginPageURL = %q, want language param", got)
	}
}

func TestSystemLang(t *testing.T) {
	lang := systemLang()
	if lang != "zh-cn" && lang != "en-us" {
		t.Fatalf("systemLang = %q, want zh-cn or en-us", lang)
	}
}
