package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newCopyCommand()) }

func newCopyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy [src] [dest]",
		Aliases: []string{"cp"},
		Short:   "Copy files or directories between the host and the container",
		Long: `devcontainer-cli copy — copy a file or directory between the host and the
running devcontainer (a friendlier 'docker cp' that resolves the container for
you).

Prefix a path with ':' to mean "inside the container"; the container is resolved
automatically so you never type its name. Exactly one of src/dest may be a
container path, and the copy direction is inferred from which side carries the
':'. If neither side is prefixed, the copy defaults to host -> container. Paths
inside the container tab-complete in real time.

With --asset, instead of a local source it copies a built-in script asset (e.g.
an AI CLI installer) into the container's devuser home, left owned by devuser and
executable; the single optional positional arg then overrides the destination.`,
		Example: `  # Host -> container
  devcontainer-cli copy ./app.go :/home/devuser/app.go

  # Container -> host
  devcontainer-cli copy :/home/devuser/out.log ./out.log

  # ':' is optional when the destination is the container
  devcontainer-cli copy ./app.go /home/devuser/app.go

  # Drop a built-in installer script into the container
  devcontainer-cli copy --asset claude`,
		Args:              copyArgs,
		SilenceUsage:      true,
		ValidArgsFunction: runCopyCompletion,
		RunE:              runCopy,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
	addAssetFlag(cmd)
	return cmd
}

// addAssetFlag registers the --asset/-a flag and its completion.
func addAssetFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("asset", "a", "", "Copy a built-in script asset into the container instead of a local path")
	_ = cmd.RegisterFlagCompletionFunc("asset", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return assets.CopyableNames(), cobra.ShellCompDirectiveNoFileComp
	})
}

func assetFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("asset")
	return v
}

// copyArgs requires exactly two positional args in local-path mode, but allows
// 0 or 1 (optional dest override) when --asset is used.
func copyArgs(cmd *cobra.Command, args []string) error {
	if assetFlag(cmd) != "" {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most 1 arg (the destination path) when --asset is set, received %d", len(args))
		}
		return nil
	}
	return cobra.ExactArgs(2)(cmd, args)
}

func runCopy(cmd *cobra.Command, args []string) error {
	wsFlag := workspaceFlag(cmd)
	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return err
	}

	svc := service.InspectService{Report: ui.Console{}}

	if asset := assetFlag(cmd); asset != "" {
		dest := ""
		if len(args) == 1 {
			dest = args[0]
		}
		return svc.CopyAsset(containerName, asset, dest)
	}

	fromContainer, src, dst, err := copyDirection(args[0], args[1])
	if err != nil {
		return err
	}
	if fromContainer {
		return svc.CopyFromContainer(containerName, src, dst)
	}
	return svc.Copy(containerName, src, dst)
}

// copyDirection inspects the ':'-prefix on src/dst to decide copy direction and
// returns the paths with any prefix stripped. A ':'-prefixed path is inside the
// container; exactly one side may be a container path. When neither is prefixed
// the copy defaults to host -> container for backward compatibility.
func copyDirection(src, dst string) (fromContainer bool, cleanSrc, cleanDst string, err error) {
	srcInContainer := strings.HasPrefix(src, ":")
	dstInContainer := strings.HasPrefix(dst, ":")
	if srcInContainer && dstInContainer {
		return false, "", "", fmt.Errorf("container-to-container copy is not supported; only one path may be a container path (':'-prefixed)")
	}
	if srcInContainer {
		return true, strings.TrimPrefix(src, ":"), dst, nil
	}
	return false, src, strings.TrimPrefix(dst, ":"), nil
}

func runCopyCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if assetFlag(cmd) != "" {
		return completeContainerPathForAsset(cmd, args, toComplete)
	}
	// Either positional (src or dest) may target the container. A ':'-prefixed
	// value completes paths inside the container; otherwise fall back to local
	// host file completion.
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if !strings.HasPrefix(toComplete, ":") {
		return nil, cobra.ShellCompDirectiveDefault
	}
	wsFlag := workspaceFlag(cmd)
	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	suggestions, directive := completeContainerPath(containerName, strings.TrimPrefix(toComplete, ":"))
	for i, s := range suggestions {
		suggestions[i] = ":" + s
	}
	return suggestions, directive
}

// completeContainerPathForAsset completes the optional destination path inside
// the container when --asset is set (the source comes from the embedded asset).
func completeContainerPathForAsset(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	wsFlag := workspaceFlag(cmd)
	containerName, err := resolveContainer(cmd, wsFlag)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeContainerPath(containerName, toComplete)
}
