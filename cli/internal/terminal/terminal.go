/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

// Package terminal wires the local terminal to a sandbox PTY: it puts the local
// terminal into raw mode, forwards keystrokes to the remote process and
// renders remote output locally, like an SSH session.
package terminal

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/term"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api/grpc/process"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
)

// maxStartAttempts bounds StartPTY retries when the remote shell exits
// without producing any output: a gateway wake-up of a paused sandbox
// (autoResume) or a lingering previous stream can tear the first stream down.
const maxStartAttempts = 3

// Terminal session tuning knobs: fallback console size, pump/poll intervals
// and the grace period that lets pending output flush before restore.
const (
	defaultCols        = 80
	defaultRows        = 24
	pumpInterval       = 10 * time.Millisecond
	resizePollInterval = 500 * time.Millisecond
	flushGracePeriod   = 100 * time.Millisecond
)

// enableVT enables ANSI escape sequence processing on out so remote PTY
// output renders correctly. Unix terminals process ANSI escapes by default,
// so this default is a no-op; vt_windows.go overrides it for Windows consoles.
var enableVT = func(f *os.File) error { return nil }

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
// or the context is canceled. It returns the end event of the remote shell
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

	cols, rows := defaultCols, defaultRows
	if w, h, err := term.GetSize(int(s.out.Fd())); err == nil && w > 0 && h > 0 {
		cols, rows = w, h
	}

	for attempt := 1; ; attempt++ {
		s.sawOutput.Store(false)
		end, err := s.runOnce(ctx, cols, rows)
		if err != nil && !s.sawOutput.Load() && attempt < maxStartAttempts {
			// Transport errors before any output (eg. a transient gateway
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

// ptyConfig builds the remote shell config: bash as an interactive login
// shell with TERM defaulted to xterm-256color and the optional working
// directory.
func (s *Session) ptyConfig() *process.ProcessConfig {
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
	return cfg
}

// runOnce starts one PTY stream and pumps I/O until it ends. It returns the
// shell's end event (nil when the stream ended without one) and a non-nil
// error for transport failures.
func (s *Session) runOnce(ctx context.Context, cols, rows int) (*process.ProcessEvent_EndEvent, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	endCh := make(chan *process.ProcessEvent_EndEvent, 1)

	stream, err := s.pc.StartPTY(ctx, s.ptyConfig(), cols, rows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	var pid atomic.Uint32

	// Output pump: render remote PTY output until the process ends.
	go s.pumpPTYOutput(ctx, cancel, stream, errCh, endCh, &pid)
	// Input pump: forward keystrokes to the remote PTY once the PID is known.
	go s.pumpPTYInput(ctx, cancel, errCh, &pid)
	// Resize poller: Windows has no SIGWINCH, so poll the console size.
	go s.pollPTYResize(ctx, &pid, cols, rows)

	<-ctx.Done()
	// Small grace period so pending remote output is flushed before the
	// terminal state is restored.
	time.Sleep(flushGracePeriod)

	select {
	case err := <-errCh:
		return nil, err
	case end := <-endCh:
		return end, nil
	default:
		return nil, nil
	}
}

// pumpPTYOutput renders remote PTY output until the process ends: start
// frames record the PID, data frames are rendered locally and the end frame
// terminates the session. Transport errors are reported on errCh.
func (s *Session) pumpPTYOutput(
	ctx context.Context, cancel context.CancelFunc,
	stream *connect.ServerStreamForClient[process.StartResponse],
	errCh chan<- error, endCh chan<- *process.ProcessEvent_EndEvent, pid *atomic.Uint32,
) {
	for stream.Receive() {
		ev := stream.Msg().GetEvent()
		if ev == nil {
			continue
		}
		switch e := ev.GetEvent().(type) {
		case *process.ProcessEvent_Start:
			pid.Store(e.Start.GetPid())
		case *process.ProcessEvent_Data:
			if !s.renderPTYOutput(cancel, errCh, ptyOutputFrame(e.Data)) {
				return
			}
		case *process.ProcessEvent_End:
			endCh <- e.End
			cancel()
			return
		default:
			// Keepalive and unknown frames: nothing to render.
		}
	}
	if err := stream.Err(); err != nil {
		errCh <- err
	}
	cancel()
}

// ptyOutputFrame returns the PTY payload of a data frame, falling back to
// stdout and then stderr while the PTY stream is empty.
func ptyOutputFrame(data *process.ProcessEvent_DataEvent) []byte {
	out := data.GetPty()
	if len(out) == 0 {
		out = data.GetStdout()
	}
	if len(out) == 0 {
		out = data.GetStderr()
	}
	return out
}

// renderPTYOutput writes one PTY data frame to the local terminal and marks
// real output for the start retry logic. It returns false when the local
// write failed and the session must stop.
func (s *Session) renderPTYOutput(cancel context.CancelFunc, errCh chan<- error, data []byte) bool {
	if len(data) == 0 {
		return true
	}
	// Only real output counts: an empty Data frame must not
	// suppress the start retry below.
	s.sawOutput.Store(true)
	if _, err := s.out.Write(data); err != nil {
		errCh <- err
		cancel()
		return false
	}
	return true
}

// pumpPTYInput forwards local keystrokes to the remote PTY once the PID is
// known; read errors end the pump silently (send failures are reported via
// errCh and tear the session down).
func (s *Session) pumpPTYInput(ctx context.Context, cancel context.CancelFunc, errCh chan<- error, pid *atomic.Uint32) {
	for pid.Load() == 0 {
		if ctx.Err() != nil {
			return
		}
		time.Sleep(pumpInterval)
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
}

// pollPTYResize keeps the remote PTY size in sync with the local console
// while the session runs; Windows has no SIGWINCH, so the size is polled.
func (s *Session) pollPTYResize(ctx context.Context, pid *atomic.Uint32, cols, rows int) {
	for pid.Load() == 0 {
		if ctx.Err() != nil {
			return
		}
		time.Sleep(pumpInterval)
	}
	lastCols, lastRows := cols, rows
	ticker := time.NewTicker(resizePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.applyPTYResize(ctx, pid, &lastCols, &lastRows) {
				return
			}
		}
	}
}

// applyPTYResize updates the remote PTY size when the console size changed;
// it returns false when the update failed and polling must stop.
func (s *Session) applyPTYResize(ctx context.Context, pid *atomic.Uint32, lastCols, lastRows *int) bool {
	w, h, err := term.GetSize(int(s.out.Fd()))
	if err != nil || w <= 0 || h <= 0 || (w == *lastCols && h == *lastRows) {
		return true
	}
	*lastCols, *lastRows = w, h
	return s.pc.UpdateSize(ctx, pid.Load(), w, h) == nil
}
