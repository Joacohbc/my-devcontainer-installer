package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newPruneCommand()) }

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove orphan devcontainer resources",
		Long: `devcontainer-cli prune — remove devcontainer resources (images, networks, volumes) whose projects no longer exist

By default removes only orphan resources (project directory is gone).
Use --all to remove every devcontainer resource regardless.`,
		SilenceUsage: true,
		RunE:         runPrune,
	}
	cmd.Flags().Bool("all", false, "Remove ALL devcontainer resources, not just orphans")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runPrune(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")

	svc := service.PruneService{Report: console}
	toRemoveImgs, anyImgs := svc.SelectImages(all)
	toRemoveNets, anyNets := svc.SelectNetworks(all)
	toRemoveVols, anyVols := svc.SelectVolumes(all)

	if !anyImgs && !anyNets && !anyVols {
		console.Info("No devcontainer resources found locally.")
		return nil
	}

	totalToRemove := len(toRemoveImgs) + len(toRemoveNets) + len(toRemoveVols)
	if totalToRemove == 0 {
		console.Info("No orphan devcontainer resources found.")
		console.Warn("Use --all to remove every devcontainer resource.")
		return nil
	}

	if len(toRemoveImgs) > 0 {
		console.Warn("\nImages to remove (%d):", len(toRemoveImgs))
		for _, img := range toRemoveImgs {
			console.Info("  %s  (%s)", img.Ref, img.ID)
		}
	}
	if len(toRemoveNets) > 0 {
		console.Warn("\nNetworks to remove (%d):", len(toRemoveNets))
		for _, net := range toRemoveNets {
			console.Info("  %s", net.Name)
		}
	}
	if len(toRemoveVols) > 0 {
		console.Warn("\nVolumes to remove (%d):", len(toRemoveVols))
		for _, vol := range toRemoveVols {
			console.Info("  %s", vol.Name)
		}
	}
	console.NewLine()

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("cannot prune resources in non-interactive mode without --yes")
		}
		proceed, err := console.ConfirmDefault("Remove these resources?", false)
		if err != nil {
			return err
		}
		if !proceed {
			console.Cancelled()
			return nil
		}
	}

	removedImgs, failedImgs := 0, 0
	if len(toRemoveImgs) > 0 {
		removedImgs, failedImgs = svc.Remove(toRemoveImgs)
	}

	removedNets, failedNets := 0, 0
	if len(toRemoveNets) > 0 {
		removedNets, failedNets = svc.RemoveNetworks(toRemoveNets)
	}

	removedVols, failedVols := 0, 0
	if len(toRemoveVols) > 0 {
		removedVols, failedVols = svc.RemoveVolumes(toRemoveVols)
	}

	removedTotal := removedImgs + removedNets + removedVols
	failedTotal := failedImgs + failedNets + failedVols

	console.Success("\nRemoved %d resource(s).", removedTotal)
	if failedTotal > 0 {
		console.Error("%d failed.", failedTotal)
	}
	return nil
}
