package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newCopyCommand()) }

func newCopyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy [src_local_path] [dest_container_path]",
		Aliases: []string{"cp"},
		Short:   "Copy a file or directory from the host to the container",
		Long: `devcontainer-cli copy — copy a local file or directory into the running devcontainer

Supports real-time dynamic completion for paths inside the container.

With --asset <name>, copy a built-in script asset (e.g. an AI CLI installer)
into the devuser home of the container instead of a local path. The script is
left owned by devuser and executable. An optional destination path may be given
as the single positional argument; otherwise it lands in the devuser home.`,
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

	return svc.Copy(containerName, args[0], args[1])
}

func runCopyCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if assetFlag(cmd) != "" {
		return completeContainerPathForAsset(cmd, args, toComplete)
	}
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	if len(args) == 1 {
		wsFlag := workspaceFlag(cmd)
		containerName, err := resolveContainer(cmd, wsFlag)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeContainerPath(containerName, toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
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
