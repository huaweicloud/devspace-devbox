// Command devbox is the command-line client for the DevBox Sandbox Manager.
package main

import (
	"os"
	"path/filepath"

	devboxcmd "github.com/huaweicloud/devspace-devbox/cli/cmd"
)

func main() {
	devboxcmd.RootCmd.Use = filepath.Base(os.Args[0])
	if err := devboxcmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
