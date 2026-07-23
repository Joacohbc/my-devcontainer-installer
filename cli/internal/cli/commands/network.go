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
		Short: "Attach or detach containers to the workspace network",
		Long: `devcontainer-cli network — wire arbitrary containers onto the current project's
workspace network so they can talk to the devcontainer.

The workspace network is the managed bridge network docker compose created for
the project (named "<workspace>-network"). Any existing container — managed by
this CLI or not — can be attached to it and will then reach the devcontainer by
service name (and vice-versa), without being recreated.

Subcommands:
  network connect <container...>     Attach containers to the workspace network.
  network disconnect <container...>  Detach containers from the workspace network.`,
		Example: `  devcontainer-cli network connect my-other-app
  devcontainer-cli network connect db --alias postgres
  devcontainer-cli network disconnect my-other-app`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newNetworkConnectCommand())
	cmd.AddCommand(newNetworkDisconnectCommand())
	return cmd
}

func newNetworkConnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect <container...>",
		Short: "Attach containers to the workspace network",
		Long: `devcontainer-cli network connect — attach one or more existing containers to the
current project's workspace network so they can reach the devcontainer by name.

Container names tab-complete from every container on the daemon. Use --alias to
register extra DNS names for them on the network.`,
		Example: `  devcontainer-cli network connect my-app
  devcontainer-cli network connect db --alias postgres,pg`,
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      true,
		RunE:              func(cmd *cobra.Command, args []string) error { return runNetworkOp(cmd, args, "connect") },
		ValidArgsFunction: completeAnyContainerArgs,
	}
	cmd.Flags().StringSlice("alias", nil, "Extra DNS alias(es) to register for the container(s) on the network; repeatable or comma-separated")
	addInteractiveFlag(cmd)
	return cmd
}

// connectAliases resolves the network aliases for a connect: the --alias flag
// when set, otherwise (in interactive mode) a single optional alias prompted
// from the user. An empty answer skips aliasing.
func connectAliases(cmd *cobra.Command) ([]string, error) {
	aliases, _ := cmd.Flags().GetStringSlice("alias")
	if len(aliases) > 0 || !interactiveFlag(cmd) {
		return aliases, nil
	}
	answer, err := console.AskDefault("Network alias for the container(s) (optional, leave empty to skip):", "", nil)
	if err != nil {
		return nil, err
	}
	return splitCSV(answer), nil
}

func newNetworkDisconnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect <container...>",
		Short: "Detach containers from the workspace network",
		Long: `devcontainer-cli network disconnect — detach one or more containers from the
current project's workspace network. Container names tab-complete.`,
		Example:           "  devcontainer-cli network disconnect my-app",
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      true,
		RunE:              func(cmd *cobra.Command, args []string) error { return runNetworkOp(cmd, args, "disconnect") },
		ValidArgsFunction: completeAnyContainerArgs,
	}
	return cmd
}

// runNetworkOp resolves the workspace network and connects/disconnects the given
// containers, reporting the tally. verb is "connect" or "disconnect".
func runNetworkOp(cmd *cobra.Command, args []string, verb string) error {
	cwd, err := currentDir()
	if err != nil {
		return err
	}
	workspace := resolveWorkspace(cwd)

	svc := service.NetworkService{Report: console}
	if err := svc.EnsureDocker(); err != nil {
		return err
	}

	var ok, failed int
	if verb == "connect" {
		aliases, aerr := connectAliases(cmd)
		if aerr != nil {
			return aerr
		}
		ok, failed, err = svc.Connect(workspace, args, aliases)
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
