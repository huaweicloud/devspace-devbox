package client

import (
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnvdLoggingTransportLogsUserAndErrorBody(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"invalid_argument","message":"invalid argument: \"dev\""}`))
	}))
	defer srv.Close()

	transport := &envdLoggingTransport{base: http.DefaultTransport}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/process.Process/Start",
		strings.NewReader(`{"process":{"cmd":"/bin/bash"}}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("dev:")))
	req.Header.Set("Cookie", "relay_token=jwt-secret-value")

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "invalid_argument") {
		t.Fatalf("caller must still receive the full error body, got: %s", body)
	}

	logged := buf.String()
	if !strings.Contains(logged, "auth_user=dev") {
		t.Errorf("log should carry the decoded Basic auth user, got: %s", logged)
	}
	if strings.Contains(logged, "jwt-secret-value") {
		t.Errorf("relay token value must never be logged, got: %s", logged)
	}
	if !strings.Contains(logged, "relay_token=\"present(len=") {
		t.Errorf("log should note the relay token presence, got: %s", logged)
	}
	if !strings.Contains(logged, "status=400") {
		t.Errorf("log should carry the response status, got: %s", logged)
	}
	if !strings.Contains(logged, "invalid_argument") {
		t.Errorf("log should carry the connect error body, got: %s", logged)
	}
}

func TestEnvdLoggingTransportStreamsSuccessBodyUntouched(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 无 Content-Length 的 chunked 响应，模拟 Connect 流式帧。
		w.Header().Set("Content-Type", "application/connect+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("frame-one"))
	}))
	defer srv.Close()

	transport := &envdLoggingTransport{base: http.DefaultTransport}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/process.Process/Start", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "frame-one" {
		t.Fatalf("stream body must pass through untouched, got: %q", body)
	}
	if !strings.Contains(buf.String(), "body=<stream>") {
		t.Errorf("success responses must not be buffered, got: %s", buf.String())
	}
}

func TestBasicAuthUser(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
		wantOK bool
	}{
		{"valid user", "Basic " + base64.StdEncoding.EncodeToString([]byte("dev:")), "dev", true},
		{"case insensitive scheme", "basic " + base64.StdEncoding.EncodeToString([]byte("dev:")), "dev", true},
		{"wrong scheme", "Bearer token", "", false},
		{"not base64", "Basic %%%", "", false},
		{"empty user", "Basic " + base64.StdEncoding.EncodeToString([]byte(":pw")), "", false},
		{"empty header", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := basicAuthUser(tc.header)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("basicAuthUser(%q) = (%q, %v), want (%q, %v)", tc.header, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
