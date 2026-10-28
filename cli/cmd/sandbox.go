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
	"github.com/huaweicloud/devspace-devbox/cli/internal/envd"
	"github.com/huaweicloud/devspace-devbox/cli/internal/output"
	"github.com/huaweicloud/devspace-devbox/cli/internal/term"

	"github.com/spf13/cobra"
)

// sandboxCmd 实现 `devbox sandbox`。
var sandboxCmd = &cobra.Command{
	// Use 里的 "sandbox|sbx" 仅用于帮助展示；命令匹配靠 Aliases。
	Use:     "sandbox|sbx",
	Aliases: []string{"sandbox", "sbx"},
	Short:   "work with sandboxes",
}

// sandboxCreateCmd 的 flag 绑定变量。
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

// sandboxCreateCmd 实现 `devbox sandbox create <templateID>`，对应 POST /sandboxes。
// 对齐 e2b：创建后默认直接进入交互式终端，--detach 只创建不进入。
var sandboxCreateCmd = &cobra.Command{
	Use:     "create|cr <templateID>",
	Aliases: []string{"create", "cr"},
	Short:   "Create a sandbox and connect to its terminal",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		req := api.NewSandbox{TemplateID: args[0]}
		if cmd.Flags().Changed("timeout") {
			req.Timeout = pInt32(sandboxCreateTimeout)
		}
		if cmd.Flags().Changed("build-id") {
			req.BuildID = pString(sandboxCreateBuildID)
		}
		if req.Metadata, err = parseMetadata(sandboxCreateMetadata); err != nil {
			return fmt.Errorf("invalid --metadata: %w", err)
		}
		envMap, err := parseKV(sandboxCreateEnv)
		if err != nil {
			return fmt.Errorf("invalid --env: %w", err)
		}
		req.EnvVars = pStringMap(envMap)

		if cmd.Flags().Changed("lifecycle.ontimeout") {
			ot := api.SandboxLifecycleOnTimeout(sandboxCreateOnTimeout)
			if !ot.Valid() {
				return fmt.Errorf("invalid --lifecycle.ontimeout %q (supported: pause, kill)", sandboxCreateOnTimeout)
			}
			if sandboxCreateAutoResume && sandboxCreateOnTimeout != "pause" {
				return fmt.Errorf("--lifecycle.autoresume requires --lifecycle.ontimeout pause")
			}
			req.Lifecycle = &api.SandboxLifecycle{OnTimeout: ot, AutoResume: sandboxCreateAutoResume}
		} else if cmd.Flags().Changed("lifecycle.autoresume") {
			return fmt.Errorf("--lifecycle.autoresume requires --lifecycle.ontimeout pause")
		}

		sb, err := c.CreateSandbox(cmd.Context(), api.CreateSandboxJSONRequestBody(req))
		if err != nil {
			return err
		}

		// --detach：只创建，打印沙箱 ID 后退出。
		if sandboxCreateDetach {
			fmt.Println(sb.SandboxID)
			return nil
		}

		// 默认：进入交互式终端。
		if sb.Domain == nil || *sb.Domain == "" {
			return fmt.Errorf("sandbox response does not include a domain")
		}
		if sb.ConnectToken == nil || *sb.ConnectToken == "" {
			return fmt.Errorf("sandbox response does not include a connect token")
		}
		ec, err := envd.New(*sb.Domain, sb.SandboxID, *sb.ConnectToken, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Connecting to sandbox %s (type 'exit' to leave)\n", sb.SandboxID)
		end, err := term.New(ec, os.Stdin, os.Stdout, sandboxCreateCwd).Run(cmd.Context())
		if err != nil {
			return err
		}
		if end != nil && end.ExitCode != 0 {
			fmt.Printf("Remote shell exited (exit_code=%d, status=%s)\n", end.ExitCode, end.Status)
		}
		fmt.Printf("Closing terminal connection to sandbox %s\n", sb.SandboxID)
		return nil
	}),
}

// sandboxListCmd 的 flag 绑定变量。
var (
	sandboxListMetadata  string
	sandboxListStates    []string
	sandboxListNextToken string
	sandboxListLimit     int32
)

// sandboxListCmd 实现 `devbox sandbox list`，对应 GET /sandboxes。
var sandboxListCmd = &cobra.Command{
	Use:     "list|ls",
	Aliases: []string{"list", "ls"},
	Short:   "List sandboxes",
	Args:    cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		params := &api.ListSandboxesParams{
			Metadata:  optString(sandboxListMetadata),
			NextToken: optString(sandboxListNextToken),
		}
		if cmd.Flags().Changed("state") {
			states := make([]api.SandboxState, 0, len(sandboxListStates))
			for _, s := range sandboxListStates {
				st := api.SandboxState(s)
				if !st.Valid() {
					return fmt.Errorf("invalid state %q (supported: running, paused)", s)
				}
				states = append(states, st)
			}
			params.State = &states
		}
		if cmd.Flags().Changed("limit") {
			params.Limit = pInt(sandboxListLimit)
		}

		res, err := c.ListSandboxes(cmd.Context(), params)
		if err != nil {
			return err
		}
		if len(res.Sandboxes) == 0 && out(cmd) == output.FormatText {
			fmt.Println("No sandboxes found")
			return nil
		}
		rows := make([][]string, 0, len(res.Sandboxes))
		for _, sb := range res.Sandboxes {
			rows = append(rows, []string{
				sb.SandboxID,
				sb.TemplateID,
				formatTime(sb.StartedAt),
				formatTimePtr(sb.EndAt),
				string(sb.State),
				strconv.Itoa(int(sb.CpuCount)),
				strconv.Itoa(int(sb.MemoryMB)),
				strconv.Itoa(int(sb.DiskSizeMB)),
				formatMetadata(sb.Metadata),
			})
		}
		if err := output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON:    res.Sandboxes,
			Headers: []string{"SANDBOX ID", "TEMPLATE ID", "STARTED AT", "END AT", "STATE", "VCPU", "RAM(MB)", "DISK(MB)", "METADATA"},
			Rows:    rows,
		}); err != nil {
			return err
		}
		if res.NextToken != "" && out(cmd) == output.FormatText {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "next token: %s (pass back with --next-token)\n", res.NextToken)
		}
		return nil
	}),
}

// sandboxInfoCmd 实现 `devbox sandbox info <sandboxID>`，对应 GET /sandboxes/{id}。
var sandboxInfoCmd = &cobra.Command{
	Use:     "info|in <sandboxID>",
	Aliases: []string{"info", "in"},
	Short:   "show information for a sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
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
			{Key: "Started At", Value: formatTime(sb.StartedAt)},
			{Key: "Ended At", Value: formatTimePtr(sb.EndAt)},
			{Key: "VCPU", Value: strconv.Itoa(int(sb.CpuCount))},
			{Key: "RAM(MB)", Value: strconv.Itoa(int(sb.MemoryMB))},
			{Key: "Disk(MB)", Value: strconv.Itoa(int(sb.DiskSizeMB))},
			{Key: "Lifecycle", Value: formatJSON(sb.Lifecycle)},
			{Key: "Metadata", Value: formatMetadata(sb.Metadata)},
			{Key: "Tunnel ID", Value: output.Dash(strFromPtr(sb.TunnelId))},
			{Key: "Domain", Value: output.Dash(strFromPtr(sb.Domain))},
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{JSON: sb, KV: kv})
	}),
}

// sandboxKillCmd 实现 `devbox sandbox kill <sandboxID>`，对应 DELETE /sandboxes/{id}。
var sandboxKillCmd = &cobra.Command{
	Use:     "kill|kl <sandboxID>",
	Aliases: []string{"kill", "kl"},
	Short:   "Kill a sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.DeleteSandbox(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("sandbox %s has been killed\n", args[0])
		return nil
	}),
}

// sandboxResumeCmd 的 flag 绑定变量。
var sandboxResumeTimeout int32

// sandboxPauseCmd 实现 `devbox sandbox pause <sandboxID>`，对应 POST /sandboxes/{id}/pause。
var sandboxPauseCmd = &cobra.Command{
	Use:     "pause|ps <sandboxID>",
	Aliases: []string{"pause", "ps"},
	Short:   "Pause a sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.PauseSandbox(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("sandbox %s has been paused\n", args[0])
		return nil
	}),
}

// sandboxResumeCmd 实现 `devbox sandbox resume <sandboxID>`，对应 POST /sandboxes/{id}/resume。
var sandboxResumeCmd = &cobra.Command{
	Use:     "resume|rs <sandboxID>",
	Aliases: []string{"resume", "rs"},
	Short:   "Resume a paused sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		sb, err := c.ResumeSandbox(cmd.Context(), args[0], api.ResumeSandboxJSONRequestBody(api.TimeoutOption{
			Timeout: pInt32(sandboxResumeTimeout),
		}))
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON: sb,
			KV: []output.KV{
				{Key: "Sandbox ID", Value: sb.SandboxID},
				{Key: "State", Value: string(sb.State)},
			},
		})
	}),
}

// sandboxConnectCmd 的 flag 绑定变量。
var sandboxConnectTimeout int32

// sandboxConnectCmd 实现 `devbox sandbox connect <sandboxID>`，对应 POST /sandboxes/{id}/connect。
// 获取连接凭证后进入交互式终端（对齐 e2b 的 connect 行为）：本地终端成为
// 沙箱 shell，键入 exit 退出。
var sandboxConnectCmd = &cobra.Command{
	Use:     "connect|cn <sandboxID>",
	Aliases: []string{"connect", "cn"},
	Short:   "Open an interactive terminal to a sandbox, resuming it if paused",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		sb, err := c.ConnectSandbox(cmd.Context(), args[0], api.ConnectSandboxJSONRequestBody(api.TimeoutOption{
			Timeout: pInt32(sandboxConnectTimeout),
		}))
		if err != nil {
			return err
		}
		if sb.Domain == nil || *sb.Domain == "" {
			return fmt.Errorf("sandbox response does not include a domain")
		}
		if sb.ConnectToken == nil || *sb.ConnectToken == "" {
			return fmt.Errorf("sandbox response does not include a connect token")
		}
		ec, err := envd.New(*sb.Domain, sb.SandboxID, *sb.ConnectToken, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Connecting to sandbox %s (type 'exit' to leave)\n", sb.SandboxID)
		end, err := term.New(ec, os.Stdin, os.Stdout, "").Run(cmd.Context())
		if err != nil {
			return err
		}
		if end != nil && end.ExitCode != 0 {
			fmt.Printf("Remote shell exited (exit_code=%d, status=%s)\n", end.ExitCode, end.Status)
		}
		fmt.Printf("Closing terminal connection to sandbox %s\n", sb.SandboxID)
		return nil
	}),
}

// sandboxTimeoutCmd 的 flag 绑定变量。
var sandboxTimeoutValue int32

// sandboxTimeoutCmd 实现 `devbox sandbox timeout <sandboxID>`，对应 POST /sandboxes/{id}/timeout。
var sandboxTimeoutCmd = &cobra.Command{
	Use:     "timeout|to <sandboxID>",
	Aliases: []string{"timeout", "to"},
	Short:   "Set the time-to-live of a sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.SetSandboxTimeout(cmd.Context(), args[0], api.SetSandboxTimeoutJSONRequestBody(api.TimeoutOption{
			Timeout: pInt32(sandboxTimeoutValue),
		})); err != nil {
			return err
		}
		fmt.Printf("sandbox %s timeout set to %ds\n", args[0], sandboxTimeoutValue)
		return nil
	}),
}

// sandboxRefreshCmd 的 flag 绑定变量。
var sandboxRefreshDuration int32

// sandboxRefreshCmd 实现 `devbox sandbox refresh <sandboxID>`，对应 POST /sandboxes/{id}/refreshes。
var sandboxRefreshCmd = &cobra.Command{
	Use:     "refresh|rf <sandboxID>",
	Aliases: []string{"refresh", "rf"},
	Short:   "Extend the lifetime of a sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.RefreshSandbox(cmd.Context(), args[0], api.RefreshSandboxJSONRequestBody(api.RefreshSandboxJSONBody{
			Duration: sandboxRefreshDuration,
		})); err != nil {
			return err
		}
		fmt.Printf("sandbox %s lifetime extended by %ds\n", args[0], sandboxRefreshDuration)
		return nil
	}),
}

// sandboxExecCmd 的 flag 绑定变量。
var (
	sandboxExecBackground bool
	sandboxExecCwd        string
	sandboxExecEnv        []string
)

// sandboxExecCmd 实现 `devbox sbx exec <sandboxID> <command...>`。
// 对齐 e2b：sandbox ID 之后的全部内容透传给远程命令，devbox 选项必须放在
// sandbox ID 之前；命令经沙箱内 /bin/bash -l -c 执行，非零退出码会作为
// 本地退出码返回。
var sandboxExecCmd = &cobra.Command{
	Use:     "exec|ex <sandboxID> <command...>",
	Aliases: []string{"exec", "ex"},
	Short:   "Execute a command in a running sandbox",
	Long: `Execute a command in a running sandbox.

Everything after the sandbox ID is passed through to the remote command,
so devbox options must come before the sandbox ID:

  devbox sbx exec -c /tmp <sandboxID> codex exec "prompt" --help`,
	Args: cobra.MinimumNArgs(2),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		cmdStr, err := buildRemoteCommand(args[1:])
		if err != nil {
			return err
		}
		envMap, err := parseKV(sandboxExecEnv)
		if err != nil {
			return fmt.Errorf("invalid --env: %w", err)
		}
		// 获取连接凭证（paused 沙箱会被自动恢复），不设置沙箱 TTL。
		sb, err := c.ConnectSandbox(cmd.Context(), args[0], api.ConnectSandboxJSONRequestBody(api.TimeoutOption{}))
		if err != nil {
			return err
		}
		if sb.Domain == nil || *sb.Domain == "" {
			return fmt.Errorf("sandbox response does not include a domain")
		}
		if sb.ConnectToken == nil || *sb.ConnectToken == "" {
			return fmt.Errorf("sandbox response does not include a connect token")
		}
		pc, err := client.NewProcessClient(*sb.Domain, args[0], *sb.ConnectToken, nil)
		if err != nil {
			return err
		}
		cfg := &process.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-l", "-c", cmdStr},
			Envs: envMap,
			Cwd:  optString(sandboxExecCwd),
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

// execForeground 前台执行：转发远程 stdout/stderr 到本地，等待进程退出并
// 以远程退出码作为本地退出码。
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

// execBackground 后台执行：等首个 start 事件拿到 PID，转发管道输入（如有），
// 打印 PID 后立即退出，远程进程继续运行。
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
		return fmt.Errorf("sandbox did not return a process id")
	}
	if piped {
		streamLocalStdin(cmd.Context(), pc, pid)
	}
	_ = stream.Close()
	fmt.Fprintln(os.Stderr, pid)
	return nil
}

// startSignalForwarder 把本地 SIGINT/SIGTERM 转发为远程 SIGKILL（对齐 e2b）：
// 远程进程终止后事件流会收到 end 事件，本地随之退出。
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

// streamLocalStdin 将本地管道 stdin 按 64KB 分块转发到远程进程 stdin，
// EOF 后发送 CloseStdin（对齐 e2b 的 sendStdin）。
func streamLocalStdin(ctx context.Context, pc *client.ProcessClient, pid uint32) {
	buf := make([]byte, 64*1024)
	for {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 {
			if err := pc.SendInput(ctx, pid, buf[:n]); err != nil {
				if client.IsNotFound(err) {
					fmt.Fprintln(os.Stderr, "devbox: Remote command exited before stdin could be delivered.")
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

// writeProcessEventOutput 把远程输出帧写到本地 stdout/stderr。
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

// buildRemoteCommand 拼接远程命令（对齐 e2b 的 buildCommand）：单个参数原样
// 使用；多个参数按 POSIX 规则引号转义后拼接，最终经 /bin/bash -c 执行。
func buildRemoteCommand(args []string) (string, error) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return "", fmt.Errorf("missing command to execute")
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

// unsafeShellChar 匹配需要引号转义的字符（对齐 e2b 的 UNSAFE_SHELL_CHAR）。
var unsafeShellChar = regexp.MustCompile(`[^\w@%+=:,./-]`)

// shellQuote POSIX 转义单个参数。
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if unsafeShellChar.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}
	return s
}

// pipedStdin 检测本地标准输入是否是管道/FIFO/socket/文件重定向（对齐 e2b 的
// isPipedStdin）；交互式终端（字符设备）返回 false。
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
		"Time to live for the sandbox in seconds (10-86400)")
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateMetadata, "metadata", "m", nil,
		"Metadata key=value (repeatable)")
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateEnv, "env", "e", nil,
		"Environment variable key=value (repeatable)")
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateBuildID, "build-id", "b", "",
		"Template build identifier")
	sandboxCreateCmd.Flags().StringVar(&sandboxCreateOnTimeout, "lifecycle.ontimeout", "",
		"Action when sandbox timeout is reached: pause or kill")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateAutoResume, "lifecycle.autoresume", false,
		"Enable sandbox auto-resume, requires --lifecycle.ontimeout pause")
	sandboxCreateCmd.Flags().BoolVarP(&sandboxCreateDetach, "detach", "d", false,
		"Create the sandbox without connecting to its terminal")
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateCwd, "cwd", "c", "",
		"Working directory for the terminal session inside the sandbox")

	sandboxListCmd.Flags().StringVarP(&sandboxListMetadata, "metadata", "m", "",
		`Metadata filter, e.g. "user=abc&app=prod"`)
	sandboxListCmd.Flags().StringSliceVarP(&sandboxListStates, "state", "s", nil,
		"Filter by state: running or paused (repeatable)")
	sandboxListCmd.Flags().StringVarP(&sandboxListNextToken, "next-token", "n", "",
		"Pagination cursor")
	sandboxListCmd.Flags().Int32VarP(&sandboxListLimit, "limit", "l", 0,
		"Maximum number of results (1-100)")

	sandboxResumeCmd.Flags().Int32Var(&sandboxResumeTimeout, "timeout", 300,
		"New time to live in seconds (10-86400)")

	sandboxConnectCmd.Flags().Int32Var(&sandboxConnectTimeout, "timeout", 300,
		"Time to live in seconds (10-86400)")

	sandboxTimeoutCmd.Flags().Int32VarP(&sandboxTimeoutValue, "timeout", "t", 300,
		"New time to live in seconds (10-86400)")

	sandboxRefreshCmd.Flags().Int32VarP(&sandboxRefreshDuration, "duration", "d", 300,
		"Lifetime extension in seconds (10-86400)")

	sandboxExecCmd.Flags().BoolVarP(&sandboxExecBackground, "background", "b", false,
		"Run in background and return immediately")
	sandboxExecCmd.Flags().StringVarP(&sandboxExecCwd, "cwd", "c", "",
		"Working directory for the remote command")
	sandboxExecCmd.Flags().StringArrayVarP(&sandboxExecEnv, "env", "e", nil,
		"Environment variable key=value (repeatable)")
	// sandbox ID 之后的内容全部透传给远程命令。
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
