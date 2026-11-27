/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/mask"
)

// maxLoggedBody bounds how many body bytes are written to the debug log;
// larger bodies are truncated in the log but delivered intact.
const maxLoggedBody = 2048

// maxBodyForLog bounds how large a body may be buffered for logging; bigger
// bodies stream through untouched and are logged as a placeholder, so a huge
// payload can never balloon the memory footprint of a -v run.
const maxBodyForLog = 1 << 20 // 1 MiB

// loggingTransport logs API requests and responses at debug level so that
// `-v` runs show the full HTTP traffic: request method/URL/body and response
// bodies. The API key is never logged in clear text.
type loggingTransport struct {
	base http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	t.logRequest(req)
	resp, err := t.base.RoundTrip(req)
	t.logResponse(req, resp, start, err)
	return resp, err
}

// logRequest emits the debug log for an outgoing request, buffering small
// bodies so the round trip still sees the full payload.
func (t *loggingTransport) logRequest(req *http.Request) {
	if !slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		return
	}
	attrs := []any{
		"method", req.Method,
		"url", req.URL.String(),
	}
	if req.Body != nil && req.ContentLength <= maxBodyForLog {
		if data, err := io.ReadAll(req.Body); err == nil {
			_ = req.Body.Close()
			req.Body = io.NopCloser(bytes.NewReader(data))
			if len(data) > 0 {
				attrs = append(attrs, "body", truncateBody(maskSensitiveKeys(string(data)), len(data)))
			}
		}
	} else if req.Body != nil {
		attrs = append(attrs, "body", fmt.Sprintf("<omitted: %d bytes>", req.ContentLength))
	}
	if key := req.Header.Get(headerAPIKey); key != "" {
		attrs = append(attrs, "x_api_key", mask.Key(key))
	}
	slog.DebugContext(req.Context(), "devbox api request", attrs...)
}

// logResponse emits the debug log for a completed round trip, buffering small
// response bodies so the caller still sees the full payload.
func (t *loggingTransport) logResponse(req *http.Request, resp *http.Response, start time.Time, err error) {
	if !slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		return
	}
	attrs := []any{
		"method", req.Method,
		"url", req.URL.String(),
		"elapsed_ms", time.Since(start).Milliseconds(),
	}
	if key := req.Header.Get(headerAPIKey); key != "" {
		attrs = append(attrs, "x_api_key", mask.Key(key))
	}
	if err != nil {
		slog.DebugContext(req.Context(), "devbox api request failed", append(attrs, "error", err)...)
		return
	}

	if resp.Body != nil && resp.ContentLength <= maxBodyForLog {
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(data))
		if readErr == nil {
			attrs = append(attrs, "body", truncateBody(maskSensitiveKeys(string(data)), len(data)))
		} else {
			attrs = append(attrs, "body", fmt.Sprintf("<unreadable: %v>", readErr))
		}
	} else if resp.Body != nil {
		attrs = append(attrs, "body", fmt.Sprintf("<omitted: %d bytes>", resp.ContentLength))
	}

	slog.DebugContext(req.Context(), "devbox api response",
		append(attrs,
			"status", resp.StatusCode,
			"content_length", resp.ContentLength,
		)...)
}

// truncateBody bounds a body string for the debug log.
func truncateBody(body string, total int) string {
	if len(body) > maxLoggedBody {
		return body[:maxLoggedBody] + fmt.Sprintf("... (%d bytes total)", total)
	}
	return body
}

// sensitiveBodyKeys lists the JSON body fields that carry connection
// credentials; their values are masked before any body reaches the debug log
// (the envd relay cookie and the X-API-Key header are masked separately).
var sensitiveBodyKeys = map[string]struct{}{
	"connectToken":    {},
	"envdAccessToken": {},
}

// maskSensitiveKeys masks credential fields inside a JSON body (objects and
// arrays, recursively) so `-v` runs never log a connect token in clear text.
// Bodies that are not valid JSON are returned unchanged; the caller's body
// string is only used for logging, so re-marshaling never affects behavior.
func maskSensitiveKeys(body string) string {
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return body
	}
	out, err := json.Marshal(maskJSONValue(v))
	if err != nil {
		return body
	}
	return string(out)
}

// maskJSONValue walks a decoded JSON value and masks the string values of
// sensitive keys (see sensitiveBodyKeys) at any nesting depth.
func maskJSONValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return maskJSONMap(t)
	case []any:
		for i, item := range t {
			t[i] = maskJSONValue(item)
		}
		return t
	default:
		return v
	}
}

// maskJSONMap masks sensitive string values of a decoded JSON object and
// recurses into nested containers.
func maskJSONMap(m map[string]any) map[string]any {
	for k, item := range m {
		if _, sensitive := sensitiveBodyKeys[k]; sensitive {
			if s, ok := item.(string); ok && s != "" {
				m[k] = mask.Key(s)
			}
			continue
		}
		m[k] = maskJSONValue(item)
	}
	return m
}
