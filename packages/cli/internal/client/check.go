package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/huaweicloud/devspace-devbox/packages/cli/internal/config"
)

const (
	// DefaultCheckPath is the API key validation endpoint path suffix on the
	// devbridge relay controller. Combined with config.RelayControllerPath it
	// mirrors the devbridge check endpoint. Both devbridge and devbox keys are
	// validated by this endpoint.
	DefaultCheckPath = "/auth/check"
)

// checkHTTPClient is used for key verification. It is intentionally separate
// from the main API client so verification stays fast and independent.
var checkHTTPClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &loggingTransport{base: http.DefaultTransport},
}

// VerifyAPIKey validates apiKey against the devbridge relay controller check
// endpoint ({server}{RelayControllerPath}/auth/check). Both devbridge and
// devbox keys are validated there. It returns nil when the key is valid
// (HTTP 200/204), ErrUnauthorized when the key is rejected (HTTP 401), and a
// wrapped error otherwise.
func VerifyAPIKey(ctx context.Context, server, apiKey string) error {
	if apiKey == "" {
		return ErrUnauthorized
	}

	url := strings.TrimRight(server, "/") + config.RelayControllerPath + DefaultCheckPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build verify request: %w", err)
	}
	req.Header.Set(headerAPIKey, apiKey)

	resp, err := checkHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("verify api key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("verify api key: unexpected status %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
}
