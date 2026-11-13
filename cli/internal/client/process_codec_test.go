package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewProcessClientUsesJSONCodec 锁定数据面编解码契约：流式请求必须是
// application/connect+json（与 processes API 文档一致，-v 日志可读）。
func TestNewProcessClientUsesJSONCodec(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		// handler 立即返回 Connect 错误即可，测试只关心请求头。
		w.Header().Set("Content-Type", "application/connect+json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"code":"unimplemented","message":"test"}`))
	}))
	defer srv.Close()

	pc, err := NewProcessClient(srv.URL, "sbx_test", "token", "dev", nil)
	if err != nil {
		t.Fatalf("NewProcessClient: %v", err)
	}
	_, _ = pc.Start(context.Background(), nil, false)

	if gotContentType != "application/connect+json" {
		t.Fatalf("streaming request content type = %q, want application/connect+json", gotContentType)
	}
}
