package commands

import (
	"fmt"
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newPasswordCommand()) }

func newPasswordCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "password",
		Short: "Show or change the devuser password in a container",
		Long: `devcontainer-cli password — manage the 'devuser' login password inside a running
devcontainer.

Subcommands:
  show     Reveal the initial password generated when the container was created.
  change   Set a new devuser password.`,
		Example: `  devcontainer-cli password show
  devcontainer-cli password change`,
	}
	cmd.AddCommand(newPasswordShowCommand())
	cmd.AddCommand(newPasswordChangeCommand())
	return cmd
}

func newPasswordShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the initial devuser password for a container",
		Long: `devcontainer-cli password show — print the initial devuser password that was
generated when the container was created.

This is the ONLY password the CLI can reveal: Linux stores credentials as a
one-way hash, so a password later set via 'password change' cannot be retrieved.
The password is printed to stdout (the explanatory note goes to stderr) so it can
be piped.`,
		Example: `  devcontainer-cli password show
  devcontainer-cli password show --container myproject-ssh`,
		SilenceUsage: true,
		RunE:         runPasswordShow,
	}
	addContainerFlag(cmd)
	return cmd
}

func newPasswordChangeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "change",
		Short: "Change the devuser password in a container",
		Long: `devcontainer-cli password change — set a new password for the devuser inside a
running container.

Interactively you are prompted for the new password (hidden input); with
--no-interactive pass it via --password. Once changed, the new password is hashed
by the OS and can no longer be revealed by 'password show'.`,
		Example: `  # Prompt for the new password
  devcontainer-cli password change

  # Non-interactive
  devcontainer-cli password change --password 's3cret' --no-interactive`,
		SilenceUsage: true,
		RunE:         runPasswordChange,
	}
	addContainerFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().String("password", "", "New password (non-interactive; omit to be prompted)")
	return cmd
}

func runPasswordShow(cmd *cobra.Command, _ []string) error {
	containerName, err := resolveContainer(cmd)
	if err != nil {
		return err
	}
	svc := service.PasswordService{Report: ui.Console{}}
	password, err := svc.InitialPassword(containerName)
	if err != nil {
		return err
	}
	fmt.Println(password)
	fmt.Fprintln(os.Stderr, ui.Subtle("Initial password only — a password changed via 'password change' is hashed and cannot be retrieved."))
	return nil
}

func runPasswordChange(cmd *cobra.Command, _ []string) error {
	containerName, err := resolveContainer(cmd)
	if err != nil {
		return err
	}

	newPassword, _ := cmd.Flags().GetString("password")
	svc := service.PasswordService{Report: ui.Console{}}

	if newPassword != "" {
		return svc.ChangePasswordNonInteractive(containerName, newPassword)
	}

	if !interactiveFlag(cmd) {
		return fmt.Errorf("--password is required in non-interactive mode")
	}

	return svc.ChangePassword(containerName)
}
