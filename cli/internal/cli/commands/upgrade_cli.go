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

By default this installs the latest stable release. Pre-releases
are opt-in: when one is newer than the installed version you are
asked whether to install it, otherwise the latest stable is used. Pass
--pre-release to install it without prompting (and in non-interactive mode).

--force reinstalls the selected target even when it matches the installed
version, so --pre-release --force reinstalls the latest pre-release.

Env:
  GITHUB_TOKEN  Optional, to avoid the 60 req/hour anonymous rate limit`,
		SilenceUsage: true,
		RunE:         runSelfUpdate,
	}
	cmd.Flags().Bool("check", false, "Print current vs latest and exit (no download)")
	cmd.Flags().Bool("force", false, "Reinstall even if the current version matches latest")
	cmd.Flags().Bool("pre-release", false, "Install the latest pre-release (testing) if it is newer")
	addInteractiveFlag(cmd)
	return cmd
}

func runSelfUpdate(cmd *cobra.Command, _ []string) error {
	check, _ := cmd.Flags().GetBool("check")
	force, _ := cmd.Flags().GetBool("force")
	wantPreRelease, _ := cmd.Flags().GetBool("pre-release")
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

	// The normal upgrade target is the latest stable release; pre-releases are
	// opt-in. An explicit --pre-release targets the newest release overall
	// (top), regardless of whether it is strictly newer, so --force reinstalls
	// the pre-release rather than the stable build.
	preReleaseAvailable := svc.PreReleaseAvailable(current, top)
	target := svc.ResolveTarget(top, stable, wantPreRelease)

	if check {
		printUpgradeCheck(svc, current, target)
		if preReleaseAvailable && !wantPreRelease {
			console.Warn("Pre-release version available: %s (install with --pre-release)", top.Tag)
		}
		return nil
	}

	// Offer a newer pre-release only when the user didn't already opt in.
	if preReleaseAvailable && !wantPreRelease {
		if interactive {
			ok, err := console.ConfirmDefault(
				fmt.Sprintf("Pre-release version %s is available (latest stable: %s). Install the pre-release version?", top.Tag, tagOrNone(stable)),
				false,
			)
			if err != nil {
				return err
			}
			if ok {
				target = top
			}
		} else {
			console.Info("Pre-release version %s available; installing latest stable (pass --pre-release to opt in).", top.Tag)
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
