// Package term wires the local terminal to a sandbox PTY: it puts the local
// terminal into raw mode, forwards keystrokes to the remote process and
// renders remote output locally, like an SSH session.
package term

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/processgen/process"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"

	"golang.org/x/term"
)

// maxStartAttempts bounds StartPTY retries when the remote shell exits
// without producing any output: a gateway wake-up of a paused sandbox
// (autoResume) or a lingering previous stream can tear the first stream down.
const maxStartAttempts = 3

// Session runs one interactive terminal against a sandbox PTY.
type Session struct {
	pc   *client.ProcessClient
	in   *os.File
	out  *os.File
	cwd  string
	envs map[string]string

	sawOutput atomic.Bool
}

// New builds a Session wired to the given process client and local files
// (typically os.Stdin and os.Stdout). An empty cwd uses the sandbox default
// working directory; envs are terminal-session environment variables.
func New(pc *client.ProcessClient, in, out *os.File, cwd string, envs map[string]string) *Session {
	return &Session{pc: pc, in: in, out: out, cwd: cwd, envs: envs}
}

// Run starts the remote shell and pumps input/output until the shell exits
// or the context is cancelled. It returns the end event of the remote shell
// (nil when the session ended without one) and restores the terminal state
// on return.
func (s *Session) Run(ctx context.Context) (*process.ProcessEvent_EndEvent, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if term.IsTerminal(int(s.in.Fd())) {
		if old, err := term.MakeRaw(int(s.in.Fd())); err == nil {
			defer func() { _ = term.Restore(int(s.in.Fd()), old) }()
			_ = enableVT(s.out)
		}
	}

	cols, rows := 80, 24
	if w, h, err := term.GetSize(int(s.out.Fd())); err == nil && w > 0 && h > 0 {
		cols, rows = w, h
	}

	for attempt := 1; ; attempt++ {
		s.sawOutput.Store(false)
		end, err := s.runOnce(ctx, cols, rows)
		if err != nil && !s.sawOutput.Load() && attempt < maxStartAttempts {
			// Transport errors before any output (e.g. a transient gateway
			// "Bad gateway" while the sandbox is still waking up) are
			// retryable.
			time.Sleep(time.Second)
			continue
		}
		if err != nil {
			return nil, err
		}
		if end != nil && !s.sawOutput.Load() && attempt < maxStartAttempts {
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
func (s *Session) runOnce(ctx context.Context, cols, rows int) (*process.ProcessEvent_EndEvent, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	endCh := make(chan *process.ProcessEvent_EndEvent, 1)

	envs := s.envs
	if envs == nil {
		envs = map[string]string{}
	}
	if _, ok := envs["TERM"]; !ok {
		envs["TERM"] = "xterm-256color"
	}

	cfg := &process.ProcessConfig{
		Cmd:  "/bin/bash",
		Args: []string{"-i", "-l"},
		Envs: envs,
	}
	if s.cwd != "" {
		cfg.Cwd = &s.cwd
	}

	stream, err := s.pc.StartPTY(ctx, cfg, cols, rows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	var pid atomic.Uint32

	// Output pump: render remote PTY output until the process ends.
	go func() {
		for stream.Receive() {
			ev := stream.Msg().GetEvent()
			if ev == nil {
				continue
			}
			switch e := ev.GetEvent().(type) {
			case *process.ProcessEvent_Start:
				pid.Store(e.Start.GetPid())
			case *process.ProcessEvent_Data:
				s.sawOutput.Store(true)
				data := e.Data.GetPty()
				if len(data) == 0 {
					data = e.Data.GetStdout()
				}
				if len(data) == 0 {
					data = e.Data.GetStderr()
				}
				if len(data) > 0 {
					if _, werr := s.out.Write(data); werr != nil {
						errCh <- werr
						cancel()
						return
					}
				}
			case *process.ProcessEvent_End:
				endCh <- e.End
				cancel()
				return
			}
		}
		if err := stream.Err(); err != nil {
			errCh <- err
		}
		cancel()
	}()

	// Input pump: forward keystrokes to the remote PTY once the PID is known.
	go func() {
		for pid.Load() == 0 {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		buf := make([]byte, 512)
		for {
			n, err := s.in.Read(buf)
			if n > 0 {
				if serr := s.pc.SendPtyInput(ctx, pid.Load(), buf[:n]); serr != nil {
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
	go func() {
		for pid.Load() == 0 {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
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
					if uerr := s.pc.UpdateSize(ctx, pid.Load(), w, h); uerr != nil {
						return
					}
				}
			}
		}
	}()

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
