package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRemoveImageCommand()) }

func newRemoveImageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove-image [ref...]",
		Aliases: []string{"rmi"},
		Short:   "Remove managed images by label",
		Long: `devcontainer-cli remove-image — remove CLI-managed images selected by the managed label

With one or more image references, removes exactly those (tab-completion
suggests managed image references). With no refs, removes images that are not
in use, or every managed image with --all.`,
		SilenceUsage:      true,
		RunE:              runRemoveImage,
		ValidArgsFunction: completeManagedImageArgs,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func imageLabel(i service.LocalImage) string { return fmt.Sprintf("%s  (%s)", i.Ref, i.ID) }

func runRemoveImage(cmd *cobra.Command, args []string) error {
	svc := service.PruneService{Report: console}
	if len(args) > 0 {
		return removeNamed(cmd, "images", args, svc.SelectImages,
			func(i service.LocalImage) string { return i.Ref },
			imageLabel, svc.Remove)
	}
	return pruneOne(cmd, "images", svc.SelectImages, imageLabel, svc.Remove)
}
