package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api"
	"github.com/huaweicloud/devspace-devbox/cli/internal/api/processgen/process"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
	"github.com/huaweicloud/devspace-devbox/cli/internal/output"
	"github.com/huaweicloud/devspace-devbox/cli/internal/parse"
	"github.com/huaweicloud/devspace-devbox/cli/internal/term"

	"github.com/spf13/cobra"
)

// validateTimeoutSeconds validates the timeout/duration input: the valid range is 10-86400 seconds,
// an explicit 0 is invalid (not conflated with omission).
func validateTimeoutSeconds(v int32) error {
	if v < 10 || v > 86400 {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidTimeout), v)
	}
	return nil
}

// validateSandboxID validates the sandboxID input: lowercase letters, digits and hyphens, 36 characters.
func validateSandboxID(s string) error {
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

// validateTemplateID validates the templateID input: lowercase letters, digits and hyphens,
// at most 64 characters, and must not be empty.
func validateTemplateID(s string) error {
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

// parseStates parses the repeatable --state values and validates them.
func parseStates(values []string) ([]api.SandboxState, error) {
	states := make([]api.SandboxState, 0, len(values))
	for _, s := range values {
		st := api.SandboxState(s)
		if !st.Valid() {
			return nil, fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidState), s)
		}
		states = append(states, st)
	}
	return states, nil
}

// runTerminalSession validates the connection info from the sandbox response
// and opens the interactive terminal, printing the connecting/closing notices.
func runTerminalSession(cmd *cobra.Command, sb *api.Sandbox, cwd string, envs map[string]string) error {
	if sb.Domain == nil || *sb.Domain == "" {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingDomain))
	}
	if sb.ConnectToken == nil || *sb.ConnectToken == "" {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingConnectToken))
	}
	pc, err := client.NewProcessClient(*sb.Domain, sb.SandboxID, *sb.ConnectToken, nil)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.ConnectingTo)+"\n", sb.SandboxID)
	end, err := term.New(pc, os.Stdin, os.Stdout, cwd, envs).Run(cmd.Context())
	if err != nil {
		return err
	}
	if end != nil && end.ExitCode != 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.RemoteShellExited)+"\n", end.ExitCode, end.Status)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.ClosingConnection)+"\n", sb.SandboxID)
	return nil
}

// sandboxCmd implements `devbox sandbox`.
var sandboxCmd = &cobra.Command{
	// The "sandbox|sbx" in Use is only for help display; matching relies on Aliases.
	Use:     "sandbox|sbx",
	Aliases: []string{"sandbox", "sbx"},
	Short:   i18n.T(i18n.Msg.Sandbox.SandboxShort),
}

// sandboxCreateCmd's flag bound variables.
var (
	sandboxCreateTimeout    int32
	sandboxCreateMetadata   []string
	sandboxCreateEnv        []string
	sandboxCreateBuildID    string
	sandboxCreateOnTimeout  string
	sandboxCreateAutoResume bool
	sandboxCreateDetach     bool
	sandboxCreateCwd        string
)

// sandboxCreateCmd implements `devbox sandbox create <templateID>`, corresponding to POST /sandboxes.
// Aligned with e2b: by default the terminal opens right after creation; --detach only creates.
var sandboxCreateCmd = &cobra.Command{
	Use:     "create|cr <templateID>",
	Aliases: []string{"create", "cr"},
	Short:   i18n.T(i18n.Msg.Sandbox.CreateShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateTemplateID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		req := api.NewSandbox{TemplateID: args[0]}
		if cmd.Flags().Changed("timeout") {
			if err := validateTimeoutSeconds(sandboxCreateTimeout); err != nil {
				return err
			}
			req.Timeout = api.PInt32(sandboxCreateTimeout)
		}
		if cmd.Flags().Changed("build-id") {
			req.BuildID = api.PString(sandboxCreateBuildID)
		}
		if req.Metadata, err = parse.ParseMetadata(sandboxCreateMetadata); err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidMetadata), err)
		}
		envMap, err := parse.ParseKV(sandboxCreateEnv)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidEnv), err)
		}
		// Aligned with e2b: -e only sets the terminal session's environment variables (sandbox-level envVars are not written).

		if cmd.Flags().Changed("lifecycle.ontimeout") {
			ot := api.SandboxLifecycleOnTimeout(sandboxCreateOnTimeout)
			if !ot.Valid() {
				return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidOnTimeout), sandboxCreateOnTimeout)
			}
			if sandboxCreateAutoResume && sandboxCreateOnTimeout != "pause" {
				return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.AutoResumeRequires))
			}
			req.Lifecycle = &api.SandboxLifecycle{OnTimeout: ot, AutoResume: sandboxCreateAutoResume}
		} else if cmd.Flags().Changed("lifecycle.autoresume") {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.AutoResumeRequires))
		}

		sb, err := c.CreateSandbox(cmd.Context(), api.CreateSandboxJSONRequestBody(req))
		if err != nil {
			return err
		}

		// --detach: create only, print the sandbox ID and exit.
		if sandboxCreateDetach {
			fmt.Println(sb.SandboxID)
			return nil
		}

		// Default: open the interactive terminal.
		return runTerminalSession(cmd, sb, sandboxCreateCwd, envMap)
	}),
}

// sandboxListCmd's flag bound variables.
var (
	sandboxListMetadata string
	sandboxListStates   []string
	sandboxListTemplate string
	sandboxListLimit    int32
	sandboxListFormat   string
)

// sandboxListCmd implements `devbox sandbox list`, corresponding to GET /sandboxes.
var sandboxListCmd = &cobra.Command{
	Use:     "list|ls",
	Aliases: []string{"list", "ls"},
	Short:   i18n.T(i18n.Msg.Sandbox.ListShort),
	Args:    cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		params := &api.ListSandboxesParams{
			Metadata: api.OptString(sandboxListMetadata),
			Template: api.OptString(sandboxListTemplate),
		}
		if cmd.Flags().Changed("state") {
			states, err := parseStates(sandboxListStates)
			if err != nil {
				return err
			}
			params.State = &states
		}
		// Aligned with e2b: 0 means no limit (the limit parameter is not sent).
		if sandboxListLimit != 0 {
			params.Limit = api.PInt(sandboxListLimit)
		}

		res, err := c.ListSandboxes(cmd.Context(), params)
		if err != nil {
			return err
		}
		if len(res.Sandboxes) == 0 && out(cmd) == output.FormatText {
			fmt.Println(i18n.T(i18n.Msg.Sandbox.ListEmpty))
			return nil
		}
		rows := make([][]string, 0, len(res.Sandboxes))
		for _, sb := range res.Sandboxes {
			rows = append(rows, []string{
				sb.SandboxID,
				sb.TemplateID,
				output.FormatTime(sb.StartedAt),
				output.FormatTimePtr(sb.EndAt),
				string(sb.State),
				strconv.Itoa(int(sb.CpuCount)),
				strconv.Itoa(int(sb.MemoryMB)),
				strconv.Itoa(int(sb.DiskSizeMB)),
				output.FormatMetadata(sb.Metadata),
			})
		}
		if err := output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON:    res.Sandboxes,
			Headers: []string{"SANDBOX ID", "TEMPLATE ID", "STARTED AT", "END AT", "STATE", "VCPUS", "RAM(MB)", "DISK(MB)", "METADATA"},
			Rows:    rows,
		}); err != nil {
			return err
		}
		return nil
	}),
}

// sandboxInfoCmd implements `devbox sandbox info <sandboxID>`, corresponding to GET /sandboxes/{id}.
var sandboxInfoFormat string

var sandboxInfoCmd = &cobra.Command{
	Use:     "info|in <sandboxID>",
	Aliases: []string{"info", "in"},
	Short:   i18n.T(i18n.Msg.Sandbox.InfoShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		sb, err := c.GetSandbox(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		kv := []output.KV{
			{Key: "Sandbox ID", Value: sb.SandboxID},
			{Key: "Template ID", Value: sb.TemplateID},
			{Key: "State", Value: string(sb.State)},
			{Key: "Started at", Value: output.FormatTime(sb.StartedAt)},
			{Key: "End at", Value: output.FormatTimePtr(sb.EndAt)},
			{Key: "vCPUs", Value: strconv.Itoa(int(sb.CpuCount))},
			{Key: "RAM(MB)", Value: strconv.Itoa(int(sb.MemoryMB))},
			{Key: "Disk(MB)", Value: strconv.Itoa(int(sb.DiskSizeMB))},
			{Key: "Lifecycle", Value: output.JSONString(sb.Lifecycle)},
			{Key: "Metadata", Value: output.FormatMetadata(sb.Metadata)},
			{Key: "Tunnel ID", Value: output.Dash(strFromPtr(sb.TunnelId))},
			{Key: "Domain", Value: output.Dash(strFromPtr(sb.Domain))},
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{JSON: sb, KV: kv})
	}),
}

// sandboxKillCmd's flag bound variables.
var (
	sandboxKillAll      bool
	sandboxKillState    []string
	sandboxKillMetadata string
)

// sandboxKillCmd implements `devbox sandbox kill [sandboxID...]`, corresponding to DELETE /sandboxes/{id}.
// Aligned with e2b: multiple IDs are killed one by one, or --all kills in batch
// filtered by state/metadata (mutually exclusive with IDs).
var sandboxKillCmd = &cobra.Command{
	Use:     "kill|kl [sandboxID...]",
	Aliases: []string{"kill", "kl"},
	Short:   i18n.T(i18n.Msg.Sandbox.KillShort),
	Args:    cobra.ArbitraryArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}

		ids := args
		if sandboxKillAll {
			if len(ids) > 0 {
				return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.KillAllConflict))
			}
			// List by state/metadata then kill each; the state defaults to running.
			states := []api.SandboxState{api.Running}
			if cmd.Flags().Changed("state") {
				parsed, err := parseStates(sandboxKillState)
				if err != nil {
					return err
				}
				states = parsed
			}
			params := &api.ListSandboxesParams{
				State:    &states,
				Metadata: api.OptString(sandboxKillMetadata),
			}
			for {
				res, listErr := c.ListSandboxes(cmd.Context(), params)
				if listErr != nil {
					return listErr
				}
				for _, sb := range res.Sandboxes {
					ids = append(ids, sb.SandboxID)
				}
				if res.NextToken == "" {
					break
				}
				params.NextToken = api.PString(res.NextToken)
			}
		} else if len(ids) == 0 {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.KillRequiresArgs))
		}

		for _, id := range ids {
			if err := validateSandboxID(id); err != nil {
				return err
			}
		}
		for _, id := range ids {
			if err := c.DeleteSandbox(cmd.Context(), id); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", id, err)
				continue
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Killed)+"\n", id)
		}
		return nil
	}),
}

// sandboxResumeCmd's flag bound variables.
var sandboxResumeTimeout int32

// sandboxPauseCmd implements `devbox sandbox pause <sandboxID>`, corresponding to POST /sandboxes/{id}/pause.
var sandboxPauseCmd = &cobra.Command{
	Use:     "pause|ps <sandboxID>",
	Aliases: []string{"pause", "ps"},
	Short:   i18n.T(i18n.Msg.Sandbox.PauseShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.PauseSandbox(cmd.Context(), args[0]); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Paused)+"\n", args[0])
		return nil
	}),
}

// sandboxResumeCmd implements `devbox sandbox resume <sandboxID>`, corresponding to POST /sandboxes/{id}/resume.
var sandboxResumeCmd = &cobra.Command{
	Use:     "resume|rs <sandboxID>",
	Aliases: []string{"resume", "rs"},
	Short:   i18n.T(i18n.Msg.Sandbox.ResumeShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		// With -t omitted the timeout field is not sent (the sandbox keeps its current TTL); an explicit value is validated and set.
		opt := api.TimeoutOption{}
		if cmd.Flags().Changed("timeout") {
			if err := validateTimeoutSeconds(sandboxResumeTimeout); err != nil {
				return err
			}
			opt.Timeout = api.PInt32(sandboxResumeTimeout)
		}
		_, err = c.ResumeSandbox(cmd.Context(), args[0], api.ResumeSandboxJSONRequestBody(opt))
		if err != nil {
			return err
		}
		// Aligned with e2b: single-line output, no fields.
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Resumed)+"\n", args[0])
		return nil
	}),
}

// sandboxConnectCmd's flag bound variables.
var (
	sandboxConnectTimeout int32
	sandboxConnectCwd     string
	sandboxConnectEnv     []string
)

// sandboxConnectCmd implements `devbox sandbox connect <sandboxID>`, corresponding to POST /sandboxes/{id}/connect.
// After obtaining the connection credentials the interactive terminal opens (aligned with e2b's connect):
// the local terminal becomes the sandbox shell; type exit to leave.
var sandboxConnectCmd = &cobra.Command{
	Use:     "connect|cn <sandboxID>",
	Aliases: []string{"connect", "cn"},
	Short:   i18n.T(i18n.Msg.Sandbox.ConnectShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		// With -t omitted the timeout field is not sent (the sandbox keeps its current TTL); an explicit value is validated and set.
		opt := api.TimeoutOption{}
		if cmd.Flags().Changed("timeout") {
			if err := validateTimeoutSeconds(sandboxConnectTimeout); err != nil {
				return err
			}
			opt.Timeout = api.PInt32(sandboxConnectTimeout)
		}
		sb, err := c.ConnectSandbox(cmd.Context(), args[0], api.ConnectSandboxJSONRequestBody(opt))
		if err != nil {
			return err
		}
		if sb.Domain == nil || *sb.Domain == "" {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingDomain))
		}
		if sb.ConnectToken == nil || *sb.ConnectToken == "" {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingConnectToken))
		}
		envMap, err := parse.ParseKV(sandboxConnectEnv)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidEnv), err)
		}
		return runTerminalSession(cmd, sb, sandboxConnectCwd, envMap)
	}),
}

// sandboxTimeoutCmd's flag bound variables.
var sandboxTimeoutValue int32

// sandboxTimeoutCmd implements `devbox sandbox timeout <sandboxID>`, corresponding to POST /sandboxes/{id}/timeout.
var sandboxTimeoutCmd = &cobra.Command{
	Use:     "timeout|to <sandboxID>",
	Aliases: []string{"timeout", "to"},
	Short:   i18n.T(i18n.Msg.Sandbox.TimeoutShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		// -t is optional: omitted uses the default 300; an explicit value is range-checked (10-86400).
		if cmd.Flags().Changed("timeout") {
			if err := validateTimeoutSeconds(sandboxTimeoutValue); err != nil {
				return err
			}
		}
		if err := c.SetSandboxTimeout(cmd.Context(), args[0], api.SetSandboxTimeoutJSONRequestBody(api.TimeoutOption{
			Timeout: api.PInt32(sandboxTimeoutValue),
		})); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.TimeoutSet)+"\n", args[0], sandboxTimeoutValue)
		return nil
	}),
}

// sandboxRefreshCmd's flag bound variables.
var sandboxRefreshDuration int32

// sandboxRefreshCmd implements `devbox sandbox refresh <sandboxID>`, corresponding to POST /sandboxes/{id}/refreshes.
var sandboxRefreshCmd = &cobra.Command{
	Use:     "refresh|rf <sandboxID>",
	Aliases: []string{"refresh", "rf"},
	Short:   i18n.T(i18n.Msg.Sandbox.RefreshShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		// -d is optional: omitted uses the default 300; an explicit value is range-checked (10-86400).
		if cmd.Flags().Changed("duration") {
			if err := validateTimeoutSeconds(sandboxRefreshDuration); err != nil {
				return err
			}
		}
		if err := c.RefreshSandbox(cmd.Context(), args[0], api.RefreshSandboxJSONRequestBody(api.RefreshSandboxJSONBody{
			Duration: sandboxRefreshDuration,
		})); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Refreshed)+"\n", args[0], sandboxRefreshDuration)
		return nil
	}),
}

// sandboxExecCmd's flag bound variables.
var (
	sandboxExecBackground bool
	sandboxExecCwd        string
	sandboxExecEnv        []string
)

// sandboxExecCmd implements `devbox sbx exec <sandboxID> <command...>`.
// Aligned with e2b: everything after the sandbox ID is passed through to the remote command,
// so devbox options must come before the sandbox ID; the command runs via /bin/bash -l -c inside the sandbox
// and a non-zero exit code is returned as the local exit code.
var sandboxExecCmd = &cobra.Command{
	Use:     "exec|ex <sandboxID> <command...>",
	Aliases: []string{"exec", "ex"},
	Short:   i18n.T(i18n.Msg.Sandbox.ExecShort),
	Long:    i18n.T(i18n.Msg.Sandbox.ExecLong),
	Args:    cobra.MinimumNArgs(2),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		cmdStr, err := buildRemoteCommand(args[1:])
		if err != nil {
			return err
		}
		envMap, err := parse.ParseKV(sandboxExecEnv)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidEnv), err)
		}
		// Obtain the connection credentials (a paused sandbox is resumed automatically); no sandbox TTL is set.
		sb, err := c.ConnectSandbox(cmd.Context(), args[0], api.ConnectSandboxJSONRequestBody(api.TimeoutOption{}))
		if err != nil {
			return err
		}
		if sb.Domain == nil || *sb.Domain == "" {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingDomain))
		}
		if sb.ConnectToken == nil || *sb.ConnectToken == "" {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingConnectToken))
		}
		pc, err := client.NewProcessClient(*sb.Domain, args[0], *sb.ConnectToken, nil)
		if err != nil {
			return err
		}
		cfg := &process.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-l", "-c", cmdStr},
			Envs: envMap,
			Cwd:  api.OptString(sandboxExecCwd),
		}
		piped := pipedStdin()
		stream, err := pc.Start(cmd.Context(), cfg, piped)
		if err != nil {
			return err
		}
		if sandboxExecBackground {
			return execBackground(cmd, pc, stream, piped)
		}
		return execForeground(cmd, pc, stream, piped)
	}),
}

// execForeground runs in the foreground: forwards the remote stdout/stderr locally, waits for the process to exit
// and returns the remote exit code as the local exit code.
func execForeground(cmd *cobra.Command, pc *client.ProcessClient, stream *connect.ServerStreamForClient[process.StartResponse], piped bool) error {
	var pid uint32
	var exitCode int32 = 1
	started := false
	for stream.Receive() {
		body := stream.Msg().GetEvent()
		if body == nil {
			continue
		}
		switch e := body.GetEvent().(type) {
		case *process.ProcessEvent_Start:
			pid = e.Start.GetPid()
			if !started {
				started = true
				startSignalForwarder(cmd.Context(), pc, pid)
				if piped {
					go streamLocalStdin(cmd.Context(), pc, pid)
				}
			}
		case *process.ProcessEvent_Data:
			writeProcessEventOutput(e.Data, cmd)
		case *process.ProcessEvent_End:
			exitCode = e.End.GetExitCode()
			if e.End.GetError() != "" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), e.End.GetError())
			}
		case *process.ProcessEvent_Keepalive:
		}
	}
	if err := stream.Err(); err != nil {
		return err
	}
	os.Exit(int(exitCode))
	return nil
}

// execBackground runs in the background: waits for the first start event to get the PID, forwards piped input (if any),
// prints the PID and exits immediately; the remote process keeps running.
func execBackground(cmd *cobra.Command, pc *client.ProcessClient, stream *connect.ServerStreamForClient[process.StartResponse], piped bool) error {
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
		return err
	}
	if pid == 0 {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.NoProcessID))
	}
	if piped {
		streamLocalStdin(cmd.Context(), pc, pid)
	}
	_ = stream.Close()
	fmt.Fprintln(os.Stderr, pid)
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
// sending CloseStdin after EOF (aligned with e2b's sendStdin).
func streamLocalStdin(ctx context.Context, pc *client.ProcessClient, pid uint32) {
	buf := make([]byte, 64*1024)
	for {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 {
			if err := pc.SendInput(ctx, pid, buf[:n]); err != nil {
				if client.IsNotFound(err) {
					fmt.Fprintln(os.Stderr, i18n.T(i18n.Msg.Sandbox.StdinAfterExit))
				}
				return
			}
		}
		if readErr != nil {
			break
		}
	}
	_ = pc.CloseStdin(ctx, pid)
}

// writeProcessEventOutput writes the remote output frames to the local stdout/stderr.
func writeProcessEventOutput(data *process.ProcessEvent_DataEvent, cmd *cobra.Command) {
	if data == nil {
		return
	}
	switch out := data.GetOutput().(type) {
	case *process.ProcessEvent_DataEvent_Stdout:
		_, _ = cmd.OutOrStdout().Write(out.Stdout)
	case *process.ProcessEvent_DataEvent_Stderr:
		_, _ = cmd.ErrOrStderr().Write(out.Stderr)
	case *process.ProcessEvent_DataEvent_Pty:
		_, _ = cmd.OutOrStdout().Write(out.Pty)
	}
}

// buildRemoteCommand assembles the remote command (aligned with e2b's buildCommand): a single argument is used
// as-is; multiple arguments are shell-quoted per POSIX rules and joined, ultimately executed via /bin/bash -c.
func buildRemoteCommand(args []string) (string, error) {
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
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " "), nil
}

// unsafeShellChar matches the characters that need shell quoting (aligned with e2b's UNSAFE_SHELL_CHAR).
var unsafeShellChar = regexp.MustCompile(`[^\w@%+=:,./-]`)

// shellQuote quotes a single argument per POSIX rules.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if unsafeShellChar.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}
	return s
}

// pipedStdin reports whether the local stdin is a pipe/FIFO/socket/redirected file (aligned with e2b's
// isPipedStdin); an interactive terminal (character device) returns false.
func pipedStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return false
	}
	return info.Mode()&(os.ModeNamedPipe|os.ModeSocket) != 0 || info.Mode().IsRegular()
}

func init() {
	sandboxCreateCmd.Flags().Int32VarP(&sandboxCreateTimeout, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagTimeout))
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateMetadata, "metadata", "m", nil,
		i18n.T(i18n.Msg.Sandbox.FlagMetadata))
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateEnv, "env", "e", nil,
		i18n.T(i18n.Msg.Sandbox.FlagEnv))
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateBuildID, "build-id", "b", "",
		i18n.T(i18n.Msg.Sandbox.FlagBuildID))
	sandboxCreateCmd.Flags().StringVar(&sandboxCreateOnTimeout, "lifecycle.ontimeout", "",
		i18n.T(i18n.Msg.Sandbox.FlagOnTimeout))
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateAutoResume, "lifecycle.autoresume", false,
		i18n.T(i18n.Msg.Sandbox.FlagAutoResume))
	sandboxCreateCmd.Flags().BoolVarP(&sandboxCreateDetach, "detach", "d", false,
		i18n.T(i18n.Msg.Sandbox.FlagDetach))
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateCwd, "cwd", "c", "",
		i18n.T(i18n.Msg.Sandbox.FlagCwd))

	sandboxListCmd.Flags().StringVarP(&sandboxListMetadata, "metadata", "m", "",
		i18n.T(i18n.Msg.Sandbox.FlagMetadataFilter))
	sandboxListCmd.Flags().StringVarP(&sandboxListTemplate, "template", "t", "",
		i18n.T(i18n.Msg.Sandbox.FlagTemplateFilter))
	sandboxListCmd.Flags().StringSliceVarP(&sandboxListStates, "state", "s", nil,
		i18n.T(i18n.Msg.Sandbox.FlagState))
	sandboxListCmd.Flags().Int32VarP(&sandboxListLimit, "limit", "l", 100,
		i18n.T(i18n.Msg.Sandbox.FlagLimit))
	sandboxListCmd.Flags().StringVarP(&sandboxListFormat, "format", "f", "pretty",
		i18n.T(i18n.Msg.Sandbox.FlagFormat))

	sandboxKillCmd.Flags().BoolVarP(&sandboxKillAll, "all", "a", false,
		i18n.T(i18n.Msg.Sandbox.FlagKillAll))
	sandboxKillCmd.Flags().StringSliceVarP(&sandboxKillState, "state", "s", nil,
		i18n.T(i18n.Msg.Sandbox.FlagKillState))
	sandboxKillCmd.Flags().StringVarP(&sandboxKillMetadata, "metadata", "m", "",
		i18n.T(i18n.Msg.Sandbox.FlagKillMetadata))

	sandboxResumeCmd.Flags().Int32VarP(&sandboxResumeTimeout, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagTimeoutKeep))

	sandboxConnectCmd.Flags().Int32VarP(&sandboxConnectTimeout, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagTimeoutKeep))
	sandboxConnectCmd.Flags().StringVarP(&sandboxConnectCwd, "cwd", "c", "",
		i18n.T(i18n.Msg.Sandbox.FlagCwd))
	sandboxConnectCmd.Flags().StringArrayVarP(&sandboxConnectEnv, "env", "e", nil,
		i18n.T(i18n.Msg.Sandbox.FlagEnvTerm))

	sandboxTimeoutCmd.Flags().Int32VarP(&sandboxTimeoutValue, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagNewTimeout))
	sandboxInfoCmd.Flags().StringVarP(&sandboxInfoFormat, "format", "f", "pretty",
		i18n.T(i18n.Msg.Sandbox.FlagFormat))

	sandboxRefreshCmd.Flags().Int32VarP(&sandboxRefreshDuration, "duration", "d", 300,
		i18n.T(i18n.Msg.Sandbox.FlagDuration))

	sandboxExecCmd.Flags().BoolVarP(&sandboxExecBackground, "background", "b", false,
		i18n.T(i18n.Msg.Sandbox.FlagBackground))
	sandboxExecCmd.Flags().StringVarP(&sandboxExecCwd, "cwd", "c", "",
		i18n.T(i18n.Msg.Sandbox.FlagCwd))
	sandboxExecCmd.Flags().StringArrayVarP(&sandboxExecEnv, "env", "e", nil,
		i18n.T(i18n.Msg.Sandbox.FlagEnv))
	// Everything after the sandbox ID is passed through to the remote command.
	sandboxExecCmd.Flags().SetInterspersed(false)

	sandboxCmd.AddCommand(
		sandboxCreateCmd,
		sandboxListCmd,
		sandboxInfoCmd,
		sandboxKillCmd,
		sandboxPauseCmd,
		sandboxResumeCmd,
		sandboxConnectCmd,
		sandboxExecCmd,
		sandboxTimeoutCmd,
		sandboxRefreshCmd,
	)
}
