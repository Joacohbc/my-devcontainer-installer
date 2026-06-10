package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newPruneCommand()) }

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove unused devcontainer resources",
		Long: `devcontainer-cli prune — remove CLI-managed devcontainer resources (images, networks, volumes)

By default only removes resources that are not in use (no container references
them). Use --all to remove every managed resource regardless.

Run a subcommand to target a single resource type:
  prune images    remove managed images
  prune network   remove managed networks
  prune volume    remove managed volumes

See also the top-level 'remove-container' (alias rm) and 'remove-image'
(alias rmi) commands.`,
		SilenceUsage: true,
		RunE:         runPruneAll,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.AddCommand(newPruneImagesCommand())
	cmd.AddCommand(newPruneNetworkCommand())
	cmd.AddCommand(newPruneVolumeCommand())
	return cmd
}

// addAllFlag registers the --all flag shared by prune/rm/rmi.
func addAllFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "Remove ALL managed resources, not just the unused ones")
}

func allFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("all")
	return v
}

// pruneOne drives the select → list → confirm → remove flow for a single
// resource type. items are selected with the --all/-y/--no-interactive flags;
// label renders one item for the listing.
func pruneOne[T any](
	cmd *cobra.Command,
	kind string,
	selectFn func(bool) ([]T, bool),
	label func(T) string,
	removeFn func([]T) (int, int),
) error {
	items, anyExist := selectFn(allFlag(cmd))
	if !anyExist {
		console.Info("No devcontainer %s found locally.", kind)
		return nil
	}
	if len(items) == 0 {
		console.Info("No unused devcontainer %s found.", kind)
		console.Warn("Use --all to remove every managed %s.", kind)
		return nil
	}
	return confirmAndRemove(cmd, kind, items, label, removeFn)
}

// removeNamed removes exactly the managed items whose name matches one of names,
// erroring if any name does not match a managed resource. It powers the
// positional-argument form of remove-container/remove-image.
func removeNamed[T any](
	cmd *cobra.Command,
	kind string,
	names []string,
	selectFn func(bool) ([]T, bool),
	nameOf func(T) string,
	label func(T) string,
	removeFn func([]T) (int, int),
) error {
	managed, _ := selectFn(true)
	selected, missing := selectByNames(managed, names, nameOf)
	if len(missing) > 0 {
		return fmt.Errorf("no managed %s found matching: %s", kind, strings.Join(missing, ", "))
	}
	return confirmAndRemove(cmd, kind, selected, label, removeFn)
}

// selectByNames partitions items into those whose name matches one of names
// (preserving the order of names) and the names that matched nothing.
func selectByNames[T any](items []T, names []string, nameOf func(T) string) (selected []T, missing []string) {
	byName := make(map[string]T, len(items))
	for _, it := range items {
		byName[nameOf(it)] = it
	}
	for _, n := range names {
		if it, ok := byName[n]; ok {
			selected = append(selected, it)
		} else {
			missing = append(missing, n)
		}
	}
	return selected, missing
}

// confirmAndRemove lists items, asks for confirmation (honoring --yes /
// --no-interactive) and removes them, reporting the result.
func confirmAndRemove[T any](
	cmd *cobra.Command,
	kind string,
	items []T,
	label func(T) string,
	removeFn func([]T) (int, int),
) error {
	console.Warn("\n%s to remove (%d):", kind, len(items))
	for _, it := range items {
		console.Info("  %s", label(it))
	}
	console.NewLine()

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("cannot remove %s in non-interactive mode without --yes", kind)
		}
		proceed, err := console.ConfirmDefault(fmt.Sprintf("Remove these %s?", kind), false)
		if err != nil {
			return err
		}
		if !proceed {
			console.Cancelled()
			return nil
		}
	}

	removed, failed := removeFn(items)
	console.Success("\nRemoved %d %s.", removed, kind)
	if failed > 0 {
		console.Error("%d failed.", failed)
	}
	return nil
}

func newPruneImagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "images",
		Aliases:      []string{"image"},
		Short:        "Remove unused managed images",
		SilenceUsage: true,
		RunE:         runPruneImages,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runPruneImages(cmd *cobra.Command, _ []string) error {
	svc := service.PruneService{Report: console}
	return pruneOne(cmd, "images", svc.SelectImages, imageLabel, svc.Remove)
}

func newPruneNetworkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "network",
		Aliases:      []string{"networks"},
		Short:        "Remove unused managed networks",
		SilenceUsage: true,
		RunE:         runPruneNetwork,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runPruneNetwork(cmd *cobra.Command, _ []string) error {
	svc := service.PruneService{Report: console}
	return pruneOne(cmd, "networks", svc.SelectNetworks,
		func(n service.LocalNetwork) string { return n.Name },
		svc.RemoveNetworks)
}

func newPruneVolumeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "volume",
		Aliases:      []string{"volumes"},
		Short:        "Remove unused managed volumes",
		SilenceUsage: true,
		RunE:         runPruneVolume,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("shared", false, "Also remove the global shared tool-config volume (devcontainer-shared-config) — deletes saved logins/sessions for ALL containers")
	return cmd
}

func runPruneVolume(cmd *cobra.Command, _ []string) error {
	shared, _ := cmd.Flags().GetBool("shared")
	svc := service.PruneService{Report: console, IncludeSharedConfig: shared}
	return pruneOne(cmd, "volumes", svc.SelectVolumes,
		func(v service.LocalVolume) string { return v.Name },
		svc.RemoveVolumes)
}

// runPruneAll cleans images, networks and volumes in one pass (the bare `prune`
// command). Containers are handled by the dedicated `rm` command.
func runPruneAll(cmd *cobra.Command, _ []string) error {
	all := allFlag(cmd)

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
		console.Info("No unused devcontainer resources found.")
		console.Warn("Use --all to remove every managed resource.")
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
