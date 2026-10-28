// Package envd is a minimal client for the EnvD data plane of one sandbox,
// reachable through the relay gateway at the sandbox's domain. It covers the
// process/terminal API (/process.Process/*) used by the interactive
// `devbox sandbox connect` terminal.
//
// Wire format follows the ConnectRPC JSON protocol documented in
// docs/api/index.md: unary calls use application/json, server-streaming calls
// use application/connect+json with 5-byte framed messages
// ([1 byte flags][4 bytes big-endian length][JSON payload]).
//
// Every request is authenticated with the manager-issued connectToken via the
// relay_token Cookie plus the E2B-Sandbox-Id header (docs/api/index.md).
package envd

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// headerCookie carries the relay connect token.
	headerCookie = "Cookie"
	// headerSandboxID carries the sandbox identifier for relay validation.
	headerSandboxID = "E2B-Sandbox-Id"
	// headerSandboxPort carries the tunnel port extracted from the domain.
	headerSandboxPort = "E2B-Sandbox-Port"
	// headerProtocolVersion is required by the ConnectRPC protocol.
	headerProtocolVersion = "Connect-Protocol-Version"
	// contentTypeUnary is used for single-response RPCs.
	contentTypeUnary = "application/json"
	// contentTypeStream is used for server-streaming RPCs.
	contentTypeStream = "application/connect+json"

	// frameFlagMessage marks an uncompressed data frame.
	frameFlagMessage = 0x00
	// frameFlagTrailer marks the end frame carrying the RPC trailer.
	frameFlagTrailer = 0x02

	// maxMessageSize bounds a single connect+json frame (16 MiB).
	maxMessageSize = 16 << 20
)

// Client talks to the EnvD data plane of one sandbox.
type Client struct {
	baseURL      string
	sandboxID    string
	connectToken string
	port         string
	httpClient   *http.Client
}

// New builds a Client for the sandbox at the given domain. The connectToken
// comes from the manager create/connect response. The httpClient may be nil;
// it then defaults to a client without an overall timeout because terminal
// streams run indefinitely.
func New(domain, sandboxID, connectToken string, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(domain) == "" {
		return nil, fmt.Errorf("envd: empty domain")
	}
	if strings.TrimSpace(connectToken) == "" {
		return nil, fmt.Errorf("envd: empty connect token")
	}
	u, err := url.Parse(domain)
	if err != nil {
		return nil, fmt.Errorf("envd: invalid domain %q: %w", domain, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("envd: invalid domain %q: expected http(s)://host", domain)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("envd: invalid domain %q: missing host", domain)
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	port := u.Port()
	if port == "" {
		port = tunnelPort(u.Hostname())
	}
	return &Client{
		baseURL:      strings.TrimRight(domain, "/"),
		sandboxID:    sandboxID,
		connectToken: connectToken,
		port:         port,
		httpClient:   httpClient,
	}, nil
}

// Port returns the tunnel port extracted from the domain (may be empty).
func (c *Client) Port() string {
	return c.port
}

// Event is one process event decoded from the stream.
type Event struct {
	// Start is set on the first event and carries the process PID.
	Start *StartEvent
	// Data carries decoded PTY output (may be nil on control events).
	Data []byte
	// End is set when the remote process exits.
	End *EndEvent
}

// StartEvent confirms the process has started and provides the PID.
type StartEvent struct {
	PID uint32
}

// EndEvent is emitted when the remote process exits.
type EndEvent struct {
	ExitCode int32
	Exited   bool
	Status   string
	Error    string
}

// StartPTY starts an interactive login shell with a PTY of the given size and
// returns the event stream plus the PID of the remote process. An empty cwd
// uses the sandbox default working directory.
func (c *Client) StartPTY(ctx context.Context, cols, rows int, cwd string) (*EventStream, uint32, error) {
	payload, err := json.Marshal(startRequest{
		Process: &processConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-i", "-l"},
			Envs: map[string]string{"TERM": "xterm-256color"},
			Cwd:  cwd,
		},
		Stdin: true,
		PTY:   &ptyOptions{Size: &ptySize{Cols: cols, Rows: rows}},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("envd: encode start body: %w", err)
	}
	stream, err := c.openStream(ctx, "Start", payload)
	if err != nil {
		return nil, 0, err
	}
	ev, err := stream.Next()
	if err != nil {
		_ = stream.Close()
		return nil, 0, fmt.Errorf("envd: read start event: %w", err)
	}
	if ev.Start == nil {
		_ = stream.Close()
		return nil, 0, fmt.Errorf("envd: expected start event, got data/end")
	}
	return stream, ev.Start.PID, nil
}

// SendInput writes bytes to the PTY of the process.
func (c *Client) SendInput(ctx context.Context, pid uint32, data []byte) error {
	return c.callUnary(ctx, "SendInput", sendInputRequest{
		Process: &processSelector{PID: pid},
		Input:   &processInput{PTY: data},
	})
}

// UpdateSize resizes the PTY window of the process.
func (c *Client) UpdateSize(ctx context.Context, pid uint32, cols, rows int) error {
	return c.callUnary(ctx, "Update", updateRequest{
		Process: &processSelector{PID: pid},
		PTY:     &ptyOptions{Size: &ptySize{Cols: cols, Rows: rows}},
	})
}

// EventStream reads framed process events from a server stream.
type EventStream struct {
	body io.ReadCloser
}

// Next reads the next event. It returns io.EOF when the stream ends cleanly;
// any other error indicates a transport or protocol failure.
func (s *EventStream) Next() (*Event, error) {
	var header [5]byte
	if _, err := io.ReadFull(s.body, header[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("envd: stream ended mid-frame: %w", err)
		}
		return nil, io.EOF
	}
	flags := header[0]
	size := binary.BigEndian.Uint32(header[1:])
	if size > maxMessageSize {
		// A non-framed body (e.g. a gateway "Bad gateway" text) reads as a
		// bogus frame header; include the body excerpt for diagnosability.
		return nil, fmt.Errorf("envd: invalid stream frame (claimed %d bytes): body starts with %s",
			size, peekExcerpt(s.body))
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(s.body, payload); err != nil {
		return nil, fmt.Errorf("envd: read frame payload: %w", err)
	}
	if flags == frameFlagTrailer {
		return nil, decodeTrailer(payload)
	}
	if flags != frameFlagMessage {
		return nil, fmt.Errorf("envd: unexpected frame flags 0x%02x", flags)
	}
	var msg wireEvent
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("envd: decode frame: %w", err)
	}
	if msg.Error != nil {
		return nil, fmt.Errorf("envd: rpc error %s: %s", msg.Error.Code, msg.Error.Message)
	}
	ev := &Event{}
	if msg.Event != nil {
		if msg.Event.Start != nil {
			ev.Start = &StartEvent{PID: msg.Event.Start.PID}
		}
		if msg.Event.Data != nil {
			// PTY output is preferred; fall back to stdout/stderr pipes.
			ev.Data = msg.Event.Data.PTY
			if ev.Data == nil {
				ev.Data = msg.Event.Data.Stdout
			}
			if ev.Data == nil {
				ev.Data = msg.Event.Data.Stderr
			}
		}
		if msg.Event.End != nil {
			ev.End = &EndEvent{
				ExitCode: msg.Event.End.ExitCode,
				Exited:   msg.Event.End.Exited,
				Status:   msg.Event.End.Status,
			}
			if msg.Event.End.Error != nil {
				ev.End.Error = *msg.Event.End.Error
			}
		}
	}
	return ev, nil
}

// Close releases the underlying HTTP body.
func (s *EventStream) Close() error {
	if s.body == nil {
		return nil
	}
	return s.body.Close()
}

// decodeTrailer interprets the end frame: {} on success or an error object.
func decodeTrailer(payload []byte) error {
	var trailer struct {
		Error *wireError `json:"error,omitempty"`
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &trailer); err != nil {
			return fmt.Errorf("envd: decode trailer: %w", err)
		}
	}
	if trailer.Error != nil {
		return fmt.Errorf("envd: stream error %s: %s", trailer.Error.Code, trailer.Error.Message)
	}
	return io.EOF
}

// openStream issues a server-streaming RPC and wraps the response body.
func (c *Client) openStream(ctx context.Context, method string, payload []byte) (*EventStream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/process.Process/"+method, bytes.NewReader(framePayload(payload)))
	if err != nil {
		return nil, fmt.Errorf("envd: build %s request: %w", method, err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", contentTypeStream)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("envd: open %s stream: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		return nil, fmt.Errorf("envd: %s stream returned HTTP %d: %s", method, resp.StatusCode, bodyExcerpt(resp.Body))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, contentTypeStream) {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("envd: %s stream unexpected content type %q", method, ct)
	}
	return &EventStream{body: resp.Body}, nil
}

// callUnary issues a single-response RPC.
func (c *Client) callUnary(ctx context.Context, method string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("envd: encode %s body: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/process.Process/"+method, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("envd: build %s request: %w", method, err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", contentTypeUnary)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("envd: %s call: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("envd: %s returned HTTP %d: %s", method, resp.StatusCode, bodyExcerpt(resp.Body))
	}
	var wire wireEvent
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if json.Unmarshal(data, &wire) == nil && wire.Error != nil {
		return fmt.Errorf("envd: %s error %s: %s", method, wire.Error.Code, wire.Error.Message)
	}
	return nil
}

// setHeaders applies the relay auth and ConnectRPC protocol headers.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set(headerCookie, "relay_token="+c.connectToken)
	req.Header.Set(headerSandboxID, c.sandboxID)
	if c.port != "" {
		req.Header.Set(headerSandboxPort, c.port)
	}
	req.Header.Set(headerProtocolVersion, "1")
}

// framePayload wraps a JSON message in a connect+json frame:
// [flags 0x00][4-byte big-endian length][payload].
func framePayload(payload []byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = frameFlagMessage
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

// bodyExcerpt returns a trimmed excerpt of an error response body.
func bodyExcerpt(r io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(r, 512))
	msg := strings.TrimSpace(string(data))
	if len(msg) > 512 {
		msg = msg[:512] + "..."
	}
	return msg
}

// peekExcerpt reads a few bytes from r and renders them as printable text so
// protocol mismatches show what the server actually sent.
func peekExcerpt(r io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(r, 128))
	if len(data) == 0 {
		return "<empty body>"
	}
	out := make([]rune, 0, len(data))
	for _, b := range data {
		if b >= 0x20 && b < 0x7F {
			out = append(out, rune(b))
		}
	}
	if len(out) > 0 {
		return strconv.Quote(string(out))
	}
	return fmt.Sprintf("<binary: % x>", data[:min(len(data), 32)])
}

// tunnelPort extracts the relay port embedded in the first domain label,
// e.g. "6vtzwk32-49983.devbox-s2.hwtunnel.com" returns "49983".
func tunnelPort(hostname string) string {
	label := hostname
	if i := strings.Index(hostname, "."); i >= 0 {
		label = hostname[:i]
	}
	if i := strings.LastIndex(label, "-"); i >= 0 {
		if p := label[i+1:]; isDigits(p) {
			return p
		}
	}
	return ""
}

// isDigits reports whether s consists only of ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ---------- wire structs (docs/api/processes.md) ----------

type startRequest struct {
	Process *processConfig `json:"process,omitempty"`
	PTY     *ptyOptions    `json:"pty,omitempty"`
	Stdin   bool           `json:"stdin"`
}

type processConfig struct {
	Cmd  string            `json:"cmd"`
	Args []string          `json:"args,omitempty"`
	Envs map[string]string `json:"envs,omitempty"`
	Cwd  string            `json:"cwd,omitempty"`
}

type ptyOptions struct {
	Size *ptySize `json:"size,omitempty"`
}

type ptySize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type sendInputRequest struct {
	Process *processSelector `json:"process,omitempty"`
	Input   *processInput    `json:"input,omitempty"`
}

type processSelector struct {
	PID uint32 `json:"pid"`
}

type processInput struct {
	PTY []byte `json:"pty,omitempty"`
}

type updateRequest struct {
	Process *processSelector `json:"process,omitempty"`
	PTY     *ptyOptions      `json:"pty,omitempty"`
}

type wireEvent struct {
	Event *wireEventBody `json:"event,omitempty"`
	Error *wireError     `json:"error,omitempty"`
}

type wireEventBody struct {
	Start *wireStartEvent `json:"start,omitempty"`
	Data  *wireDataEvent  `json:"data,omitempty"`
	End   *wireEndEvent   `json:"end,omitempty"`
}

type wireStartEvent struct {
	PID uint32 `json:"pid"`
}

type wireDataEvent struct {
	Stdout []byte `json:"stdout,omitempty"`
	Stderr []byte `json:"stderr,omitempty"`
	PTY    []byte `json:"pty,omitempty"`
}

type wireEndEvent struct {
	ExitCode int32   `json:"exitCode"`
	Exited   bool    `json:"exited"`
	Status   string  `json:"status"`
	Error    *string `json:"error,omitempty"`
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
