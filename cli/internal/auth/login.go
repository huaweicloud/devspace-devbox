package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/cli/browser"

	"github.com/huaweicloud/devspace-devbox/cli/internal/config"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

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

	srv      *http.Server
	listener net.Listener
	apiKeyCh chan string
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
	errMissingAPIKey = errors.New("login response did not contain an API key")
	errLoginTimeout  = errors.New("login timed out; no API key was received")
)

const (
	// loginOriginParam tells the devstation login page which product is
	// logging in. It is the only parameter that differs from devbridge.
	loginOriginParam = "devbox"

	// loginPagePath mirrors devbridge's login page route on the devstation
	// portal; the portal distinguishes products via the origin parameter.
	loginPagePath = "/space/devbridge/redirect"

	// loginSuccessCode is the success error_code the login page reports in
	// the callback body (mirrors devbridge).
	loginSuccessCode = "0000"
)

// StartLogin begins browser login against the given login page base URL. It
// binds a local callback server, opens the default browser when available and
// returns the session. Call Close to release the listener, Wait to block for
// the API key.
func StartLogin(loginBase string) (*LoginSession, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("bind login callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	s := &LoginSession{
		URL:           loginPageURL(loginBase, port),
		BrowserOpened: true,
		listener:      listener,
		apiKeyCh:      make(chan string, 1),
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

// Wait blocks until an API key is delivered by the login page, an error
// occurs or ctx is done (recommend auth.LoginTimeout).
func (s *LoginSession) Wait(ctx context.Context) (string, error) {
	select {
	case key := <-s.apiKeyCh:
		return key, nil
	case err := <-s.errCh:
		return "", err
	case <-ctx.Done():
		return "", errLoginTimeout
	}
}

// Close shuts down the callback server and frees the port.
func (s *LoginSession) Close() {
	if s.srv != nil {
		_ = s.srv.Close()
	}
}

func (s *LoginSession) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Allow cross-origin posts from the login page and CORS preflights.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	key := r.URL.Query().Get("apiKey")
	var cb callbackBody
	if key == "" {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
		if err == nil && json.Unmarshal(data, &cb) == nil {
			// 登录页回报业务错误码（如 API Key 数量超上限）时，提示前往
			// API Key 管理页面处理（对齐 devbridge）。
			if cb.ErrorCode != "" && cb.ErrorCode != loginSuccessCode {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("login failed"))
				s.errCh <- fmt.Errorf("登录失败: %s: %s\n请前往管理页面删除 API Key 后重试。API Key 管理页面： %s",
					cb.ErrorCode, cb.ErrorMsg, apikeyPageURL())
				return
			}
			key = cb.APIKey
			if key == "" && cb.Result != nil {
				key = cb.Result.APIKey
			}
		}
	}

	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing api key"))
		s.errCh <- errMissingAPIKey
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
	s.apiKeyCh <- key
}

// loginPageURL builds the devstation login page URL with the local callback
// port. The format mirrors devbridge's: {base}/space/devbridge/redirect with
// origin/language/callback parameters, only origin differs (devbox).
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

// BrowserLogin 打开登录页并等待 API Key 回调，校验通过后存储凭据。登录页与
// 校验地址均取构建期注入的配置（config.LoginURL / config.DefaultServer）。
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
		// 对齐 devbridge：无浏览器时直接报错，引导用户到 API Key 管理页面。
		return fmt.Errorf(i18n.T(i18n.Msg.Auth.NoBrowserHint), apikeyPageURL())
	}

	loginCtx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()
	key, err := session.Wait(loginCtx)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Auth.LoginFailed)+"%w", err)
	}

	if err := VerifyAPIKey(ctx, config.AuthCheckDomain, key); err != nil {
		return LoginVerifyError(i18n.T(i18n.Msg.Auth.LoginKeyInvalid), err)
	}
	ui, uiErr := LoadUserInfo()
	if uiErr != nil || ui == nil {
		if err := StoreCredential(&Credential{APIKey: key}, nil); err != nil {
			return err
		}
	} else {
		if err := StoreCredential(&Credential{APIKey: key}, ui); err != nil {
			return err
		}
	}
	fmt.Println(i18n.T(i18n.Msg.Auth.LoginSuccess))
	return nil
}

// LoginVerifyError 包装登录校验失败错误：401 时给出统一提示。
func LoginVerifyError(prefix string, err error) error {
	if IsUnauthorized(err) {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Auth.LoginAPIKeyInvalid))
	}
	return fmt.Errorf(prefix+"%w", err)
}
