package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newNetworkCommand()) }

func newNetworkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Attach/detach containers to the workspace network",
		Long: `devcontainer-cli network — wire containers onto the current workspace network

The workspace network is the managed bridge network docker compose created for
the project (named "<workspace>-network"). Any existing container — managed by
this CLI or not — can be attached so it can reach the devcontainer by service
name; it does not need to be recreated.

  network connect <container...>     attach containers to the workspace network
  network disconnect <container...>  detach containers from the workspace network`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newNetworkConnectCommand())
	cmd.AddCommand(newNetworkDisconnectCommand())
	return cmd
}

func newNetworkConnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "connect <container...>",
		Short:             "Attach containers to the workspace network",
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      true,
		RunE:              func(cmd *cobra.Command, args []string) error { return runNetworkOp(cmd, args, "connect") },
		ValidArgsFunction: completeAnyContainerArgs,
	}
	addWorkspaceFlag(cmd)
	return cmd
}

func newNetworkDisconnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "disconnect <container...>",
		Short:             "Detach containers from the workspace network",
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      true,
		RunE:              func(cmd *cobra.Command, args []string) error { return runNetworkOp(cmd, args, "disconnect") },
		ValidArgsFunction: completeAnyContainerArgs,
	}
	addWorkspaceFlag(cmd)
	return cmd
}

// runNetworkOp resolves the workspace network and connects/disconnects the given
// containers, reporting the tally. verb is "connect" or "disconnect".
func runNetworkOp(cmd *cobra.Command, args []string, verb string) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	workspace := resolveWorkspace(cwd, workspaceFlag(cmd))

	svc := service.NetworkService{Report: console}
	if err := svc.EnsureDocker(); err != nil {
		return err
	}

	var ok, failed int
	if verb == "connect" {
		ok, failed, err = svc.Connect(workspace, args)
	} else {
		ok, failed, err = svc.Disconnect(workspace, args)
	}
	if err != nil {
		return err
	}

	past := "Connected"
	if verb == "disconnect" {
		past = "Disconnected"
	}
	console.Success("\n%s %d container(s).", past, ok)
	if failed > 0 {
		console.Error("%d failed.", failed)
		return fmt.Errorf("%d container(s) failed to %s", failed, verb)
	}
	return nil
}

// completeAnyContainerArgs completes positional container names from every
// container on the daemon (managed or not), skipping names already on the line.
func completeAnyContainerArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, name := range (service.NetworkService{Report: console}).ContainerNames() {
		if strings.HasPrefix(name, toComplete) && !slices.Contains(args, name) {
			out = append(out, name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
