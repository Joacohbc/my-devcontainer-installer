package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newBackupConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup [tool...]",
		Short: "Back up the shared config volume to a zip file",
		Long: `devcontainer-cli config shared backup — save the current content of the shared
config volume (` + types.SharedConfigVolumeName + `) to a zip file on this machine, so
it can be restored later with 'config shared restore' (e.g. before wiping the volume,
or to move logins to another machine).

Pass one or more tool ids to back up only those; with no arguments every entry
currently present in the volume is included. Entries with no data in the volume
are silently skipped.

Known tools: ` + strings.Join(types.SharedConfigIDs(), ", ") + `.`,
		Example: `  # Back up everything to a timestamped zip
  devcontainer-cli config shared backup

  # Back up to a specific file
  devcontainer-cli config shared backup -o shared-config-backup.zip

  # Back up only specific tools
  devcontainer-cli config shared backup claude gh -o claude-gh-backup.zip`,
		SilenceUsage: true,
		RunE:         runBackupConfig,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return types.SharedConfigIDs(), cobra.ShellCompDirectiveNoFileComp
		},
	}
	cmd.Flags().StringP("output", "o", "", "Destination zip file (default: shared-config-backup-<timestamp>.zip in the current directory)")
	return cmd
}

func runBackupConfig(cmd *cobra.Command, args []string) error {
	entries, err := resolveSharedConfigArgs(args)
	if err != nil {
		return err
	}

	out, _ := cmd.Flags().GetString("output")
	if out == "" {
		out = fmt.Sprintf("shared-config-backup-%s.zip", time.Now().Format("20060102-150405"))
	}

	console.Header("\nBack up %s → %s", types.SharedConfigVolumeName, out)
	console.NewLine()

	svc := service.SharedConfigService{Report: console}
	res, err := svc.BackupToZip(entries, out)
	if err != nil {
		return err
	}

	console.NewLine()
	console.Success("Backed up %d entr%s to %s (%d not present in the volume).",
		len(res.Included), pluralY(len(res.Included)), out, len(res.Missing))
	return nil
}
