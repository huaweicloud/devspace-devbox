// Package auth implements credential storage, validation and browser-based
// login for the devbox CLI.
//
// the API key is kept in the OS keyring
// (github.com/zalando/go-keyring) and falls back to the devbox config file
// (~/.devbox/config.json) when the keyring is unavailable.
package auth

import "time"

const (
	// CredentialName is the keyring service name and the error sentinel scope.
	CredentialName = "DEVBOX"

	// LoginTimeout bounds how long [*LoginSession.Wait] blocks for the browser
	// callback.
	LoginTimeout = 5 * time.Minute
)

// Credential is the stored authentication material.
type Credential struct {
	APIKey string `json:"api_key" yaml:"api_key"`
}

// UserInfo carries optional account metadata returned by the login page.
type UserInfo struct {
	UserName string `json:"user_name" yaml:"user_name"`
	UserID   string `json:"user_id" yaml:"user_id"`
}
