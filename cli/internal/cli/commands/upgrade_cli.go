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
		Short: "Upgrade the CLI to the latest GitHub release",
		Long: `devcontainer-cli upgrade-cli — downloads and replaces the current binary with the latest available release from GitHub.

This provides a seamless, self-updating mechanism, ensuring you always have access to the latest features, bug fixes, and performance improvements of the devcontainer-cli tool itself.

Scope: Global CLI

By default this installs the latest stable release. Pre-releases
are opt-in: when one is newer than the installed version you are
asked whether to install it, otherwise the latest stable is used. Pass
--pre-release to install it without prompting (and in non-interactive mode).

Examples:
  devcontainer-cli upgrade-cli
  devcontainer-cli upgrade-cli --check
  devcontainer-cli upgrade-cli --pre-release
  devcontainer-cli upgrade-cli --force -y

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

	// The normal upgrade target is the latest stable release; pre-releases
	// are opt-in. Fall back to top only when no stable release exists.
	target := stable
	if target == nil {
		target = top
	}

	newerThanCurrent := func(rel *service.Release) bool {
		return rel != nil && (current == "dev" || svc.CompareVersions(current, rel.Tag) < 0)
	}
	preReleaseAvailable := top.Prerelease && newerThanCurrent(top)

	if check {
		printUpgradeCheck(svc, current, target)
		if preReleaseAvailable {
			console.Warn("Pre-release version available: %s (install with --pre-release)", top.Tag)
		}
		return nil
	}

	if preReleaseAvailable {
		if !wantPreRelease && interactive {
			ok, err := console.ConfirmDefault(
				fmt.Sprintf("Pre-release version %s is available (latest stable: %s). Install the pre-release version?", top.Tag, tagOrNone(stable)),
				false,
			)
			if err != nil {
				return err
			}
			wantPreRelease = ok
		}
		switch {
		case wantPreRelease:
			target = top
		case !interactive:
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
