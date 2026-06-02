package commands

import (
	"fmt"

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
		Use:          "show",
		Short:        "Show the devuser /etc/shadow entry (hash and aging info)",
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
	entry, err := svc.ShowPassword(containerName)
	if err != nil {
		return err
	}
	fmt.Println(entry)
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
