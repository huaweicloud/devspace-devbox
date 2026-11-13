package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/processgen"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// defaultEnvdPort is the envd port baked into sandbox domains; it is also the
// value used by the relay gateway when the E2B-Sandbox-Port header is absent.
const defaultEnvdPort = 49983

// relayTokenCookie is the cookie name expected by the sandbox relay gateway.
const relayTokenCookie = "relay_token"

// relayInterceptor attaches the sandbox connection credentials defined in the
// DevBox processes API: the relay token cookie plus the sandbox id/port headers.
// A non-empty user additionally selects the sandbox user for process starts via
// HTTP Basic auth with an empty password, as the envd contract specifies.
type relayInterceptor struct {
	sandboxID    string
	connectToken string
	sandboxPort  int
	user         string
}

func (i *relayInterceptor) attach(h http.Header) {
	h.Set("Cookie", relayTokenCookie+"="+i.connectToken)
	h.Set("E2B-Sandbox-Id", i.sandboxID)
	h.Set("E2B-Sandbox-Port", strconv.Itoa(i.sandboxPort))
	if i.user != "" {
		// The username alone selects the sandbox user; the password stays empty.
		encoded := base64.StdEncoding.EncodeToString([]byte(i.user + ":"))
		h.Set("Authorization", "Basic "+encoded)
	}
}

// WrapUnary implements connect.Interceptor.
func (i *relayInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		i.attach(req.Header())
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor.
func (i *relayInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		i.attach(conn.RequestHeader())
		return conn
	}
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *relayInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// ProcessClient talks to the sandbox-local envd process.Process service via
// the sandbox domain returned by the Manager connect endpoint. Requests use
// the Connect protocol (JSON codecs); the http.Client must NOT carry a global
// timeout or long-running command streams would be cut off.
type ProcessClient struct {
	rpc process.ProcessClient
}

// NewProcessClient builds a client against the sandbox domain. The domain
// embeds the envd port (e.g. https://<tunnel>-49983.host); when it cannot be
// parsed the default 49983 is used. A non-empty user is sent via HTTP Basic
// auth on every request so envd starts processes as that sandbox user. All
// traffic is logged at debug level (see envdLoggingTransport) so `-v` runs
// show the data-plane requests and responses.
func NewProcessClient(domain, sandboxID, connectToken, user string, httpClient *http.Client) (*ProcessClient, error) {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if httpClient.Transport == nil {
		httpClient.Transport = http.DefaultTransport
	}
	// Only wrap once: re-wrapping an already-instrumented transport would
	// duplicate log lines when the same http.Client is reused.
	if _, ok := httpClient.Transport.(*envdLoggingTransport); !ok {
		httpClient.Transport = &envdLoggingTransport{base: httpClient.Transport}
	}
	u, err := url.Parse(domain)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.InvalidSandboxDomain), domain, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.InvalidDomainScheme), domain)
	}
	rpc := process.NewProcessClient(
		httpClient,
		domain,
		connect.WithInterceptors(&relayInterceptor{
			sandboxID:    sandboxID,
			connectToken: connectToken,
			sandboxPort:  envdPortFromDomain(u),
			user:         user,
		}),
		// JSON codec：与 processes API 文档对齐（application/json /
		// application/connect+json），-v 日志里请求体直接可读；
		// envd（connect-go 服务端）默认支持 JSON codec。
		connect.WithProtoJSON(),
	)
	return &ProcessClient{rpc: rpc}, nil
}

// envdPortFromDomain extracts the envd port from the first subdomain label of
// the sandbox domain (<tunnelId>-<port>.host), falling back to 49983. The
// tunnel ID may itself contain hyphens, so the label is split on the LAST
// hyphen.
func envdPortFromDomain(u *url.URL) int {
	labels := strings.Split(u.Hostname(), ".")
	if len(labels) == 0 {
		return defaultEnvdPort
	}
	label := labels[0]
	if idx := strings.LastIndex(label, "-"); idx >= 0 {
		if n, err := strconv.Atoi(label[idx+1:]); err == nil && n > 0 {
			return n
		}
	}
	return defaultEnvdPort
}

// Start starts a process in the sandbox and returns the event stream. The
// stream ends when the process exits (EndEvent) or the context is canceled.
func (p *ProcessClient) Start(ctx context.Context, cfg *process.ProcessConfig, stdin bool) (*connect.ServerStreamForClient[process.StartResponse], error) {
	req := connect.NewRequest(&process.StartRequest{
		Process: cfg,
		Stdin:   &stdin,
	})
	return p.rpc.Start(ctx, req)
}

// StartPTY starts an interactive shell with a PTY of the given size and
// returns the event stream. The first event carries the process PID.
func (p *ProcessClient) StartPTY(ctx context.Context, cfg *process.ProcessConfig, cols, rows int) (*connect.ServerStreamForClient[process.StartResponse], error) {
	stdin := true
	req := connect.NewRequest(&process.StartRequest{
		Process: cfg,
		Stdin:   &stdin,
		Pty:     &process.PTY{Size: &process.PTY_Size{Cols: uint32(cols), Rows: uint32(rows)}},
	})
	return p.rpc.Start(ctx, req)
}

// SendPtyInput writes bytes to the PTY of a running process.
func (p *ProcessClient) SendPtyInput(ctx context.Context, pid uint32, data []byte) error {
	_, err := p.rpc.SendInput(ctx, connect.NewRequest(&process.SendInputRequest{
		Process: &process.ProcessSelector{
			Selector: &process.ProcessSelector_Pid{Pid: pid},
		},
		Input: &process.ProcessInput{
			Input: &process.ProcessInput_Pty{Pty: data},
		},
	}))
	return err
}

// UpdateSize resizes the PTY of a running process.
func (p *ProcessClient) UpdateSize(ctx context.Context, pid uint32, cols, rows int) error {
	_, err := p.rpc.Update(ctx, connect.NewRequest(&process.UpdateRequest{
		Process: &process.ProcessSelector{
			Selector: &process.ProcessSelector_Pid{Pid: pid},
		},
		Pty: &process.PTY{Size: &process.PTY_Size{Cols: uint32(cols), Rows: uint32(rows)}},
	}))
	return err
}

// SendInput writes bytes to the stdin of a running process.
func (p *ProcessClient) SendInput(ctx context.Context, pid uint32, data []byte) error {
	_, err := p.rpc.SendInput(ctx, connect.NewRequest(&process.SendInputRequest{
		Process: &process.ProcessSelector{
			Selector: &process.ProcessSelector_Pid{Pid: pid},
		},
		Input: &process.ProcessInput{
			Input: &process.ProcessInput_Stdin{Stdin: data},
		},
	}))
	return err
}

// CloseStdin signals EOF to a non-PTY process so it can consume its input and exit.
func (p *ProcessClient) CloseStdin(ctx context.Context, pid uint32) error {
	_, err := p.rpc.CloseStdin(ctx, connect.NewRequest(&process.CloseStdinRequest{
		Process: &process.ProcessSelector{
			Selector: &process.ProcessSelector_Pid{Pid: pid},
		},
	}))
	return err
}

// SendSignal delivers a signal to a running process.
func (p *ProcessClient) SendSignal(ctx context.Context, pid uint32, sig process.Signal) error {
	_, err := p.rpc.SendSignal(ctx, connect.NewRequest(&process.SendSignalRequest{
		Process: &process.ProcessSelector{
			Selector: &process.ProcessSelector_Pid{Pid: pid},
		},
		Signal: sig,
	}))
	return err
}

// IsNotFound reports whether err is an envd not_found RPC error, e.g. when
// connecting to a process that already exited.
func IsNotFound(err error) bool {
	return connect.CodeOf(err) == connect.CodeNotFound
}
