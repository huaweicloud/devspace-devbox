/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

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

// envdRelayCookie is the cookie name carrying the relay connect token on
// data-plane requests. Its value is a JWT credential: never logged, only its
// presence and length.
const envdRelayCookie = "relay_token"

// statusOKMin and statusOKMax bound the 2xx range for response body logging.
const (
	statusOKMin = 200
	statusOKMax = 299
)

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

// requestLogAttrs builds the request log attributes: auth user selection,
// relay token presence and the body preview. The request body is buffered
// only when its length is known and bounded: a client stream's body
// (StreamInput) is written while the request is in flight, and reading it
// would deadlock.
func requestLogAttrs(req *http.Request) []any {
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
	return attrs
}

// logErrorResponse buffers a bounded prefix of a non-2xx body for the debug
// log and prepends it to the response body, so the caller still reads the
// full body while the log carries the Connect error JSON.
func logErrorResponse(resp *http.Response, rattrs []any) []any {
	if resp.Body == nil {
		return rattrs
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxLoggedErrorBody))
	resp.Body = &prefixedReadCloser{
		Reader:   io.MultiReader(bytes.NewReader(data), resp.Body),
		original: resp.Body,
	}
	if len(data) > 0 {
		rattrs = append(rattrs, "body", truncateBody(string(data), len(data)))
	}
	return rattrs
}

func (t *envdLoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Quiet requests (interactive PTY keystrokes and resize streams) bypass
	// the debug log entirely; one log pair per keystroke would flood the
	// raw-mode terminal and make the session unusable under -v.
	if req.Context().Value(quietKey{}) != nil || !slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		return t.base.RoundTrip(req)
	}

	slog.DebugContext(req.Context(), "devbox envd request", requestLogAttrs(req)...)

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
	// Non-2xx: buffer the error body for the log; 2xx bodies are never
	// touched - a streaming response only returns at EOF, so buffering it
	// would stall the session.
	if resp.StatusCode < statusOKMin || resp.StatusCode > statusOKMax {
		rattrs = logErrorResponse(resp, rattrs)
	} else {
		rattrs = append(rattrs, "body", "<stream>")
	}
	slog.DebugContext(req.Context(), "devbox envd response", rattrs...)
	return resp, nil
}

// prefixedReadCloser prepends the buffered bytes to the original response
// body: the caller still reads the full body, and Close delegates to the
// original body.
type prefixedReadCloser struct {
	io.Reader // MultiReader(buffered prefix, original body)
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
