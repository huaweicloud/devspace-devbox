// Package cmd defines the cobra command tree for the devbox CLI.
//
// Each command is a package-level var (e.g. loginCmd), registered to its parent via AddCommand in each file's init().
// Business errors are uniformly wrapped by runError and printed; main only handles the ExitCode.
package cmd

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/auth"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
	"github.com/huaweicloud/devspace-devbox/cli/internal/config"
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
	Long:    i18n.T(i18n.Msg.Common.RootLong) + config.DefaultServer + "\n",
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

func init() {
	// Global verbose switch (-v): enables debug logs (including API requests/responses).
	// After registering -v, cobra's --version flag no longer occupies the -v shorthand.
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, i18n.T(i18n.Msg.Common.FlagVerbose))

	// --version outputs only the version string itself (the default template adds a "devbox.exe version " prefix).
	RootCmd.SetVersionTemplate("{{.Version}}\n")

	RootCmd.AddCommand(authCmd, sandboxCmd, templateCmd)
}

// runError wraps RunE: on error it silences cobra's usage/error duplication and prints to stderr.
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

// newClient builds the API client. The API key resolution order mirrors devbridge: command line
// override > HW_DEVBOX_API_KEY env var > stored credentials (keyring first, config file fallback),
// see auth.ReadValidAPIKey. An unregistered --timeout falls back to the default 30s.
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

// strFromPtr converts a nullable string to a display value.
func strFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
