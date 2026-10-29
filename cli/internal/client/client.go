// Package client is a thin, hand-reviewed wrapper around the generated
// client (internal/api). It centralises authentication, error
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

const (
	// headerAPIKey is the auth header of the DevBox Manager API.
	headerAPIKey = "X-API-Key"
	// HeaderNextToken is returned by list endpoints for pagination.
	HeaderNextToken = "X-Next-Token"
	// HeaderTotalRunning is returned by the sandbox list v2 endpoint.
	HeaderTotalRunning = "X-Total-Running"
)

// Client talks to the DevBox Sandbox Manager API.
type Client struct {
	api     *api.ClientWithResponses
	baseURL string
}

// APIError is an error returned by the DevBox API. It exposes the HTTP status
// code and the machine-readable error code from the OpenAPI Error schema so
// callers can branch on the failure cause (e.g. 401 == bad API key).
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf(i18n.T(i18n.Msg.Client.APIErrorFormat), e.StatusCode, http.StatusText(e.StatusCode), e.Message)
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
	httpClient.Transport = &loggingTransport{base: httpClient.Transport}
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
			if apiKey != "" {
				req.Header.Set(headerAPIKey, apiKey)
			}
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.CreateClientFailed), err)
	}
	return &Client{api: apiClient, baseURL: server}, nil
}

// BaseURL returns the configured server URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// checkStatus returns an *APIError derived from the DevBox error schema when
// the HTTP response is not a 2xx.
func checkStatus(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	apiErr := &APIError{StatusCode: resp.StatusCode}
	var e api.Error
	if err := json.Unmarshal(body, &e); err == nil && e.Message != "" {
		apiErr.Message = e.Message
		apiErr.Code = e.Error
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

// headerInt parses an optional integer response header, defaulting to 0.
func headerInt(resp *http.Response, name string) int64 {
	n := int64(0)
	if resp != nil {
		_, _ = fmt.Sscanf(resp.Header.Get(name), "%d", &n)
	}
	return n
}
