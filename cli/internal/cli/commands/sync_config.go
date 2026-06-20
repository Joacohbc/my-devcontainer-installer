package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newSyncConfigCommand()) }

func newSyncConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync-config [tool...]",
		Short: "Seed the shared config volume from this machine's tool configs",
		Long: `devcontainer-cli sync-config — copy the tool configs already on this machine
(e.g. ~/.claude, ~/.config/gh, …) into the shared config volume
(` + types.SharedConfigVolumeName + `), so any container that mounts it starts already
logged in, without you redoing each tool's setup.

Pass one or more tool ids to sync only those; with no arguments every known tool
is synced. Restart (or start) containers afterwards to pick up the seeded config.

Known tools: ` + strings.Join(types.SharedConfigIDs(), ", ") + `.

Flags:
  --force           Replace entries that ALREADY have data in the volume with the
                    host copy. Without it, only empty entries are filled (existing
                    volume data is never overwritten). This is destructive, so it
                    prompts for confirmation.
  -y, --yes         Skip the --force confirmation prompt (required to use --force
                    with --no-interactive).
      --no-interactive  Never prompt; --force without --yes errors out.`,
		Example: `  # Seed everything not already present in the volume
  devcontainer-cli sync-config

  # Sync only specific tools
  devcontainer-cli sync-config claude gh

  # Overwrite existing volume data with the host copy, unattended
  devcontainer-cli sync-config --force --yes`,
		SilenceUsage: true,
		RunE:         runSyncConfig,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return types.SharedConfigIDs(), cobra.ShellCompDirectiveNoFileComp
		},
	}
	cmd.Flags().Bool("force", false, "Replace entries that already have data in the volume with the host copy")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runSyncConfig(cmd *cobra.Command, args []string) error {
	entries, err := resolveSharedConfigArgs(args)
	if err != nil {
		return err
	}
	force, _ := cmd.Flags().GetBool("force")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve home directory: %w", err)
	}

	console.Header("\nSync host configs → %s", types.SharedConfigVolumeName)
	console.NewLine()
	for _, e := range entries {
		console.Info("  %s  (~/%s)", e.ID, e.Target)
	}
	console.NewLine()

	// Replacing volume data with the host copy is destructive; plain runs only
	// fill empty entries and need no confirmation.
	if force && !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("--force replaces volume data; pass --yes to confirm in non-interactive mode")
		}
		proceed, perr := console.ConfirmDefault("Replace existing volume data with the host copies?", false)
		if perr != nil {
			return perr
		}
		if !proceed {
			console.Cancelled()
			return nil
		}
	}

	svc := service.SharedConfigService{Report: console}
	res, err := svc.SyncFromHost(entries, home, force)
	if err != nil {
		return err
	}

	console.NewLine()
	console.Success("Synced %d entr%s into the shared volume (%d skipped, %d not on host).",
		len(res.Copied), pluralY(len(res.Copied)), len(res.Skipped), len(res.Missing))
	if len(res.Copied) > 0 {
		console.Info("Restart containers (or start new ones) to pick up the seeded config symlinks.")
	}
	return nil
}

func resolveSharedConfigArgs(args []string) ([]types.SharedConfigEntry, error) {
	if len(args) == 0 {
		return types.SharedConfigEntries, nil
	}
	var entries []types.SharedConfigEntry
	for _, id := range args {
		e, ok := types.SharedConfigEntryByID(id)
		if !ok {
			return nil, fmt.Errorf("unknown tool: %s. Expected one of: %s", id, strings.Join(types.SharedConfigIDs(), ", "))
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
