package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRestoreConfigCommand()) }

func newRestoreConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore-config <zip-file> [tool...]",
		Short: "Restore the shared config volume from a zip backup",
		Long: `devcontainer-cli restore-config — load a zip file produced by 'backup-config'
back into the shared config volume (` + types.SharedConfigVolumeName + `).

Pass one or more tool ids to restore only those; with no arguments every entry
present in the zip is restored. By default only entries with no data in the
volume are filled, so existing logins are never clobbered — use --force to
overwrite them. Restart (or start) containers afterwards to pick up the restored
config.

Known tools: ` + strings.Join(types.SharedConfigIDs(), ", ") + `.`,
		Example: `  # Restore everything from a backup, without touching existing data
  devcontainer-cli restore-config shared-config-backup.zip

  # Restore only specific tools, overwriting existing volume data
  devcontainer-cli restore-config shared-config-backup.zip claude gh --force --yes`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE:         runRestoreConfig,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return types.SharedConfigIDs(), cobra.ShellCompDirectiveNoFileComp
		},
	}
	cmd.Flags().Bool("force", false, "Replace entries that already have data in the volume with the zip's copy")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runRestoreConfig(cmd *cobra.Command, args []string) error {
	zipPath := args[0]
	if _, statErr := os.Stat(zipPath); statErr != nil {
		return fmt.Errorf("zip file not found: %s", zipPath)
	}
	entries, err := resolveSharedConfigArgs(args[1:])
	if err != nil {
		return err
	}
	force, _ := cmd.Flags().GetBool("force")

	console.Header("\nRestore %s → %s", zipPath, types.SharedConfigVolumeName)
	console.NewLine()

	// Replacing volume data with the zip's copy is destructive; plain runs
	// only fill empty entries and need no confirmation.
	if force && !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("--force replaces volume data; pass --yes to confirm in non-interactive mode")
		}
		proceed, perr := console.ConfirmDefault("Replace existing volume data with the zip's copies?", false)
		if perr != nil {
			return perr
		}
		if !proceed {
			console.Cancelled()
			return nil
		}
	}

	svc := service.SharedConfigService{Report: console}
	res, err := svc.RestoreFromZip(zipPath, entries, force)
	if err != nil {
		return err
	}

	console.NewLine()
	console.Success("Restored %d entr%s from the backup (%d skipped, %d not in the zip).",
		len(res.Restored), pluralY(len(res.Restored)), len(res.Skipped), len(res.Missing))
	if len(res.Restored) > 0 {
		console.Info("Restart containers (or start new ones) to pick up the restored config symlinks.")
	}
	return nil
}
