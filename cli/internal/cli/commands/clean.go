package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up CLI-managed resources and stale configuration entries",
		Long: `devcontainer-cli config clean — unified cleanup command for Docker resources and configuration entries.

Run with no subcommand to interactively select cleanup actions or pass --include-active / --yes
to sweep across all categories at once.

Subcommands:
  clean catalog      Prune entries in images.json whose project directory no longer exists.
  clean containers   Remove CLI-managed containers (by name, non-running, or --include-active).
  clean images       Remove CLI-managed images (by reference, unused, or --include-active).
  clean ssh          Prune stale SSH config blocks and the host keys they pinned.
  clean networks     Remove CLI-managed Docker networks.
  clean volumes      Remove CLI-managed Docker volumes (--shared includes shared tool config).
  clean all          Sweep every category (catalog, containers, images, ssh, networks, volumes).`,
		Example: `  # Interactive clean picker
  devcontainer-cli config clean

  # Sweep all categories without prompting
  devcontainer-cli config clean --include-active --yes

  # Clean specific resource categories
  devcontainer-cli config clean containers
  devcontainer-cli config clean images
  devcontainer-cli config clean ssh`,
		SilenceUsage: true,
		RunE:         runCleanRoot,
	}
	addCleanAllFlag(cmd)
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

func addCleanAllFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("include-active", false, "Remove active/running resources in addition to unused ones")
}

func cleanIncludeActiveFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("include-active")
	return v
}

func newCleanCatalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Prune entries in images.json whose project directory no longer exists",
		Long: `devcontainer-cli config clean catalog — prune stale registry entries in ~/.config/devcontainer-cli/images.json
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
		Use:   "containers [name...]",
		Short: "Remove CLI-managed containers (by name, non-running, or --include-active)",
		Long: `devcontainer-cli config clean containers — remove containers created by this CLI, identified by the managed label.

Give one or more container names to remove exactly those (names tab-complete managed containers).
With no names it removes managed containers that are NOT running, or — with --include-active — every managed container regardless of state.`,
		Example: `  # Remove stopped managed containers
  devcontainer-cli config clean containers

  # Remove specific containers by name
  devcontainer-cli config clean containers myproject-postgres myproject-redis

  # Remove every managed container, no prompt
  devcontainer-cli config clean containers --include-active --yes`,
		SilenceUsage:      true,
		RunE:              runCleanContainers,
		ValidArgsFunction: completeManagedContainerArgs,
	}
	addCleanAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runCleanContainers(cmd *cobra.Command, args []string) error {
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanContainers(args, service.CleanOptions{
		All:         cleanIncludeActiveFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanImagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images [ref...]",
		Short: "Remove CLI-managed images (by reference, unused, or --include-active)",
		Long: `devcontainer-cli config clean images — remove images created by this CLI, identified by the managed label.

Give one or more image references to remove exactly those (refs tab-complete managed images).
With no refs it removes managed project images that no container and no
registered project uses, or — with --include-active — every managed project
image regardless.

Named images (reusable 'image' definitions, devcontainer-cli-img/*) are kept:
an interactive run asks separately before removing the unused ones, and --yes
alone never removes them. Name one explicitly to remove it. Untagged images left
by a rebuild are removed by ID.`,
		Example: `  # Remove unused managed images
  devcontainer-cli config clean images

  # Remove a specific image
  devcontainer-cli config clean images devcontainer-cli/nodejs-ab12cd34:latest

  # Remove every managed image, no prompt
  devcontainer-cli config clean images --include-active --yes`,
		SilenceUsage:      true,
		RunE:              runCleanImages,
		ValidArgsFunction: completeManagedImageArgs,
	}
	addCleanAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runCleanImages(cmd *cobra.Command, args []string) error {
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanImages(args, service.CleanOptions{
		All:         cleanIncludeActiveFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Prune stale SSH config blocks and their pinned host keys",
		Long: `devcontainer-cli config clean ssh — prune stale SSH entries this CLI wrote.

'ssh' tags every Host block it adds to the CLI's managed SSH config with a marker recording what it targets
(a workspace or a specific container). This command scans those markers and removes blocks whose target is gone,
plus any duplicated or orphaned marker left behind by an interrupted write — those are always removed regardless
of whether the target they name happens to still be alive, since no command can resolve a block like that anyway.
A --via block's container is checked against the remote Docker host it was set up through, not this machine's;
listings mark those with "[via <host> — remote container, not managed on this machine's Docker]" so they're never
mistaken for an ordinary local one. If that remote host can't be reached right now (its jump SSH connection was
removed or renamed, the machine is off, ...) its blocks are never auto-removed, but they are listed separately as
unverified — in interactive mode you can still select them for removal if you know they're really gone;
non-interactive (--yes) runs skip them and report how many were skipped.

--include-active additionally lists every managed SSH block regardless of status, including ones that are currently
alive and working — useful to review or manually remove a healthy entry. It only widens what is shown and
selectable in interactive mode: --yes never auto-removes an alive block just because --include-active was passed.

It then sweeps the CLI-managed known_hosts, dropping the host keys no remaining Host block dials — the ones
left behind by the blocks just removed, by an earlier 'destroy', or by a container that came back on a
different address. Your own ~/.ssh/known_hosts is never touched.`,
		Example: `  # Preview stale SSH blocks and orphaned host keys
  devcontainer-cli config clean ssh --dry-run

  # Remove stale SSH blocks and their pinned host keys
  devcontainer-cli config clean ssh

  # Review every managed SSH block, including healthy ones
  devcontainer-cli config clean ssh --include-active --dry-run`,
		SilenceUsage: true,
		RunE:         runCleanSsh,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("dry-run", false, "List stale blocks and orphaned host keys without removing them")
	cmd.Flags().Bool("include-active", false, "Also list every managed block regardless of status, including alive ones (never auto-removed by --yes)")
	return cmd
}

func runCleanSsh(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	includeActive, _ := cmd.Flags().GetBool("include-active")
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanSSH(service.CleanOptions{
		DryRun:      dryRun,
		All:         includeActive,
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanNetworksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "networks",
		Short: "Remove CLI-managed Docker networks",
		Long: `devcontainer-cli config clean networks — remove CLI-managed bridge networks that no container is attached to.
--include-active removes every managed network, even ones in use.`,
		SilenceUsage: true,
		RunE:         runCleanNetworks,
	}
	addCleanAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runCleanNetworks(cmd *cobra.Command, _ []string) error {
	svc := service.PruneService{Report: console, Prompt: console}
	_, err := svc.CleanNetworks(service.CleanOptions{
		All:         cleanIncludeActiveFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanVolumesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volumes",
		Short: "Remove CLI-managed Docker volumes (--shared includes shared tool config)",
		Long: `devcontainer-cli config clean volumes — remove CLI-managed volumes that no container is using.
--include-active removes every managed volume, and --shared also drops the global shared tool-config volume.`,
		SilenceUsage: true,
		RunE:         runCleanVolumes,
	}
	addCleanAllFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("shared", false, "Also remove global shared tool-config volume (devcontainer-shared-config)")
	return cmd
}

func runCleanVolumes(cmd *cobra.Command, _ []string) error {
	shared, _ := cmd.Flags().GetBool("shared")
	svc := service.PruneService{Report: console, Prompt: console, IncludeSharedConfig: shared}
	_, err := svc.CleanVolumes(service.CleanOptions{
		All:         cleanIncludeActiveFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
	return err
}

func newCleanAllCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Sweep every category (catalog, containers, images, ssh, networks, volumes)",
		Long:  `devcontainer-cli config clean all — sweep every cleanup category in one execution.`,
		Example: `  # Sweep all categories with confirmation
  devcontainer-cli config clean all

  # Sweep all categories without prompting
  devcontainer-cli config clean all --yes --include-active`,
		SilenceUsage: true,
		RunE:         runCleanAll,
	}
	addCleanAllFlag(cmd)
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
		All:         cleanIncludeActiveFlag(cmd),
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
		All:         cleanIncludeActiveFlag(cmd),
		Yes:         yesFlag(cmd),
		Interactive: interactiveFlag(cmd),
	})
}
