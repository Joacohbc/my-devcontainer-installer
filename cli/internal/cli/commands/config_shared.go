package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/spf13/cobra"
)

// The shared-config group is attached under `config` (see newConfigCommand); it
// is not registered as a top-level command.
func newConfigSharedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shared",
		Short: "Manage the shared tool-config volume (logins/sessions reused by every container)",
		Long: `devcontainer-cli config shared — seed, back up and restore the shared config
volume (` + types.SharedConfigVolumeName + `), the daemon-level Docker volume that
holds the tool logins/sessions (e.g. ~/.claude, ~/.config/gh, …) reused by every
container that mounts it.

These operations run on the host and act on the whole volume, independently of any
single project. Use them to bring existing host logins into the volume, snapshot it
before wiping, or move it to another machine.

Subcommands:
  sync                 Seed the volume from this machine's tool configs.
  backup               Back up the volume to a zip file.
  restore              Restore the volume from a zip made by 'backup'.`,
		Example: `  devcontainer-cli config shared sync
  devcontainer-cli config shared backup -o shared-config-backup.zip
  devcontainer-cli config shared restore shared-config-backup.zip`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newSyncConfigCommand())
	cmd.AddCommand(newBackupConfigCommand())
	cmd.AddCommand(newRestoreConfigCommand())
	return cmd
}
