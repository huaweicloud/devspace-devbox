// Package client is a thin, hand-reviewed wrapper around the generated
// client (internal/api). It centralizes authentication, error
// decoding and pagination meta-headers so that the CLI layers stay focused on
// command semantics rather than HTTP plumbing.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// ErrUnauthorized is returned by [VerifyAPIKey] when the API key is rejected
// by the server (HTTP 401).
var ErrUnauthorized = errors.New(i18n.T(i18n.Msg.Client.Unauthorized))

var UserAgent = "devbox-cli/dev"

const (
	// headerAPIKey is the auth header of the DevBox Manager API.
	headerAPIKey = "X-API-Key"
	// HeaderNextToken is returned by list endpoints for pagination.
	HeaderNextToken = "X-Next-Token"
)

// Client talks to the DevBox Sandbox Manager API.
type Client struct {
	api *api.ClientWithResponses
}

// APIError is an error returned by the DevBox API. It exposes the HTTP status
// code and the human-readable message so callers can branch on the failure
// cause (e.g. 401 == bad API key).
type APIError struct {
	StatusCode int
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf(i18n.T(i18n.Msg.Client.APIErrorFormat), e.StatusCode, http.StatusText(e.StatusCode), e.Message)
}

// Is reports whether the error matches the given sentinel: an APIError with
// status 401 matches ErrUnauthorized, so every unauthorized failure can be
// recognized uniformly via errors.Is regardless of which code path built it.
func (e *APIError) Is(target error) bool {
	return target == ErrUnauthorized && e.StatusCode == http.StatusUnauthorized
}

// UpgradeRequiredError is returned when the server rejects a request with
// HTTP 426 because the CLI version is below the required minimum. Retrying
// cannot succeed; the user must upgrade the CLI.
type UpgradeRequiredError struct {
	// Message is the server-provided explanation, empty when the response
	// carries no parseable message.
	Message string
}

// Error returns the server explanation followed by the one-click upgrade
// commands, falling back to the localized hint when the server sent none.
func (e *UpgradeRequiredError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = i18n.T(i18n.Msg.Client.UpgradeRequired)
	}
	return fmt.Sprintf("\n%s\n%s\n%s\n", msg, i18n.T(i18n.Msg.Client.UpdateLabel), installCommands())
}

// New builds a Client authenticated against the given server. An empty
// apiKey skips the auth header.
func New(server, apiKey string, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if httpClient.Transport == nil {
		httpClient.Transport = http.DefaultTransport
	}
	// Only wrap once: re-wrapping an already-instrumented transport would
	// duplicate log lines when the same http.Client is reused.
	if _, ok := httpClient.Transport.(*loggingTransport); !ok {
		httpClient.Transport = &loggingTransport{base: httpClient.Transport}
	}
	u, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.InvalidServerURL), server, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.InvalidServerURLScheme), server)
	}
	if u.Host == "" {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.InvalidServerURLHost), server)
	}
	apiClient, err := api.NewClientWithResponses(
		server,
		api.WithHTTPClient(httpClient),
		api.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("User-Agent", UserAgent)
			if apiKey != "" {
				req.Header.Set(headerAPIKey, apiKey)
			}
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.CreateClientFailed), err)
	}
	return &Client{api: apiClient}, nil
}

// checkStatus returns an *APIError derived from the DevBox error schema when
// the HTTP response is not a 2xx. A 426 response maps to UpgradeRequiredError.
func checkStatus(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusUpgradeRequired {
		return &UpgradeRequiredError{Message: serverMessage(body)}
	}

	apiErr := &APIError{StatusCode: resp.StatusCode}
	if msg := serverMessage(body); msg != "" {
		apiErr.Message = msg
		return apiErr
	}
	// Non-JSON or unexpected body: echo a trimmed excerpt for debuggability.
	msg := strings.TrimSpace(string(body))
	if len(msg) > 512 {
		msg = msg[:512] + "..."
	}
	apiErr.Message = i18n.T(i18n.Msg.Client.ErrorResponsePrefix) + msg
	return apiErr
}

// serverMessage extracts the human-readable message from a DevBox error body
// ({message: ...}), returning empty for non-JSON or message-less bodies.
func serverMessage(body []byte) string {
	var e api.Error
	if err := json.Unmarshal(body, &e); err == nil && e.Message != "" {
		return e.Message
	}
	return ""
}
