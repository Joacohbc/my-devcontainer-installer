package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRemoveContainerCommand()) }

func newRemoveContainerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove-container [name...]",
		Aliases: []string{"rm"},
		Short:   "Remove CLI-managed containers (by name or in bulk)",
		Long: `devcontainer-cli remove-container — remove containers created by this CLI,
identified by the managed label. Lists what will be removed and confirms first.

Give one or more container names to remove exactly those (names tab-complete
managed containers). With no names it removes the managed containers that are NOT
running, or — with --all — every managed container regardless of state.`,
		Example: `  # Remove stopped managed containers
  devcontainer-cli remove-container

  # Remove specific containers by name
  devcontainer-cli rm myproject-postgres myproject-redis

  # Remove every managed container, no prompt
  devcontainer-cli rm --all --yes`,
		SilenceUsage:      true,
		RunE:              runRemoveContainer,
		ValidArgsFunction: completeManagedContainerArgs,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func containerLabel(c service.LocalContainer) string {
	if c.State != "" {
		return c.Name + "  (" + c.State + ")"
	}
	return c.Name
}

func runRemoveContainer(cmd *cobra.Command, args []string) error {
	svc := service.PruneService{Report: console}
	if len(args) > 0 {
		return removeNamed(cmd, "containers", args, svc.SelectContainers,
			func(c service.LocalContainer) string { return c.Name },
			containerLabel, svc.RemoveContainers)
	}
	return pruneOne(cmd, "containers", svc.SelectContainers, containerLabel, svc.RemoveContainers)
}
