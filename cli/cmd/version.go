package cmd

import (
	"fmt"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"

	"github.com/spf13/cobra"
)

// versionCmd implements `devbox version`: prints the version string exactly
// (the install scripts compare against this output).
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: i18n.T(i18n.Msg.Common.VersionShort),
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), Version)
		return nil
	}),
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
