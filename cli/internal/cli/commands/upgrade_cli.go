package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newUpgradeCliCommand()) }

func newUpgradeCliCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-cli",
		Short: "Replace this binary with the latest GitHub release",
		Long: `devcontainer-cli upgrade-cli — replace the current binary with the latest release

By default this installs the latest stable release. Testing builds (tagged
vX.Y.Z-vt.N) are opt-in: when one is newer than the installed version you are
asked whether to install it, otherwise the latest stable is used. Pass
--testing to install it without prompting (and in non-interactive mode).

Env:
  GITHUB_TOKEN  Optional, to avoid the 60 req/hour anonymous rate limit`,
		SilenceUsage: true,
		RunE:         runSelfUpdate,
	}
	cmd.Flags().Bool("check", false, "Print current vs latest and exit (no download)")
	cmd.Flags().Bool("force", false, "Reinstall even if the current version matches latest")
	cmd.Flags().Bool("testing", false, "Install the latest testing (vt) build if it is newer")
	addInteractiveFlag(cmd)
	return cmd
}

func runSelfUpdate(cmd *cobra.Command, _ []string) error {
	check, _ := cmd.Flags().GetBool("check")
	force, _ := cmd.Flags().GetBool("force")
	wantTesting, _ := cmd.Flags().GetBool("testing")
	interactive := interactiveFlag(cmd)

	svc := service.UpgradeService{Report: console}

	console.Debug("Note: 'devcontainer-cli update' now manages container images. Self-update lives at 'devcontainer-cli upgrade-cli'.")

	current := version
	console.Info("Current version: %s", current)
	console.Info("Fetching releases...")
	top, stable, err := svc.UpgradeTargets()
	if err != nil {
		return err
	}

	// The normal upgrade target is the latest stable release; testing (vt)
	// builds are opt-in. Fall back to top only when no stable release exists.
	target := stable
	if target == nil {
		target = top
	}

	newerThanCurrent := func(rel *service.Release) bool {
		return rel != nil && (current == "dev" || svc.CompareVersions(current, rel.Tag) < 0)
	}
	testingAvailable := svc.IsTestingVersion(top.Tag) && newerThanCurrent(top)

	if check {
		printUpgradeCheck(svc, current, target)
		if testingAvailable {
			console.Warn("Testing version available: %s (install with --testing)", top.Tag)
		}
		return nil
	}

	if testingAvailable {
		if !wantTesting && interactive {
			ok, err := console.ConfirmDefault(
				fmt.Sprintf("Testing version %s is available (latest stable: %s). Install the testing version?", top.Tag, tagOrNone(stable)),
				false,
			)
			if err != nil {
				return err
			}
			wantTesting = ok
		}
		switch {
		case wantTesting:
			target = top
		case !interactive:
			console.Info("Testing version %s available; installing latest stable (pass --testing to opt in).", top.Tag)
		}
	}

	if target == nil {
		return fmt.Errorf("no installable release found")
	}

	cmp := -1
	if current != "dev" {
		cmp = svc.CompareVersions(current, target.Tag)
	}
	if cmp >= 0 && !force {
		console.Success("Already up to date.")
		return nil
	}

	if err := svc.Install(target); err != nil {
		return err
	}

	console.Success("Updated to %s. Restart any running session to use the new binary.", target.Tag)
	return nil
}

func printUpgradeCheck(svc service.UpgradeService, current string, target *service.Release) {
	if target == nil {
		console.Warn("No stable release found.")
		return
	}
	console.Info("Latest version : %s", target.Tag)
	cmp := -1
	if current != "dev" {
		cmp = svc.CompareVersions(current, target.Tag)
	}
	switch {
	case cmp < 0:
		console.Warn("Update available: %s → %s", current, target.Tag)
	case cmp == 0:
		console.Success("Already up to date.")
	default:
		console.Warn("Current version is ahead of latest release.")
	}
}

func tagOrNone(r *service.Release) string {
	if r == nil {
		return "(none)"
	}
	return r.Tag
}
