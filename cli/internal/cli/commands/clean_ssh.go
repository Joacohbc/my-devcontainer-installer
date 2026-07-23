package commands

import (
	"fmt"
	"strconv"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newCleanSshCommand()) }

func newCleanSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean-ssh",
		Short: "Remove ~/.ssh/config blocks whose workspace or container no longer exists",
		Long: `devcontainer-cli clean-ssh — prune stale SSH config entries this CLI wrote.

setup-ssh tags every Host block it adds to ~/.ssh/config with a managed marker
recording what it targets (a workspace or a specific container). This command scans
those markers and removes the blocks whose target is gone — a workspace that was
destroyed, or a container that no longer exists — leaving live entries and any
blocks you wrote yourself untouched. Unlike 'destroy', which removes the one block
for the current project, clean-ssh sweeps every stale managed block at once.

A workspace block is kept while a managed container reports it or the project is
still recorded, so a merely stopped ('down') stack is not cleaned. Removals are
listed first; in interactive mode you then pick which of the stale blocks to
remove (all are pre-selected, so deselect any you want to keep) instead of an
all-or-nothing confirmation, and the original config is backed up alongside it.
--yes skips the picker and removes every stale block, which is also what
happens in --no-interactive mode.`,
		Example: `  # Show what would be removed, change nothing
  devcontainer-cli clean-ssh --dry-run

  # Pick which stale blocks to remove
  devcontainer-cli clean-ssh

  # Remove all stale blocks without prompting
  devcontainer-cli clean-ssh --yes`,
		SilenceUsage: true,
		RunE:         runCleanSsh,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("dry-run", false, "List stale blocks without removing them")
	return cmd
}

func runCleanSsh(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	ssh := service.SshService{Report: console}

	existsWorkspace, existsContainer, err := ssh.LiveTargetPredicates()
	if err != nil {
		return err
	}

	// Preview first (dryRun=true never mutates), so we can report and confirm before
	// touching the file.
	stale, _, err := ssh.PruneManagedBlocks(existsWorkspace, existsContainer, true)
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		console.Success("No stale SSH config blocks found.")
		return nil
	}

	console.Info("Found %d stale SSH config block(s):", len(stale))
	for _, b := range stale {
		console.Print(fmt.Sprintf("  - Host %s  (%s %s)\n", b.Alias, b.Kind, b.Ref))
	}

	if dryRun {
		console.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return nil
	}

	toRemove := stale
	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("refusing to remove SSH config blocks without confirmation; pass --yes to confirm in non-interactive mode")
		}
		choices := make([]service.Option, len(stale))
		for i, b := range stale {
			choices[i] = service.Option{
				Value: strconv.Itoa(i),
				Label: fmt.Sprintf("Host %s  (%s %s)", b.Alias, b.Kind, b.Ref),
			}
		}
		picked, err := console.Multiselect("Select SSH config blocks to remove:", choices, choices)
		if err != nil {
			return err
		}
		if len(picked) == 0 {
			console.Warn("Cancelled.")
			return nil
		}
		toRemove = make([]service.ManagedMarker, len(picked))
		for i, p := range picked {
			idx, err := strconv.Atoi(p.Value)
			if err != nil {
				return err
			}
			toRemove[i] = stale[idx]
		}
	}

	removed, backup, err := ssh.RemoveManagedBlocks(toRemove)
	if err != nil {
		return err
	}
	if backup != "" {
		console.Ok(fmt.Sprintf("Backup saved: %s", backup))
	}
	console.Success("Removed %d SSH config block(s) from ~/.ssh/config.", len(removed))
	return nil
}
