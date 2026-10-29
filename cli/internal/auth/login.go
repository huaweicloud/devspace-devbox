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
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Auth.BindCallbackServer), err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	s := &LoginSession{
		URL:           loginPageURL(loginBase, port),
		BrowserOpened: true,
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
			// 回调数据日志：打印完整 body（apiKey 字段脱敏），绝不输出明文。
			slog.Debug("login callback: body received",
				"remote", r.RemoteAddr,
				"body", maskBodyKeys(data),
			)
			// 登录页回报业务错误码（如 API Key 数量超上限）时，提示前往
			// API Key 管理页面处理（对齐 devbridge）。
			if cb.ErrorCode != "" && cb.ErrorCode != loginSuccessCode {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("login failed"))
				s.errCh <- fmt.Errorf(i18n.T(i18n.Msg.Auth.CallbackBizError),
					cb.ErrorCode, cb.ErrorMsg, apikeyPageURL())
				return
			}
			key = cb.APIKey
			if key == "" && cb.Result != nil {
				key = cb.Result.APIKey
				// 用户信息可能嵌套在 result 里，提升到顶层以便回传。
				cb.UserName = cb.Result.UserName
				cb.UserID = cb.Result.UserID
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
	cb.APIKey = key
	s.resultCh <- cb
}

// maskAPIKey renders only the head and tail fragments of the key.
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
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
				m[k] = maskAPIKey(s)
			}
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			maskJSONKeys(sub)
		}
	}
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
	cred, userInfo, err := session.Wait(loginCtx)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Auth.LoginFailed)+"%w", err)
	}

	if err := VerifyAPIKey(ctx, config.AuthCheckDomain, cred.APIKey); err != nil {
		return LoginVerifyError(i18n.T(i18n.Msg.Auth.LoginKeyInvalid), err)
	}
	if err := StoreCredential(&cred, userInfo); err != nil {
		return err
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
