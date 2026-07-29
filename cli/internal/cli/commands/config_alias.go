package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// The alias group is attached under `config` (see newConfigCommand); it is not
// registered as a top-level command.
func newConfigAliasCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage your own shell aliases for every container",
		Long: `devcontainer-cli config alias — manage the alias file that every container
picks up.

Containers source two files, in this order:

  ~/.devcontainer_aliases.sh   the CLI's defaults, baked into the image
                               (kill_port, npm->pnpm, pip->uv, prompt-free agents)
  ~/.alias.sh                  YOURS — this file

Because yours is sourced last it always wins, and because it lives in the shared
config volume it applies to every container without rebuilding any image and
without restarting anything — a change takes effect in the next shell.

Edit it whenever you like, from either side: inside a container ~/.alias.sh is a
symlink into that volume, so editing it there is immediately shared with every
other container. This command edits the host's copy; push it out with
'config shared sync alias.sh --force'.

With no subcommand this prints the file's path and contents.`,
		Example: `  # Show the current aliases
  devcontainer-cli config alias

  # Edit them, then push them to every container
  devcontainer-cli config alias edit
  devcontainer-cli config shared sync alias.sh --force

  # Start over from the commented template
  devcontainer-cli config alias reset`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runConfigAliasShow,
	}
	cmd.AddCommand(newConfigAliasEditCommand())
	cmd.AddCommand(newConfigAliasResetCommand())
	return cmd
}

func newConfigAliasEditCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open your alias file in $EDITOR",
		Long: `devcontainer-cli config alias edit — open your alias file in an editor,
creating it from a commented template when it does not exist yet.

The editor is $EDITOR, then $VISUAL, then nano. Edits only reach running
containers after 'config shared sync alias.sh --force'.`,
		Example:      `  EDITOR=vim devcontainer-cli config alias edit`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runConfigAliasEdit,
	}
}

func newConfigAliasResetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Overwrite your alias file with the default template",
		Long: `devcontainer-cli config alias reset — discard your alias file and rewrite it
from the commented template.

Destructive: whatever is in the file is lost. It only affects your own file; the
CLI's baked defaults are part of the image and are unaffected.`,
		Example:      `  devcontainer-cli config alias reset -y`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runConfigAliasReset,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runConfigAliasShow(_ *cobra.Command, _ []string) error {
	svc := service.ConfigService{Report: console}
	path, content, exists, err := svc.ReadAliasFile()
	if err != nil {
		return err
	}
	console.Info("%s", path)
	if !exists {
		console.Warn("Not created yet (run: config alias edit).")
		return nil
	}
	if strings.TrimSpace(content) == "" {
		console.Warn("(empty)")
		return nil
	}
	console.NewLine()
	console.Print(strings.TrimRight(content, "\n") + "\n")
	return nil
}

func runConfigAliasEdit(_ *cobra.Command, _ []string) error {
	svc := service.ConfigService{Report: console}
	path, _, err := svc.EnsureAliasFile()
	if err != nil {
		return err
	}

	editor := firstNonEmptyEnv("EDITOR", "VISUAL")
	if editor == "" {
		editor = "nano"
	}
	// The editor takes over the terminal, so it must inherit stdio.
	ed := exec.Command("sh", "-c", editor+" "+quoteShellArg(path))
	ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := ed.Run(); err != nil {
		return fmt.Errorf("editor %q failed: %w", editor, err)
	}

	console.Ok(fmt.Sprintf("Saved %s", path))
	console.Info("Apply it to every container with: devcontainer-cli config shared sync %s --force", types.SharedConfigAliasID)
	return nil
}

func runConfigAliasReset(cmd *cobra.Command, _ []string) error {
	svc := service.ConfigService{Report: console}
	path, _, exists, err := svc.ReadAliasFile()
	if err != nil {
		return err
	}
	if exists && !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("%s already exists; pass --yes to overwrite it in non-interactive mode", path)
		}
		proceed, cerr := console.ConfirmDefault(fmt.Sprintf("Overwrite %s with the default template?", path), false)
		if cerr != nil {
			return cerr
		}
		if !proceed {
			console.Warn("Cancelled.")
			return nil
		}
	}
	if _, err := svc.ResetAliasFile(); err != nil {
		return err
	}
	console.Info("Apply it to every container with: devcontainer-cli config shared sync %s --force", types.SharedConfigAliasID)
	return nil
}

// firstNonEmptyEnv returns the value of the first environment variable that is
// set and non-empty.
func firstNonEmptyEnv(names ...string) string {
	for _, n := range names {
		if v := strings.TrimSpace(os.Getenv(n)); v != "" {
			return v
		}
	}
	return ""
}

// quoteShellArg single-quotes a path so it survives being handed to `sh -c`
// alongside a user-supplied $EDITOR (which may itself carry flags).
func quoteShellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
