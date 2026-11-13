// Package auth implements credential storage, validation and browser-based
// login for the devbox CLI.
//
// the API key is kept in the OS keyring
// (github.com/zalando/go-keyring) and falls back to the devbox config file
// (~/.devbox/config.json) when the keyring is unavailable.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cli/browser"

	"github.com/huaweicloud/devspace-devbox/cli/internal/config"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
	"github.com/huaweicloud/devspace-devbox/cli/internal/mask"
)

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

// LoginSession runs a local callback server and hands the API key back after
// the login page redirects the browser to it.
//
//	open the login page → login page POSTs the key to 127.0.0.1:<port> →
//
// the CLI stores it.
type LoginSession struct {
	// URL is the full login page address (with the local callback port).
	URL string
	// BrowserOpened is false when no default browser could be launched.
	BrowserOpened bool
	// origin is the login page origin allowed to POST to the callback.
	origin string

	srv      *http.Server
	listener net.Listener
	resultCh chan callbackBody
	errCh    chan error
}

// callbackBody is the JSON payload the login page posts back. The key may be
// delivered either at the top level or nested under `result` .
type callbackBody struct {
	ErrorCode string `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
	APIKey    string `json:"apiKey"`
	UserName  string `json:"userName"`
	UserID    string `json:"userId"`
	Result    *struct {
		APIKey   string `json:"apiKey"`
		UserName string `json:"userName"`
		UserID   string `json:"userId"`
	} `json:"result"`
}

var (
	errMissingAPIKey = errors.New(i18n.T(i18n.Msg.Auth.MissingAPIKey))
	errLoginTimeout  = errors.New(i18n.T(i18n.Msg.Auth.LoginTimeout))
)

const (
	// loginOriginParam tells the devstation login page which product is
	// logging in.
	loginOriginParam = "devbox"

	// loginPagePath is the devstation portal's login redirect route; the
	// portal distinguishes products via the origin parameter.
	loginPagePath = "/space/devbridge/redirect"

	// loginSuccessCode is the success error_code the login page reports in
	// the callback body.
	loginSuccessCode = "0000"
)

// StartLogin begins browser login against the given login page base URL. It
// binds a local callback server, opens the default browser when available and
// returns the session. Call Close to release the listener, Wait to block for
// the API key.
func StartLogin(loginBase string) (*LoginSession, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Auth.BindCallbackServer), err)
	}
	port := 0
	if addr, ok := listener.Addr().(*net.TCPAddr); ok {
		port = addr.Port
	}

	s := &LoginSession{
		URL:           loginPageURL(loginBase, port),
		BrowserOpened: true,
		origin:        strings.TrimRight(loginBase, "/"),
		listener:      listener,
		resultCh:      make(chan callbackBody, 1),
		errCh:         make(chan error, 1),
	}
	s.srv = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.handleCallback(w, r)
		}),
	}
	go func() {
		if err := s.srv.Serve(listener); err != nil &&
			!errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			s.errCh <- err
		}
	}()

	if err := browser.OpenURL(s.URL); err != nil {
		s.BrowserOpened = false
	}
	return s, nil
}

// Wait blocks until the login page delivers the callback payload (API key and
// account metadata), an error occurs or ctx is done (recommend
// auth.LoginTimeout).
func (s *LoginSession) Wait(ctx context.Context) (Credential, *UserInfo, error) {
	select {
	case cb := <-s.resultCh:
		cred := Credential{APIKey: cb.APIKey}
		var ui *UserInfo
		if cb.UserName != "" || cb.UserID != "" {
			ui = &UserInfo{UserName: cb.UserName, UserID: cb.UserID}
		}
		return cred, ui, nil
	case err := <-s.errCh:
		return Credential{}, nil, err
	case <-ctx.Done():
		return Credential{}, nil, errLoginTimeout
	}
}

// Close shuts down the callback server and frees the port.
func (s *LoginSession) Close() {
	if s.srv != nil {
		_ = s.srv.Close()
	}
}

func (s *LoginSession) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Fail closed: only the login page origin may drive this callback. A
	// browser always attaches Origin on a cross-origin POST, so a missing or
	// mismatched header means the request does not come from the login page
	// (e.g. a web page trying to plant its own API key).
	if reqOrigin := r.Header.Get("Origin"); reqOrigin != s.origin {
		slog.Warn("login callback rejected: origin mismatch",
			"remote", r.RemoteAddr,
			"origin", reqOrigin,
		)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(i18n.T(i18n.Msg.Auth.CallbackForbidden)))
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", s.origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, i18n.T(i18n.Msg.Auth.CallbackNoMethod), http.StatusMethodNotAllowed)
		return
	}

	var cb callbackBody
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
	if err != nil || json.Unmarshal(data, &cb) != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(i18n.T(i18n.Msg.Auth.CallbackBadBody)))
		s.errCh <- errMissingAPIKey
		return
	}

	// Callback data log: prints the full body (apiKey masked); the clear text is never output.
	slog.Debug("login callback: body received",
		"remote", r.RemoteAddr,
		"body", maskBodyKeys(data),
	)
	// When the login page reports a business error code (e.g. the API key limit is
	// exceeded), direct the user to the API key management page.
	if cb.ErrorCode != "" && cb.ErrorCode != loginSuccessCode {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(i18n.T(i18n.Msg.Auth.CallbackLoginFail)))
		s.errCh <- fmt.Errorf(i18n.T(i18n.Msg.Auth.CallbackBizError),
			cb.ErrorCode, cb.ErrorMsg, apikeyPageURL())
		return
	}

	key := cb.APIKey
	if key == "" && cb.Result != nil {
		key = cb.Result.APIKey
		// The user info may be nested under result; lift it to the top level for the reply.
		cb.UserName = cb.Result.UserName
		cb.UserID = cb.Result.UserID
	}
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(i18n.T(i18n.Msg.Auth.CallbackMissingKey)))
		s.errCh <- errMissingAPIKey
		return
	}

	// "OK" is a fixed success token the login page expects; do not translate.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
	cb.APIKey = key
	s.resultCh <- cb
}

// maskBodyKeys renders the callback body as JSON with every apiKey field
// masked, so the full browser payload is visible without leaking the key.
func maskBodyKeys(data []byte) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return string(data)
	}
	maskJSONKeys(m)
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(out)
}

// maskJSONKeys masks apiKey values in a decoded JSON object, recursively.
func maskJSONKeys(m map[string]any) {
	for k, v := range m {
		if k == "apiKey" {
			if s, ok := v.(string); ok {
				m[k] = mask.Key(s)
			}
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			maskJSONKeys(sub)
		}
	}
}

// loginPageURL builds the devstation login page URL with the local callback
// port: {base}/space/devbridge/redirect with origin/language/callback
// parameters (the portal's shared login route; origin=devbox selects DevBox).
func loginPageURL(loginBase string, port int) string {
	params := url.Values{}
	params.Set("origin", loginOriginParam)
	params.Set("language", systemLang())
	params.Set("callback", fmt.Sprintf("%d", port))
	return strings.TrimRight(loginBase, "/") + loginPagePath + "?" + params.Encode()
}

// apikeyPageURL builds the API key management page URL on the login portal
// ({LoginURL}/space/devbox/apikey), shown in login error hints.
func apikeyPageURL() string {
	return strings.TrimRight(config.LoginURL, "/") + config.ApikeyPagePath
}

// BrowserLogin opens the login page, waits for the API key callback and stores the credentials after verification. Both the login page
// and the check address come from the build-time config (config.LoginURL / config.DefaultServer).
func BrowserLogin(ctx context.Context) error {
	loginBase := config.LoginURL
	if loginBase == "" {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Auth.LoginURLNotSet))
	}
	session, err := StartLogin(loginBase)
	if err != nil {
		return err
	}
	defer session.Close()

	if session.BrowserOpened {
		fmt.Println(i18n.T(i18n.Msg.Auth.BrowserOpened))
	} else {
		// Without a browser, fail immediately and direct the user to the API
		// key management page.
		return fmt.Errorf(i18n.T(i18n.Msg.Auth.NoBrowserHint), apikeyPageURL())
	}

	loginCtx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()
	cred, userInfo, err := session.Wait(loginCtx)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Auth.LoginFailed)+"%w", err)
	}

	if err := VerifyAPIKey(ctx, config.DefaultServer, cred.APIKey); err != nil {
		return LoginVerifyError(i18n.T(i18n.Msg.Auth.LoginKeyInvalid), err)
	}
	if err := StoreCredential(&cred, userInfo); err != nil {
		return err
	}
	fmt.Println(i18n.T(i18n.Msg.Auth.LoginSuccess))
	return nil
}

// LoginVerifyError wraps verification failures: 401 gets a unified message.
func LoginVerifyError(prefix string, err error) error {
	if IsUnauthorized(err) {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Auth.LoginAPIKeyInvalid))
	}
	return fmt.Errorf(prefix+"%w", err)
}
