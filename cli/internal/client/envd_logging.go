package client

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// envdRelayCookie 是数据面请求携带的连接凭据 Cookie 名。其值是 JWT 凭据，
// 永远不落日志，只记录存在性与长度。
const envdRelayCookie = "relay_token"

// maxLoggedErrorBody bounds how many bytes of a non-2xx response body are
// captured for the debug log; the remainder still reaches the caller.
const maxLoggedErrorBody = 4096

// envdLoggingTransport logs data-plane (envd) traffic at debug level so that
// `-v` runs show which requests reached the sandbox, with which Basic-auth
// user selection and what status came back. Unlike the Manager logging
// transport it never buffers 2xx response bodies: PTY and command output
// streams flow through untouched, so logging cannot stall a live session.
// Non-2xx bodies carry the Connect error JSON and are buffered (bounded)
// for the log while remaining fully readable by the caller.
type envdLoggingTransport struct {
	base http.RoundTripper
}

func (t *envdLoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		return t.base.RoundTrip(req)
	}

	attrs := []any{
		"method", req.Method,
		"url", req.URL.String(),
	}
	if user, ok := basicAuthUser(req.Header.Get("Authorization")); ok {
		attrs = append(attrs, "auth_user", user)
	}
	if cookie, err := req.Cookie(envdRelayCookie); err == nil {
		attrs = append(attrs, "relay_token", fmt.Sprintf("present(len=%d)", len(cookie.Value)))
	}
	// 请求体只在长度已知且不大时缓冲：客户端流（StreamInput）的 body 是
	// 边发边写的管道，读它会造成死锁。
	if req.Body != nil && req.ContentLength >= 0 && req.ContentLength <= maxBodyForLog {
		if data, err := io.ReadAll(req.Body); err == nil {
			_ = req.Body.Close()
			req.Body = io.NopCloser(bytes.NewReader(data))
			if len(data) > 0 {
				attrs = append(attrs, "body", truncateBody(string(data), len(data)))
			}
		}
	} else if req.Body != nil {
		attrs = append(attrs, "body", "<streaming>")
	}
	slog.DebugContext(req.Context(), "devbox envd request", attrs...)

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	if !slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		return resp, err
	}

	rattrs := []any{
		"method", req.Method,
		"url", req.URL.String(),
		"elapsed_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		slog.DebugContext(req.Context(), "devbox envd request failed", append(rattrs, "error", err)...)
		return resp, err
	}
	rattrs = append(rattrs,
		"status", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"),
	)
	// 非 2xx：错误体（Connect 错误 JSON）缓冲进日志，剩余字节经 prefixed
	// ReadCloser 交还调用方；2xx 一律不碰 body——流式响应读到 EOF 才返回，
	// 缓冲即卡死。
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.Body != nil {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, maxLoggedErrorBody))
			resp.Body = &prefixedReadCloser{
				Reader:   io.MultiReader(bytes.NewReader(data), resp.Body),
				original: resp.Body,
			}
			if len(data) > 0 {
				rattrs = append(rattrs, "body", truncateBody(string(data), len(data)))
			}
		}
	} else {
		rattrs = append(rattrs, "body", "<stream>")
	}
	slog.DebugContext(req.Context(), "devbox envd response", rattrs...)
	return resp, nil
}

// prefixedReadCloser 把缓冲的前缀字节拼回原始响应体：调用方仍能读到完整
// body，Close 委托给原始 body。
type prefixedReadCloser struct {
	io.Reader // MultiReader(缓冲前缀, 原始 body)
	original  io.ReadCloser
}

func (p *prefixedReadCloser) Close() error {
	return p.original.Close()
}

// basicAuthUser decodes the "Authorization: Basic base64(user:pass)" header
// and returns the username part. The processes API uses an empty password to
// select the sandbox user, so the username alone is the user selection.
func basicAuthUser(header string) (string, bool) {
	const prefix = "Basic "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len(prefix):]))
	if err != nil {
		return "", false
	}
	user, _, _ := strings.Cut(string(decoded), ":")
	return user, user != ""
}
