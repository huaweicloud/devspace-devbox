package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/processgen/process"
	"github.com/huaweicloud/devspace-devbox/cli/internal/api/processgen/process/processconnect"
)

// defaultEnvdPort is the envd port baked into sandbox domains; it is also the
// value used by the relay gateway when the E2B-Sandbox-Port header is absent.
const defaultEnvdPort = 49983

// relayTokenCookie is the cookie name expected by the devbridge relay gateway.
const relayTokenCookie = "relay_token"

// relayInterceptor attaches the sandbox connection credentials defined in the
// DevBox processes API: the relay token cookie plus the sandbox id/port headers.
type relayInterceptor struct {
	sandboxID    string
	connectToken string
	sandboxPort  int
}

func (i *relayInterceptor) attach(h http.Header) {
	h.Set("Cookie", relayTokenCookie+"="+i.connectToken)
	h.Set("E2B-Sandbox-Id", i.sandboxID)
	h.Set("E2B-Sandbox-Port", strconv.Itoa(i.sandboxPort))
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
	rpc processconnect.ProcessClient
}

// NewProcessClient builds a client against the sandbox domain. The domain
// embeds the envd port (e.g. https://<tunnel>-49983.host); when it cannot be
// parsed the default 49983 is used.
func NewProcessClient(domain, sandboxID, connectToken string, httpClient *http.Client) (*ProcessClient, error) {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	u, err := url.Parse(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid sandbox domain %q: %w", domain, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid sandbox domain %q: expected http(s)://host[:port]", domain)
	}
	rpc := processconnect.NewProcessClient(
		httpClient,
		domain,
		connect.WithInterceptors(&relayInterceptor{
			sandboxID:    sandboxID,
			connectToken: connectToken,
			sandboxPort:  envdPortFromDomain(u),
		}),
	)
	return &ProcessClient{rpc: rpc}, nil
}

// envdPortFromDomain extracts the envd port from the first subdomain label of
// the sandbox domain (<tunnelId>-<port>.host), falling back to 49983.
func envdPortFromDomain(u *url.URL) int {
	labels := strings.Split(u.Hostname(), ".")
	if len(labels) == 0 {
		return defaultEnvdPort
	}
	if _, port, ok := strings.Cut(labels[0], "-"); ok {
		if n, err := strconv.Atoi(port); err == nil && n > 0 {
			return n
		}
	}
	return defaultEnvdPort
}

// Start starts a process in the sandbox and returns the event stream. The
// stream ends when the process exits (EndEvent) or the context is cancelled.
func (p *ProcessClient) Start(ctx context.Context, cfg *process.ProcessConfig, stdin bool) (*connect.ServerStreamForClient[process.StartResponse], error) {
	req := connect.NewRequest(&process.StartRequest{
		Process: cfg,
		Stdin:   &stdin,
	})
	return p.rpc.Start(ctx, req)
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
