package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRemoveImageCommand()) }

func newRemoveImageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove-image",
		Aliases: []string{"rmi"},
		Short:   "Remove managed images by label",
		Long: `devcontainer-cli remove-image — remove CLI-managed images selected by the managed label

By default only removes images that are not in use (no container references
them). Use --all to remove every managed image regardless.`,
		SilenceUsage: true,
		RunE:         runRemoveImage,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runRemoveImage(cmd *cobra.Command, _ []string) error {
	svc := service.PruneService{Report: console}
	return pruneOne(cmd, "images", svc.SelectImages,
		func(i service.LocalImage) string { return fmt.Sprintf("%s  (%s)", i.Ref, i.ID) },
		svc.Remove)
}
