package auth

import (
	"context"
	"errors"

	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
)

// VerifyAPIKey validates apiKey against the given server. It returns
// ErrUnauthorized when the key is rejected (HTTP 401) and a wrapped error for
// connectivity or other failures.
func VerifyAPIKey(ctx context.Context, server, apiKey string) error {
	if apiKey == "" {
		return client.ErrUnauthorized
	}
	return client.VerifyAPIKey(ctx, server, apiKey)
}

// IsUnauthorized reports whether err is an HTTP 401 rejection.
func IsUnauthorized(err error) bool {
	return errors.Is(err, client.ErrUnauthorized)
}
