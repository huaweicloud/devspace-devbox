// Package cmd defines the cobra command tree for the devbox CLI.
//
// 每个命令都是包级 var（如 loginCmd、configGetCmd），通过各文件 init() 中的 AddCommand 注册到父命令。
// 业务错误统一由 runError 包装后打印，main 只负责 ExitCode。
package cmd

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/auth"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
	"github.com/huaweicloud/devspace-devbox/cli/internal/config"
	"github.com/huaweicloud/devspace-devbox/cli/internal/output"

	"github.com/spf13/cobra"
)

// Version 可通过 -ldflags "-X .../cmd.Version=..." 注入。
var Version = "dev"

// RootCmd 是所有子命令的根。
var RootCmd = &cobra.Command{
	Use:   "devbox",
	Short: "CLI for the DevBox sandbox manager",
	Long: `devbox is a command-line client for the DevBox Sandbox Manager API.

It covers the full sandbox lifecycle (create, list, info, kill, logs,
metrics, pause, resume, connect) and templates.

Configuration is resolved in this order:
  1. flags                --api-key (auth login)
  2. environment          HW_DEVBOX_API_KEY
  3. config file          ~/.devbox/config.json
  4. build-time default   API server URL injected via Makefile, currently ` + config.DefaultServer + `
`,
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if f := cmd.Flags().Lookup("output"); f != nil {
			if _, err := output.ParseFormat(f.Value.String()); err != nil {
				return err
			}
		}
		if verbose, err := cmd.Flags().GetBool("verbose"); err == nil && verbose {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
		}
		return nil
	},
}

func init() {
	// 全局 verbose 开关（-v）：开启后打印 debug 日志（含接口请求/返回）。
	// 注册 -v 后，cobra 的 --version 旗标不再占用 -v 简写。
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "print debug logs (API requests/responses)")

	RootCmd.AddCommand(authCmd, sandboxCmd, templateCmd)
}

// runError 包装 RunE：出错时关闭 cobra 的 usage/error 重复输出，统一打印到 stderr。
func runError(fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := fn(cmd, args)
		if err != nil {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.PrintErrln(err)
		}
		return err
	}
}

// newClient 构造 API 客户端。API Key 的解析顺序与 devbridge 一致：命令行
// override > HW_DEVBOX_API_KEY 环境变量 > 存储的凭据（keyring 优先、配置文件兜底），
// 见 auth.ReadValidAPIKey。--timeout 未注册时使用默认 30s。
func newClient(cmd *cobra.Command) (*client.Client, error) {
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil || timeout == 0 {
		timeout = 30 * time.Second
	}
	apiKey := ""
	if cred := auth.ReadValidAPIKey(); cred != nil {
		apiKey = cred.APIKey
	}
	return client.New(config.DefaultServer, apiKey, &http.Client{Timeout: timeout})
}

// out 返回当前命令 --output 对应的格式（未注册或非法时按 text 处理）。
func out(cmd *cobra.Command) output.Format {
	f, err := cmd.Flags().GetString("output")
	if err != nil {
		return output.FormatText
	}
	format, err := output.ParseFormat(f)
	if err != nil {
		return output.FormatText
	}
	return format
}

// strFromPtr 将可空字符串转为展示值。
func strFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
