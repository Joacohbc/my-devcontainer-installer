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
		Short:   "Remove managed containers by label",
		Long: `devcontainer-cli remove-container — remove CLI-managed containers selected by the managed label

With one or more container names, removes exactly those (tab-completion
suggests managed container names). With no names, removes containers that are
not running, or every managed container with --all.`,
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
