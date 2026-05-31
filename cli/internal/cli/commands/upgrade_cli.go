package commands

import (
	"fmt"
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newUpgradeCliCommand()) }

func newUpgradeCliCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-cli",
		Short: "Replace this binary with the latest GitHub release",
		Long: `devcontainer-cli upgrade-cli — replace the current binary with the latest release

Env:
  GITHUB_TOKEN  Optional, to avoid the 60 req/hour anonymous rate limit`,
		SilenceUsage: true,
		RunE:         runSelfUpdate,
	}
	cmd.Flags().Bool("check", false, "Print current vs latest and exit (no download)")
	cmd.Flags().Bool("force", false, "Reinstall even if the current version matches latest")
	return cmd
}

func runSelfUpdate(cmd *cobra.Command, _ []string) error {
	check, _ := cmd.Flags().GetBool("check")
	force, _ := cmd.Flags().GetBool("force")

	svc := service.UpgradeService{Report: ui.Console{}}

	fmt.Fprintln(os.Stderr, ui.Subtle("Note: 'devcontainer-cli update' now manages container images. Self-update lives at 'devcontainer-cli upgrade-cli'."))

	current := version
	fmt.Printf(ui.Subtle("Current version: %s\n"), current)
	fmt.Println(ui.Subtle("Fetching latest release..."))
	rel, err := svc.LatestRelease()
	if err != nil {
		return err
	}
	latest := rel.Tag
	fmt.Printf(ui.Subtle("Latest version : %s\n"), latest)

	cmp := -1
	if current != "dev" {
		cmp = svc.CompareVersions(current, latest)
	}
	if check {
		switch {
		case cmp < 0:
			ui.Yellow("Update available: %s → %s", current, latest)
		case cmp == 0:
			ui.Green("Already up to date.")
		default:
			fmt.Println(ui.Subtle("Current version is ahead of latest release."))
		}
		return nil
	}

	if cmp >= 0 && !force {
		ui.Green("Already up to date.")
		return nil
	}

	if err := svc.Install(rel); err != nil {
		return err
	}

	ui.Green("Updated to %s. Restart any running session to use the new binary.", latest)
	return nil
}
