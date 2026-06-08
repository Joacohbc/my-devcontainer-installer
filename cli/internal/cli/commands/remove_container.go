package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRemoveContainerCommand()) }

func newRemoveContainerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove-container",
		Aliases: []string{"rm"},
		Short:   "Remove managed containers by label",
		Long: `devcontainer-cli remove-container — remove CLI-managed containers selected by the managed label

By default only removes containers that are not running. Use --all to remove
every managed container (running ones are force-removed).`,
		SilenceUsage: true,
		RunE:         runRemoveContainer,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runRemoveContainer(cmd *cobra.Command, _ []string) error {
	svc := service.PruneService{Report: console}
	return pruneOne(cmd, "containers", svc.SelectContainers,
		func(c service.LocalContainer) string {
			if c.State != "" {
				return c.Name + "  (" + c.State + ")"
			}
			return c.Name
		},
		svc.RemoveContainers)
}
