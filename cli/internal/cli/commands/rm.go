package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRmCommand()) }

func newRmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove managed containers by label",
		Long: `devcontainer-cli rm — remove CLI-managed containers selected by the managed label

By default only removes containers that are not running. Use --all to remove
every managed container (running ones are force-removed).`,
		SilenceUsage: true,
		RunE:         runRm,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runRm(cmd *cobra.Command, _ []string) error {
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
