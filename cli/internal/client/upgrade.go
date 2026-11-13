package client

import (
	"fmt"
	"runtime"
)

// One-click install/upgrade script URLs per channel, used by the HTTP 426
// upgrade notice to tell the user how to upgrade the CLI.
const (
	obsInstallSh     = "https://tools-artifact.developer.huaweicloud.com/sharedata/devbox/install.sh"
	obsInstallPs     = "https://tools-artifact.developer.huaweicloud.com/sharedata/devbox/install.ps1"
	gitcodeInstallSh = "https://gitcode.com/CloudDeveloperDepartment/devbox/releases/download/latest/install.sh"
	gitcodeInstallPs = "https://gitcode.com/CloudDeveloperDepartment/devbox/releases/download/latest/install.ps1"
	githubInstallSh  = "https://github.com/huaweicloud/devspace-devbox/releases/latest/download/install.sh"
	githubInstallPs  = "https://github.com/huaweicloud/devspace-devbox/releases/latest/download/install.ps1"
)

// installCommands returns the one-liner install/upgrade commands for the
// current OS, ordered GitHub, GitCode, OBS.
func installCommands() string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("  # GitHub\n  irm %s | iex\n\n  # GitCode\n  irm %s | iex\n\n  # OBS\n  irm %s | iex",
			githubInstallPs, gitcodeInstallPs, obsInstallPs)
	}
	return fmt.Sprintf("  # GitHub\n  curl -fsSL %s | bash\n\n  # GitCode\n  curl -fsSL %s | bash\n\n  # OBS\n  curl -fsSL %s | bash",
		githubInstallSh, gitcodeInstallSh, obsInstallSh)
}
