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
		Short:   "Remove CLI-managed images (by reference or in bulk)",
		Long: `devcontainer-cli remove-image — remove images created by this CLI, identified by
the managed label. Lists what will be removed and confirms first.

Give one or more image references to remove exactly those (refs tab-complete
managed images). With no refs it removes the managed images that are NOT in use
by any container, or — with --all — every managed image regardless.`,
		Example: `  # Remove unused managed images
  devcontainer-cli remove-image

  # Remove a specific image
  devcontainer-cli rmi devcontainer-cli/ab12cd34ef56:latest

  # Remove every managed image, no prompt
  devcontainer-cli rmi --all --yes`,
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
