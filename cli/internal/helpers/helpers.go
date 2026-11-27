// Package helpers provides shared command-line utilities: input validation,
// POSIX shell quoting, stdin detection and sandbox terminal/exec session
// helpers, kept free of cobra command wiring.
package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/grpc/process"
	"github.com/huaweicloud/devspace-devbox/cli/internal/api/rest/manager"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
	"github.com/huaweicloud/devspace-devbox/cli/internal/terminal"
)

// ValidateTimeoutSeconds validates the timeout/duration input: the valid range is 10-86400 seconds,
// an explicit 0 is invalid (not conflated with omission).
func ValidateTimeoutSeconds(v int32) error {
	if v < 10 || v > 86400 {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidTimeout), v)
	}
	return nil
}

// ValidateSandboxID validates the sandboxID input: lowercase letters, digits and hyphens, 36 characters.
func ValidateSandboxID(s string) error {
	if len(s) != 36 {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidSandboxID), s)
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidSandboxID), s)
		}
	}
	return nil
}

// ValidateTemplateID validates the templateID input: lowercase letters, digits and hyphens,
// at most 64 characters, and must not be empty.
func ValidateTemplateID(s string) error {
	if s == "" || len(s) > 64 {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidTemplateID), s)
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidTemplateID), s)
		}
	}
	return nil
}

// ParseStates parses the repeatable --state values and validates them.
func ParseStates(values []string) ([]manager.SandboxState, error) {
	states := make([]manager.SandboxState, 0, len(values))
	for _, s := range values {
		st := manager.SandboxState(s)
		if !st.Valid() {
			return nil, fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidState), s)
		}
		states = append(states, st)
	}
	return states, nil
}

// ParseKV parses repeated key=value arguments into a map; split on the first "=",
// the value may contain further "=" characters.
func ParseKV(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf(i18n.T(i18n.Msg.Parse.ExpectedKeyValue), p)
		}
		out[k] = v
	}
	return out, nil
}

// ParseMetadata parses the --metadata argument; empty returns nil.
func ParseMetadata(pairs []string) (*map[string]string, error) {
	m, err := ParseKV(pairs)
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	return &m, nil
}

// StrFromPtr converts a nullable string to a display value.
func StrFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// SandboxJSON renders the sandbox as a JSON-ready map with the sensitive
// fields (connectToken, tokenExpiration) removed, so `info -f json` never
// leaks credentials.
func SandboxJSON(sb *manager.Sandbox) map[string]any {
	data, err := json.Marshal(sb)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	delete(m, "connectToken")
	delete(m, "tokenExpiration")
	return m
}

// unsafeShellChar matches the characters that need shell quoting (aligned with e2b's UNSAFE_SHELL_CHAR).
var unsafeShellChar = regexp.MustCompile(`[^\w@%+=:,./-]`)

// ShellQuote quotes a single argument per POSIX rules.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if unsafeShellChar.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}
	return s
}

// PipedStdin reports whether the local stdin is a pipe/FIFO/socket/redirected file (aligned with e2b's
// isPipedStdin); an interactive terminal (character device) returns false.
func PipedStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return false
	}
	return info.Mode()&(os.ModeNamedPipe|os.ModeSocket) != 0 || info.Mode().IsRegular()
}

// BuildRemoteCommand assembles the remote command (aligned with e2b's buildCommand): a single argument is used
// as-is; multiple arguments are shell-quoted per POSIX rules and joined, ultimately executed via /bin/bash -c.
func BuildRemoteCommand(args []string) (string, error) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return "", fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingCommand))
	}
	if len(args) == 1 {
		return args[0], nil
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = ShellQuote(a)
	}
	return strings.Join(quoted, " "), nil
}

// MissingDomainError returns the localized error for a connect response that
// carries no sandbox domain.
func MissingDomainError() error {
	return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingDomain))
}

// TerminalSessionOptions bundles the parameters of one interactive terminal
// session: the sandbox connection info plus the per-session working directory,
// user and environment. A nil Probe skips stream-failure arbitration (see
// translateSessionError).
type TerminalSessionOptions struct {
	Sandbox *manager.Sandbox
	Cwd     string
	User    string
	Envs    map[string]string
	Probe   SandboxProbe
}

// RunTerminalSession validates the connection info from the sandbox response
// and opens the interactive terminal, printing the connecting/closing notices
// to msgs. The raw session I/O flows through in/out (typically os.Stdin and
// os.Stdout). A non-empty opts.User opens the terminal as that sandbox user.
// When the session stream fails mid-flight, opts.Probe arbitrates the sandbox
// state so a TTL-expired/destroyed sandbox yields a friendly notice instead of
// a raw connect protocol error (mirrors e2b's health-check arbitration).
func RunTerminalSession(ctx context.Context, msgs io.Writer, in, out *os.File, opts TerminalSessionOptions) error {
	sb := opts.Sandbox
	if sb == nil || sb.Domain == nil || *sb.Domain == "" {
		return MissingDomainError()
	}
	if sb.ConnectToken == nil || *sb.ConnectToken == "" {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingConnectToken))
	}
	pc, err := client.NewProcessClient(client.SandboxConnection{
		Domain:       *sb.Domain,
		SandboxID:    sb.SandboxID,
		ConnectToken: *sb.ConnectToken,
		User:         opts.User,
	}, nil)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(msgs, i18n.T(i18n.Msg.Sandbox.ConnectingTo)+"\n", sb.SandboxID)
	end, err := terminal.New(pc, in, out, opts.Cwd, opts.Envs).Run(ctx)
	if err != nil {
		return translateSessionError(ctx, msgs, sb.SandboxID, err, opts.Probe)
	}
	if end != nil && end.ExitCode != 0 {
		_, _ = fmt.Fprintf(msgs, i18n.T(i18n.Msg.Sandbox.RemoteShellExited)+"\n", end.ExitCode, end.Status)
	}
	_, _ = fmt.Fprintf(msgs, i18n.T(i18n.Msg.Sandbox.ClosingConnection)+"\n", sb.SandboxID)
	return nil
}

// SandboxProbe queries the Manager for a sandbox's current state by ID. It is
// used to arbitrate transport-level stream failures, mirroring e2b's SDK which
// follows a failed envd request with a health probe: whether the sandbox is
// gone can only be told by asking the Manager.
type SandboxProbe func(ctx context.Context, sandboxID string) (*manager.Sandbox, error)

// translateSessionError arbitrates a failed terminal/exec session. When the
// sandbox is destroyed mid-session (TTL expiry or a kill), the gateway cuts
// the stream and the connect client surfaces a meaningless transport error
// (e.g. "protocol error: promised ... bytes in enveloped message"). Probing
// the Manager translates that into a friendly notice and a clean session end:
// NotFound means the sandbox was destroyed, a paused state means it was
// auto-paused on timeout; any other outcome (sandbox still running, probe
// failed) keeps the original error. A normal remote exit (*ExitCodeError) is
// never translated.
func translateSessionError(ctx context.Context, msgs io.Writer, sandboxID string, err error, probe SandboxProbe) error {
	if err == nil || probe == nil {
		return err
	}
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return err
	}
	sb, probeErr := probe(ctx, sandboxID)
	if probeErr != nil {
		var apiErr *client.APIError
		if errors.As(probeErr, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			_, _ = fmt.Fprintf(msgs, i18n.T(i18n.Msg.Sandbox.SessionEndedSandboxGone)+"\n", sandboxID)
			return nil
		}
		return err
	}
	if sb != nil && sb.State == manager.Paused {
		_, _ = fmt.Fprintf(msgs, i18n.T(i18n.Msg.Sandbox.SessionEndedSandboxPaused)+"\n", sandboxID)
		return nil
	}
	return err
}

// ExitCodeError carries a process exit code up to main so that remote exit
// codes survive without os.Exit (which would skip deferred cleanup). It is
// not printed to the user.
type ExitCodeError struct {
	ExitCode int
}

// Error implements the error interface.
func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit status %d", e.ExitCode)
}

// ExecOptions bundles the I/O and arbitration parameters of one exec session:
// where remote output goes, whether stdin is piped, and how to arbitrate a
// mid-flight stream failure (see translateSessionError).
type ExecOptions struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Piped     bool
	SandboxID string
	Probe     SandboxProbe
}

// execEventHandler processes one process event for the foreground exec: on
// the first start frame it starts the signal/stdin forwarders, output frames
// are forwarded and the final exit code is captured. ended reports whether
// the event carried the process exit.
func execEventHandler(
	ctx context.Context, pc *client.ProcessClient, body *process.ProcessEvent, started *bool, opts ExecOptions,
) (int32, bool) {
	switch e := body.GetEvent().(type) {
	case *process.ProcessEvent_Start:
		pid := e.Start.GetPid()
		if *started {
			return 0, false
		}
		*started = true
		startSignalForwarder(ctx, pc, pid)
		if opts.Piped {
			go streamLocalStdin(ctx, pc, pid)
		}
	case *process.ProcessEvent_Data:
		writeProcessEventOutput(e.Data, opts.Stdout, opts.Stderr)
	case *process.ProcessEvent_End:
		if e.End.GetError() != "" {
			_, _ = fmt.Fprintln(opts.Stderr, e.End.GetError())
		}
		return e.End.GetExitCode(), true
	case *process.ProcessEvent_Keepalive:
	}
	return 0, false
}

// ExecForeground runs in the foreground: forwards the remote stdout/stderr locally, waits for the process to exit
// and returns the remote exit code as the local exit code. A mid-flight stream
// failure is arbitrated via opts.Probe (see translateSessionError).
func ExecForeground(ctx context.Context, pc *client.ProcessClient, stream *connect.ServerStreamForClient[process.StartResponse], opts ExecOptions) error {
	defer func() { _ = stream.Close() }()

	started := false
	exitCode := int32(1)
	for stream.Receive() {
		body := stream.Msg().GetEvent()
		if body == nil {
			continue
		}
		if code, ended := execEventHandler(ctx, pc, body, &started, opts); ended {
			exitCode = code
		}
	}
	if err := stream.Err(); err != nil {
		return translateSessionError(ctx, opts.Stderr, opts.SandboxID, err, opts.Probe)
	}
	return &ExitCodeError{ExitCode: int(exitCode)}
}

// ExecBackground runs in the background: waits for the first start event to get the PID, forwards piped input (if any),
// prints the PID and exits immediately; the remote process keeps running. A mid-flight
// stream failure is arbitrated via opts.Probe (see translateSessionError).
func ExecBackground(ctx context.Context, pc *client.ProcessClient, stream *connect.ServerStreamForClient[process.StartResponse], opts ExecOptions) error {
	var pid uint32
	for stream.Receive() {
		body := stream.Msg().GetEvent()
		if body == nil {
			continue
		}
		if e, ok := body.GetEvent().(*process.ProcessEvent_Start); ok {
			pid = e.Start.GetPid()
			break
		}
	}
	if err := stream.Err(); err != nil {
		return translateSessionError(ctx, opts.Stderr, opts.SandboxID, err, opts.Probe)
	}
	if pid == 0 {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.NoProcessID))
	}
	if opts.Piped {
		streamLocalStdin(ctx, pc, pid)
	}
	_ = stream.Close()
	_, _ = fmt.Fprintln(opts.Stderr, pid)
	return nil
}

// startSignalForwarder forwards local SIGINT/SIGTERM as a remote SIGKILL (aligned with e2b):
// after the remote process is terminated the event stream receives an end event and the local side exits accordingly.
func startSignalForwarder(ctx context.Context, pc *client.ProcessClient, pid uint32) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(sigCh)
		select {
		case <-sigCh:
			killCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = pc.SendSignal(killCtx, pid, process.Signal_SIGNAL_SIGKILL)
		case <-ctx.Done():
		}
	}()
}

// streamLocalStdin forwards the local piped stdin to the remote process stdin in 64KB chunks,
// sending CloseStdin after EOF (aligned with e2b's sendStdin). Errors are reported
// on stderr; a not-found remote process only gets the dedicated exit hint.
func streamLocalStdin(ctx context.Context, pc *client.ProcessClient, pid uint32) {
	buf := make([]byte, 64*1024)
	for {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 && !sendStdinChunk(ctx, pc, pid, buf[:n]) {
			return
		}
		if readErr != nil {
			break
		}
	}
	if err := pc.CloseStdin(ctx, pid); err != nil && !client.IsNotFound(err) {
		_, _ = fmt.Fprintf(os.Stderr, i18n.T(i18n.Msg.Sandbox.CloseStdinFailed)+"\n", err)
	}
}

// sendStdinChunk forwards one stdin chunk to the remote process, reporting a
// failure on stderr; it returns false when forwarding must stop (the remote
// process is gone or the send failed for another reason).
func sendStdinChunk(ctx context.Context, pc *client.ProcessClient, pid uint32, chunk []byte) bool {
	err := pc.SendInput(ctx, pid, chunk)
	if err == nil {
		return true
	}
	if client.IsNotFound(err) {
		_, _ = fmt.Fprintln(os.Stderr, i18n.T(i18n.Msg.Sandbox.StdinAfterExit))
	} else {
		_, _ = fmt.Fprintf(os.Stderr, i18n.T(i18n.Msg.Sandbox.StdinForwardFailed)+"\n", err)
	}
	return false
}

// writeProcessEventOutput writes the remote output frames to the local stdout/stderr.
func writeProcessEventOutput(data *process.ProcessEvent_DataEvent, stdout, stderr io.Writer) {
	if data == nil {
		return
	}
	switch out := data.GetOutput().(type) {
	case *process.ProcessEvent_DataEvent_Stdout:
		_, _ = stdout.Write(out.Stdout)
	case *process.ProcessEvent_DataEvent_Stderr:
		_, _ = stderr.Write(out.Stderr)
	case *process.ProcessEvent_DataEvent_Pty:
		_, _ = stdout.Write(out.Pty)
	}
}
