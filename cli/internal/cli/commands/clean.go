package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newCleanCommand()) }

func newCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up CLI-managed resources and stale configuration entries",
		Long: `devcontainer-cli clean — unified cleanup command for Docker resources and configuration entries.

Run with no subcommand to interactively select cleanup actions or pass --all / --yes
to sweep across all categories at once.

Subcommands:
  clean catalog      Prune entries in images.json whose project directory no longer exists.
  clean containers   Remove CLI-managed containers (by name, non-running, or --all).
  clean images       Remove CLI-managed images (by reference, unused, or --all).
  clean ssh          Prune stale SSH config blocks and the host keys they pinned.
  clean networks     Remove CLI-managed Docker networks.
  clean volumes      Remove CLI-managed Docker volumes (--shared includes shared tool config).
  clean all          Sweep every category (catalog, containers, images, ssh, networks, volumes).`,
		Example: `  # Interactive clean picker
  devcontainer-cli clean

  # Sweep all categories without prompting
  devcontainer-cli clean --all --yes

  # Clean specific resource categories
  devcontainer-cli clean containers
  devcontainer-cli clean images
  devcontainer-cli clean ssh`,
		SilenceUsage: true,
		RunE:         runCleanRoot,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("dry-run", false, "Preview items that would be removed without executing")
	cmd.Flags().Bool("shared", false, "Also remove the global shared tool-config volume (devcontainer-shared-config)")

	cmd.AddCommand(newCleanCatalogCommand())
	cmd.AddCommand(newCleanContainersCommand())
	cmd.AddCommand(newCleanImagesCommand())
	cmd.AddCommand(newCleanSshCommand())
	cmd.AddCommand(newCleanNetworksCommand())
	cmd.AddCommand(newCleanVolumesCommand())
	cmd.AddCommand(newCleanAllCommand())

	return cmd
}

func addAllFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "Remove ALL managed resources, not just unused ones")
}

func allFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("all")
	return v
}

func newCleanCatalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "catalog",
		Aliases: []string{"deprecated", "registry"},
		Short:   "Prune entries in images.json whose project directory no longer exists",
		Long: `devcontainer-cli clean catalog — prune stale registry entries in ~/.config/devcontainer-cli/images.json
where the referenced project directory no longer exists on disk.`,
		SilenceUsage: true,
		RunE:         runCleanCatalog,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("dry-run", false, "List stale catalog entries without removing them")
	return cmd
}

func runCleanCatalog(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanCatalog(service.CleanOptions{
		DryRun:      dryRun,
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanContainersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "containers [name...]",
		Aliases: []string{"container", "rm"},
		Short:   "Remove CLI-managed containers (by name, non-running, or --all)",
		Long: `devcontainer-cli clean containers — remove containers created by this CLI, identified by the managed label.

Give one or more container names to remove exactly those (names tab-complete managed containers).
With no names it removes managed containers that are NOT running, or — with --all — every managed container regardless of state.`,
		Example: `  # Remove stopped managed containers
  devcontainer-cli clean containers

  # Remove specific containers by name
  devcontainer-cli clean containers myproject-postgres myproject-redis

  # Remove every managed container, no prompt
  devcontainer-cli clean containers --all --yes`,
		SilenceUsage:      true,
		RunE:              runCleanContainers,
		ValidArgsFunction: completeManagedContainerArgs,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runCleanContainers(cmd *cobra.Command, args []string) error {
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanContainers(args, service.CleanOptions{
		All:         allFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanImagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "images [ref...]",
		Aliases: []string{"image", "rmi"},
		Short:   "Remove CLI-managed images (by reference, unused, or --all)",
		Long: `devcontainer-cli clean images — remove images created by this CLI, identified by the managed label.

Give one or more image references to remove exactly those (refs tab-complete managed images).
With no refs it removes managed images that are NOT in use by any container, or — with --all — every managed image regardless.`,
		Example: `  # Remove unused managed images
  devcontainer-cli clean images

  # Remove a specific image
  devcontainer-cli clean images devcontainer-cli/ab12cd34ef56:latest

  # Remove every managed image, no prompt
  devcontainer-cli clean images --all --yes`,
		SilenceUsage:      true,
		RunE:              runCleanImages,
		ValidArgsFunction: completeManagedImageArgs,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runCleanImages(cmd *cobra.Command, args []string) error {
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanImages(args, service.CleanOptions{
		All:         allFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh",
		Aliases: []string{"sshs"},
		Short:   "Prune stale SSH config blocks and their pinned host keys",
		Long: `devcontainer-cli clean ssh — prune stale SSH entries this CLI wrote.

setup-ssh tags every Host block it adds to ~/.ssh/config with a managed marker recording what it targets
(a workspace or a specific container). This command scans those markers and removes blocks whose target is gone.

It then sweeps the CLI-managed known_hosts, dropping the host keys no remaining Host block dials — the ones
left behind by the blocks just removed, by an earlier 'destroy', or by a container that came back on a
different address. Your own ~/.ssh/known_hosts is never touched.`,
		Example: `  # Preview stale SSH blocks and orphaned host keys
  devcontainer-cli clean ssh --dry-run

  # Remove stale SSH blocks and their pinned host keys
  devcontainer-cli clean ssh`,
		SilenceUsage: true,
		RunE:         runCleanSsh,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("dry-run", false, "List stale blocks and orphaned host keys without removing them")
	return cmd
}

func runCleanSsh(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanSSH(service.CleanOptions{
		DryRun:      dryRun,
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanNetworksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "networks",
		Aliases: []string{"network"},
		Short:   "Remove CLI-managed Docker networks",
		Long: `devcontainer-cli clean networks — remove CLI-managed bridge networks that no container is attached to.
--all removes every managed network, even ones in use.`,
		SilenceUsage: true,
		RunE:         runCleanNetworks,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runCleanNetworks(cmd *cobra.Command, _ []string) error {
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanNetworks(service.CleanOptions{
		All:         allFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanVolumesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "volumes",
		Aliases: []string{"volume"},
		Short:   "Remove CLI-managed Docker volumes (--shared includes shared tool config)",
		Long: `devcontainer-cli clean volumes — remove CLI-managed volumes that no container is using.
--all removes every managed volume, and --shared also drops the global shared tool-config volume.`,
		SilenceUsage: true,
		RunE:         runCleanVolumes,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("shared", false, "Also remove global shared tool-config volume (devcontainer-shared-config)")
	return cmd
}

func runCleanVolumes(cmd *cobra.Command, _ []string) error {
	shared, _ := cmd.Flags().GetBool("shared")
	svc := service.PruneService{Report: console, Prompt: console, IncludeSharedConfig: shared}
	_, err := svc.CleanVolumes(service.CleanOptions{
		All:         allFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanAllCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Sweep every category (catalog, containers, images, ssh, networks, volumes)",
		Long:  `devcontainer-cli clean all — sweep every cleanup category in one execution.`,
		Example: `  # Sweep all categories with confirmation
  devcontainer-cli clean all

  # Sweep all categories without prompting
  devcontainer-cli clean all --yes --all`,
		SilenceUsage: true,
		RunE:         runCleanAll,
	}
	addAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("dry-run", false, "List items that would be removed without executing")
	cmd.Flags().Bool("shared", false, "Also remove global shared tool-config volume (devcontainer-shared-config)")
	return cmd
}

func runCleanAll(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	shared, _ := cmd.Flags().GetBool("shared")
	svc := service.PruneService{Report: console, Prompt: console, IncludeSharedConfig: shared}
	return svc.CleanAll(service.CleanOptions{
		DryRun:      dryRun,
		All:         allFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
}

func runCleanRoot(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	shared, _ := cmd.Flags().GetBool("shared")
	svc := service.PruneService{Report: console, Prompt: console, IncludeSharedConfig: shared}
	return svc.RunClean(service.CleanOptions{
		DryRun:      dryRun,
		All:         allFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
}
