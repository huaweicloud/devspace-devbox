// Package cmd defines the cobra command tree for the devbox CLI.
//
// Each command is a package-level var (e.g. loginCmd), registered to its parent via AddCommand in each file's init().
// Business errors are uniformly wrapped by runError and printed; main only handles the ExitCode.
package cmd

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/auth"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
	"github.com/huaweicloud/devspace-devbox/cli/internal/config"
	"github.com/huaweicloud/devspace-devbox/cli/internal/helpers"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
	"github.com/huaweicloud/devspace-devbox/cli/internal/output"

	"github.com/spf13/cobra"
)

// Version is injected via -ldflags "-X .../cmd.Version=...".
var Version = "dev"

// RootCmd is the root of all subcommands.
var RootCmd = &cobra.Command{
	Use:     "devbox",
	Short:   i18n.T(i18n.Msg.Common.RootShort),
	Long:    i18n.T(i18n.Msg.Common.RootLong),
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if f := cmd.Flags().Lookup("format"); f != nil {
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

// usageTemplate 基于 cobra v1.10.2 的 defaultUsageTemplate（command.go），仅移除
// Aliases 段：Use 里的管道写法（如 sandbox|sbx、create|cr）已经向用户展示了
// 缩写，而 Aliases 行的第一项是 Name() 派生的 "sandbox|sbx"，会与后面的别名
// 重复。SetUsageTemplate 在根命令上设置后由全部子命令继承。
// 注意：升级 cobra 版本时，需对照其 defaultUsageTemplate 同步本模板。
const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func init() {
	client.UserAgent = "devbox-cli/" + Version
	// Global verbose switch (-v): enables debug logs (including API requests/responses).
	// After registering -v, cobra's --version flag no longer occupies the -v shorthand.
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, i18n.T(i18n.Msg.Common.FlagVerbose))

	// --version outputs only the version string itself (the default template adds a "devbox.exe version " prefix).
	RootCmd.SetVersionTemplate("{{.Version}}\n")

	// 隐藏 Aliases 段，避免 "sandbox|sbx, sandbox, sbx" 这类重复展示（见 usageTemplate 注释）。
	RootCmd.SetUsageTemplate(usageTemplate)

	RootCmd.AddCommand(authCmd, sandboxCmd, templateCmd)
}

// runError wraps RunE: on error it silences cobra's usage/error duplication and prints to stderr,
// except for exit-code carriers which are handled (and exited on) by main.
func runError(fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := fn(cmd, args)
		if err != nil {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if _, ok := errors.AsType[*helpers.ExitCodeError](err); !ok {
				cmd.PrintErrln(err)
			}
		}
		return err
	}
}

// newClient builds the API client. The API key resolution order: command line
// override > HW_DEVBOX_API_KEY env var > stored credentials (keyring first, config file fallback),
// see auth.ReadValidAPIKey.
func newClient() (*client.Client, error) {
	apiKey := ""
	if cred := auth.ReadValidAPIKey(); cred != nil {
		apiKey = cred.APIKey
	}
	return client.New(config.DefaultServer, apiKey, &http.Client{Timeout: 30 * time.Second})
}

// out returns the format for the command's --format flag (unregistered or invalid falls back to pretty/text).
func out(cmd *cobra.Command) output.Format {
	f, err := cmd.Flags().GetString("format")
	if err != nil {
		return output.FormatText
	}
	format, err := output.ParseFormat(f)
	if err != nil {
		return output.FormatText
	}
	return format
}
