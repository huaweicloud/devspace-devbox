package client

import (
	"bytes"
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
	if slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		attrs := []any{
			"method", req.Method,
			"url", req.URL.String(),
		}
		if req.Body != nil && req.ContentLength <= maxBodyForLog {
			if data, err := io.ReadAll(req.Body); err == nil {
				_ = req.Body.Close()
				req.Body = io.NopCloser(bytes.NewReader(data))
				if len(data) > 0 {
					attrs = append(attrs, "body", truncateBody(string(data), len(data)))
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

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	if !slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		return resp, err
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
		return resp, err
	}

	if resp.Body != nil && resp.ContentLength <= maxBodyForLog {
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(data))
		if readErr == nil {
			attrs = append(attrs, "body", truncateBody(string(data), len(data)))
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
	return resp, err
}

// truncateBody bounds a body string for the debug log.
func truncateBody(body string, total int) string {
	if len(body) > maxLoggedBody {
		return body[:maxLoggedBody] + fmt.Sprintf("... (%d bytes total)", total)
	}
	return body
}
