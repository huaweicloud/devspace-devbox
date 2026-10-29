package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

const (
	// DefaultCheckPath is the API key validation endpoint path on the bridge
	// domain ({AuthCheckDomain}/v1/api-keys/check). Both devbridge and devbox
	// keys are validated by this endpoint.
	DefaultCheckPath = "/v1/api-keys/check"
)

// checkHTTPClient is used for key verification. It is intentionally separate
// from the main API client so verification stays fast and independent.
var checkHTTPClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &loggingTransport{base: http.DefaultTransport},
}

// VerifyAPIKey validates apiKey against the api-keys check endpoint
// ({server}/v1/api-keys/check). Both devbridge and devbox keys are validated
// there. It returns nil when the key is valid (HTTP 200/204), ErrUnauthorized
// when the key is rejected (HTTP 401), and a wrapped error otherwise.
func VerifyAPIKey(ctx context.Context, server, apiKey string) error {
	if apiKey == "" {
		return ErrUnauthorized
	}

	url := strings.TrimRight(server, "/") + DefaultCheckPath

	// 校验接口是 POST(GET 路由不存在)，body 为空 JSON。
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Client.BuildVerifyRequest), err)
	}
	req.Header.Set(headerAPIKey, apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := checkHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Client.VerifyFailed), err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf(i18n.T(i18n.Msg.Client.VerifyUnexpectedStatus), resp.StatusCode, http.StatusText(resp.StatusCode))
	}
}
