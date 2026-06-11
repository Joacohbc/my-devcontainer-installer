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
		Short: "Copy host tool configs into the shared config volume",
		Long: `devcontainer-cli sync-config — seed the shared tool-config volume (` + types.SharedConfigVolumeName + `)
with the configs already present on this machine, so containers start logged in
without redoing any setup.

Tools: ` + strings.Join(types.SharedConfigIDs(), ", ") + ` (default: all of them).

By default only entries with no data in the volume are copied; --force replaces
existing volume data with the host copy.

For ad-hoc folder syncing with a running container use rsync over SSH instead
(rsync ships in the image): rsync -av -e ssh ./dir <ssh-alias>:/workspace/dir
(run 'devcontainer-cli setup-ssh' first to register the alias).`,
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
