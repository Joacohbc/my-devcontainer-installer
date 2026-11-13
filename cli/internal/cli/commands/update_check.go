package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

const envNoUpdateCheck = "DEVCONTAINER_CLI_NO_UPDATE_CHECK"

var skipUpdateCheckCommands = map[string]bool{
	"__complete":       true,
	"__completeNoDesc": true,
	"completion":       true,
	"help":             true,
	"upgrade-cli":      true,
}

func shouldSkipStartupUpdateCheck(cmd *cobra.Command, currentVersion string) bool {
	if os.Getenv(envNoUpdateCheck) != "" {
		return true
	}
	if currentVersion == "" || currentVersion == "dev" || currentVersion == "test" {
		return true
	}
	if cmd == nil {
		return false
	}
	return skipUpdateCheckCommands[cmd.Name()]
}

func checkStartupUpdates(cmd *cobra.Command, currentVersion string, svc service.UpdateCheckService, errOut io.Writer) {
	if errOut == nil || shouldSkipStartupUpdateCheck(cmd, currentVersion) {
		return
	}
	status := svc.Check(currentVersion, false)
	for _, line := range service.FormatUpdateWarnings(status) {
		fmt.Fprintln(errOut, ui.YellowS("! %s", line))
	}
}

func reportHostSkillStatus(status service.UpdateStatus) {
	if !status.SkillChecked {
		return
	}
	if len(status.SkillPaths) == 0 {
		console.Info("Host skill     : not installed (install with '%s')", service.HostSkillInstallHint)
		return
	}
	if len(status.OutdatedPaths) > 0 {
		console.Warn("Host skill     : outdated at %s (update with '%s')",
			strings.Join(status.OutdatedPaths, ", "), service.HostSkillInstallHint)
		return
	}
	console.Success("Host skill     : up to date (%s)", strings.Join(status.SkillPaths, ", "))
}
