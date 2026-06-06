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
		Short: "Manage the devuser password in the running container",
	}
	cmd.AddCommand(newPasswordShowCommand())
	cmd.AddCommand(newPasswordChangeCommand())
	return cmd
}

func newPasswordShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the initial devuser password",
		Long: "devcontainer-cli password show — displays the initial devuser password generated when the container was created.\n\n" +
			"This is necessary if you need to perform actions requiring sudo inside the container or when setting up SSH access for the first time.\n" +
			"This is the only password the CLI can reveal: Linux stores credentials as a\n" +
			"one-way hash, so a password later set via 'password change' cannot be retrieved.\n\nScope: Container\n\nExamples:\n  devcontainer-cli password show\n  devcontainer-cli password show -w my-workspace",
		SilenceUsage: true,
		RunE:         runPasswordShow,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
	return cmd
}

func newPasswordChangeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "change",
		Short:        "Change the devuser password",
		Long:         "devcontainer-cli password change — changes the devuser password inside the running container.\n\nExamples:\n  devcontainer-cli password change\n  devcontainer-cli password change --password new_secure_password\n  devcontainer-cli password change -w my-workspace -c database",
		SilenceUsage: true,
		RunE:         runPasswordChange,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().String("password", "", "New password (non-interactive; omit to be prompted)")
	return cmd
}

func runPasswordShow(cmd *cobra.Command, _ []string) error {
	wsFlag := workspaceFlag(cmd)
	containerName, err := resolveContainer(cmd, wsFlag)
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
	wsFlag := workspaceFlag(cmd)
	containerName, err := resolveContainer(cmd, wsFlag)
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
