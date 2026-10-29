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
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
	"github.com/huaweicloud/devspace-devbox/cli/internal/output"
	"github.com/huaweicloud/devspace-devbox/cli/internal/parse"
	"github.com/huaweicloud/devspace-devbox/cli/internal/term"

	"github.com/spf13/cobra"
)

// validateTimeoutSeconds 校验 timeout/duration 入参：合法范围 10-86400 秒，
// 显式传 0 视为无效（与省略互不混淆）。
func validateTimeoutSeconds(v int32) error {
	if v < 10 || v > 86400 {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidTimeout), v)
	}
	return nil
}

// validateSandboxID 校验 sandboxID 入参：小写字母、数字、中划线，长度 36。
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

// validateTemplateID 校验 templateID 入参：小写字母、数字、中划线，
// 长度不超过 64，且不能为空。
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

// sandboxCmd 实现 `devbox sandbox`。
var sandboxCmd = &cobra.Command{
	// Use 里的 "sandbox|sbx" 仅用于帮助展示；命令匹配靠 Aliases。
	Use:     "sandbox|sbx",
	Aliases: []string{"sandbox", "sbx"},
	Short:   i18n.T(i18n.Msg.Sandbox.SandboxShort),
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
		req.EnvVars = api.PStringMap(envMap)

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

		// --detach：只创建，打印沙箱 ID 后退出。
		if sandboxCreateDetach {
			fmt.Println(sb.SandboxID)
			return nil
		}

		// 默认：进入交互式终端。
		if sb.Domain == nil || *sb.Domain == "" {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingDomain))
		}
		if sb.ConnectToken == nil || *sb.ConnectToken == "" {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingConnectToken))
		}
		ec, err := envd.New(*sb.Domain, sb.SandboxID, *sb.ConnectToken, nil)
		if err != nil {
			return err
		}
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.ConnectingTo)+"\n", sb.SandboxID)
		end, err := term.New(ec, os.Stdin, os.Stdout, sandboxCreateCwd, envMap).Run(cmd.Context())
		if err != nil {
			return err
		}
		if end != nil && end.ExitCode != 0 {
			fmt.Printf(i18n.T(i18n.Msg.Sandbox.RemoteShellExited)+"\n", end.ExitCode, end.Status)
		}
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.ClosingConnection)+"\n", sb.SandboxID)
		return nil
	}),
}

// sandboxListCmd 的 flag 绑定变量。
var (
	sandboxListMetadata string
	sandboxListStates   []string
	sandboxListLimit    int32
	sandboxListFormat   string
)

// sandboxListCmd 实现 `devbox sandbox list`，对应 GET /sandboxes。
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
		}
		if cmd.Flags().Changed("state") {
			states := make([]api.SandboxState, 0, len(sandboxListStates))
			for _, s := range sandboxListStates {
				st := api.SandboxState(s)
				if !st.Valid() {
					return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidState), s)
				}
				states = append(states, st)
			}
			params.State = &states
		}
		// 对齐 e2b：默认 1000，0 表示不限制（不发送 limit 参数）。
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

// sandboxInfoCmd 实现 `devbox sandbox info <sandboxID>`，对应 GET /sandboxes/{id}。
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

// sandboxKillCmd 实现 `devbox sandbox kill <sandboxID>`，对应 DELETE /sandboxes/{id}。
var sandboxKillCmd = &cobra.Command{
	Use:     "kill|kl <sandboxID>",
	Aliases: []string{"kill", "kl"},
	Short:   i18n.T(i18n.Msg.Sandbox.KillShort),
	Args:    cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := validateSandboxID(args[0]); err != nil {
			return err
		}
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.DeleteSandbox(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.Killed)+"\n", args[0])
		return nil
	}),
}

// sandboxResumeCmd 的 flag 绑定变量。
var sandboxResumeTimeout int32

// sandboxPauseCmd 实现 `devbox sandbox pause <sandboxID>`，对应 POST /sandboxes/{id}/pause。
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
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.Paused)+"\n", args[0])
		return nil
	}),
}

// sandboxResumeCmd 实现 `devbox sandbox resume <sandboxID>`，对应 POST /sandboxes/{id}/resume。
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
		// 省略 -t 时不上送 timeout 字段（保持沙箱当前 TTL）；显式传值才校验并设置。
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
		// 对齐 e2b：单行展示，不打印字段。
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.Resumed)+"\n", args[0])
		return nil
	}),
}

// sandboxConnectCmd 的 flag 绑定变量。
var (
	sandboxConnectTimeout int32
	sandboxConnectCwd     string
	sandboxConnectEnv     []string
)

// sandboxConnectCmd 实现 `devbox sandbox connect <sandboxID>`，对应 POST /sandboxes/{id}/connect。
// 获取连接凭证后进入交互式终端（对齐 e2b 的 connect 行为）：本地终端成为
// 沙箱 shell，键入 exit 退出。
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
		// 省略 -t 时不上送 timeout 字段（保持沙箱当前 TTL）；显式传值才校验并设置。
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
		ec, err := envd.New(*sb.Domain, sb.SandboxID, *sb.ConnectToken, nil)
		if err != nil {
			return err
		}
		envMap, err := parse.ParseKV(sandboxConnectEnv)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidEnv), err)
		}
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.ConnectingTo)+"\n", sb.SandboxID)
		end, err := term.New(ec, os.Stdin, os.Stdout, sandboxConnectCwd, envMap).Run(cmd.Context())
		if err != nil {
			return err
		}
		if end != nil && end.ExitCode != 0 {
			fmt.Printf(i18n.T(i18n.Msg.Sandbox.RemoteShellExited)+"\n", end.ExitCode, end.Status)
		}
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.ClosingConnection)+"\n", sb.SandboxID)
		return nil
	}),
}

// sandboxTimeoutCmd 的 flag 绑定变量。
var sandboxTimeoutValue int32

// sandboxTimeoutCmd 实现 `devbox sandbox timeout <sandboxID>`，对应 POST /sandboxes/{id}/timeout。
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
		// -t 非必传：省略时使用默认值 300；显式传值时做范围校验（10-86400）。
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
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.TimeoutSet)+"\n", args[0], sandboxTimeoutValue)
		return nil
	}),
}

// sandboxRefreshCmd 的 flag 绑定变量。
var sandboxRefreshDuration int32

// sandboxRefreshCmd 实现 `devbox sandbox refresh <sandboxID>`，对应 POST /sandboxes/{id}/refreshes。
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
		// -d 非必传：省略时使用默认值 300；显式传值时做范围校验（10-86400）。
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
		fmt.Printf(i18n.T(i18n.Msg.Sandbox.Refreshed)+"\n", args[0], sandboxRefreshDuration)
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
		// 获取连接凭证（paused 沙箱会被自动恢复），不设置沙箱 TTL。
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
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.NoProcessID))
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
		`i18n.T(i18n.Msg.Sandbox.FlagMetadataFilter)`)
	sandboxListCmd.Flags().StringSliceVarP(&sandboxListStates, "state", "s", nil,
		i18n.T(i18n.Msg.Sandbox.FlagState))
	sandboxListCmd.Flags().Int32VarP(&sandboxListLimit, "limit", "l", 100,
		i18n.T(i18n.Msg.Sandbox.FlagLimit))
	sandboxListCmd.Flags().StringVarP(&sandboxListFormat, "format", "f", "pretty",
		i18n.T(i18n.Msg.Sandbox.FlagFormat))

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
