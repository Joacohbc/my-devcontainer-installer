package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newPruneCommand()) }

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove orphan devcontainer-cli/* images",
		Long: `devcontainer-cli prune — remove devcontainer-cli images whose projects no longer exist

By default removes only orphan images (project directory is gone).
Use --all to remove every devcontainer-cli/* image regardless.`,
		SilenceUsage: true,
		RunE:         runPrune,
	}
	cmd.Flags().Bool("all", false, "Remove ALL devcontainer-cli/* images, not just orphans")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runPrune(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")

	svc := service.PruneService{Report: ui.Console{}}
	toRemove, anyExist := svc.SelectImages(all)
	if !anyExist {
		fmt.Println(ui.Subtle("No devcontainer-cli/* images found locally."))
		return nil
	}
	if len(toRemove) == 0 {
		fmt.Println(ui.Subtle("No orphan devcontainer-cli/* images found."))
		fmt.Println(ui.Subtle("Use --all to remove every devcontainer-cli/* image."))
		return nil
	}

	ui.Yellow("\nImages to remove (%d):", len(toRemove))
	for _, img := range toRemove {
		fmt.Printf(ui.Subtle("  %s  (%s)\n"), img.Ref, img.ID)
	}
	println()

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("cannot prune images in non-interactive mode without --yes")
		}
		proceed, err := ui.Confirm("Remove these images?", false)
		if err != nil {
			return err
		}
		if !proceed {
			ui.Cancelled()
			return nil
		}
	}

	removed, failed := svc.Remove(toRemove)
	msg := ui.GreenS("\nRemoved %d image(s).", removed)
	if failed > 0 {
		msg += " " + ui.RedS("%d failed.", failed)
	}
	println(msg)
	return nil
}
