// Package term wires the local terminal to a sandbox PTY: it puts the local
// terminal into raw mode, forwards keystrokes to the remote process and
// renders remote output locally, like an SSH session.
package term

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/envd"

	"golang.org/x/term"
)

// maxStartAttempts bounds StartPTY retries when the remote shell exits
// without producing any output: a gateway wake-up of a paused sandbox
// (autoResume) or a lingering previous stream can tear the first stream down.
const maxStartAttempts = 3

// Session runs one interactive terminal against a sandbox PTY.
type Session struct {
	ec   *envd.Client
	in   *os.File
	out  *os.File
	cwd  string
	envs map[string]string

	sawOutput bool
}

// New builds a Session wired to the given envd client and local files
// (typically os.Stdin and os.Stdout). An empty cwd uses the sandbox default
// working directory; envs are terminal-session environment variables.
func New(ec *envd.Client, in, out *os.File, cwd string, envs map[string]string) *Session {
	return &Session{ec: ec, in: in, out: out, cwd: cwd, envs: envs}
}

// Run starts the remote shell and pumps input/output until the shell exits
// or the context is cancelled. It returns the end event of the remote shell
// (nil when the session ended without one) and restores the terminal state
// on return.
func (s *Session) Run(ctx context.Context) (*envd.EndEvent, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	raw := false
	if term.IsTerminal(int(s.in.Fd())) {
		if old, err := term.MakeRaw(int(s.in.Fd())); err == nil {
			raw = true
			defer func() { _ = term.Restore(int(s.in.Fd()), old) }()
			_ = enableVT(s.out)
		}
	}
	_ = raw

	cols, rows := 80, 24
	if w, h, err := term.GetSize(int(s.out.Fd())); err == nil && w > 0 && h > 0 {
		cols, rows = w, h
	}

	for attempt := 1; ; attempt++ {
		s.sawOutput = false
		end, err := s.runOnce(ctx, cols, rows)
		if err != nil && !s.sawOutput && attempt < maxStartAttempts {
			// Transport errors before any output (e.g. a transient gateway
			// "Bad gateway" while the sandbox is still waking up) are
			// retryable.
			time.Sleep(time.Second)
			continue
		}
		if err != nil {
			return nil, err
		}
		if end != nil && !s.sawOutput && attempt < maxStartAttempts {
			// The shell exited without any output: the sandbox may still be
			// waking up (autoResume). Retry after a short delay.
			time.Sleep(time.Second)
			continue
		}
		return end, nil
	}
}

// runOnce starts one PTY stream and pumps I/O until it ends. It returns the
// shell's end event (nil when the stream ended without one) and a non-nil
// error for transport failures.
func (s *Session) runOnce(ctx context.Context, cols, rows int) (*envd.EndEvent, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	endCh := make(chan *envd.EndEvent, 1)

	stream, pid, err := s.ec.StartPTY(ctx, cols, rows, s.cwd, s.envs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	// Output pump: render remote PTY output until the process ends.
	go func() {
		for {
			ev, err := stream.Next()
			if err != nil {
				if err != io.EOF {
					errCh <- err
				}
				cancel()
				return
			}
			if ev.Data != nil {
				s.sawOutput = true
				if _, werr := s.out.Write(ev.Data); werr != nil {
					errCh <- werr
					cancel()
					return
				}
			}
			if ev.End != nil {
				endCh <- ev.End
				cancel()
				return
			}
		}
	}()

	// Input pump: forward keystrokes to the remote PTY.
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := s.in.Read(buf)
			if n > 0 {
				if serr := s.ec.SendInput(ctx, pid, buf[:n]); serr != nil {
					errCh <- serr
					cancel()
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Resize poller: Windows has no SIGWINCH, so poll the console size.
	go s.pollResize(ctx, pid, cols, rows)

	<-ctx.Done()
	// Small grace period so pending remote output is flushed before the
	// terminal state is restored.
	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-errCh:
		return nil, err
	case end := <-endCh:
		return end, nil
	default:
		return nil, nil
	}
}

// pollResize forwards local terminal size changes to the remote PTY.
func (s *Session) pollResize(ctx context.Context, pid uint32, cols, rows int) {
	lastCols, lastRows := cols, rows
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w, h, err := term.GetSize(int(s.out.Fd()))
			if err == nil && w > 0 && h > 0 && (w != lastCols || h != lastRows) {
				lastCols, lastRows = w, h
				if uerr := s.ec.UpdateSize(ctx, pid, w, h); uerr != nil {
					return
				}
			}
		}
	}
}
