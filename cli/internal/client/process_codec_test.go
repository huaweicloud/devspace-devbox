/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewProcessClientUsesJSONCodec locks the data-plane codec contract: a
// streaming request must be application/connect+json (aligned with the
// processes API docs, keeping -v logs readable).
func TestNewProcessClientUsesJSONCodec(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		// The handler returning a Connect error immediately is fine; the test only inspects the request header.
		w.Header().Set("Content-Type", "application/connect+json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"code":"unimplemented","message":"test"}`))
	}))
	defer srv.Close()

	pc, err := NewProcessClient(SandboxConnection{
		Domain: srv.URL, SandboxID: "sbx_test", ConnectToken: "token", User: "dev",
	}, nil)
	if err != nil {
		t.Fatalf("NewProcessClient: %v", err)
	}
	_, _ = pc.Start(context.Background(), nil, false)

	if gotContentType != "application/connect+json" {
		t.Fatalf("streaming request content type = %q, want application/connect+json", gotContentType)
	}
}
