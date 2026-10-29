package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	register(newImageCommand())
}

func newImageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage reusable, named devcontainer images",
		Long: `devcontainer-cli image — define, build, pull, export, and manage reusable images.

Images defined here are decoupled from any single project workspace and tagged under
the isolated 'devcontainer-cli-img/<name>:<tag>' namespace. Projects can consume them directly
via 'devcontainer-cli generate --mode image --image <name>' or 'devcontainer-cli agent create --mode image --image <name>',
avoiding per-project rebuilds.

Subcommands:
  list              List all registered image definitions and their local daemon status
  info <name>       Show detailed status of an image definition
  create <name>     Define a new image from a profile or remote reference
  build <name>      Build the Dockerfile for an image definition
  pull <name>       Pull a remote image and verify compatibility
  export <name> <d> Export the Dockerfile, assets, and build instructions to a directory
  remove <name>     Remove an image definition (and optionally the Docker image)`,
		Example: `  # List registered images
  devcontainer-cli image list

  # Create an image from a built-in profile
  devcontainer-cli image create data-stack --from-profile python

  # Create a definition pointing to a remote registry image
  devcontainer-cli image create team-base --remote ghcr.io/org/devcontainer:v1.0

  # Build the image locally
  devcontainer-cli image build data-stack

  # Export image Dockerfile and assets
  devcontainer-cli image export data-stack ./export-dir

  # Use the built image in a project
  devcontainer-cli agent create --mode image --image data-stack`,
		SilenceUsage: true,
	}

	cmd.AddCommand(newImageListCommand())
	cmd.AddCommand(newImageInfoCommand())
	cmd.AddCommand(newImageCreateCommand())
	cmd.AddCommand(newImageBuildCommand())
	cmd.AddCommand(newImagePullCommand())
	cmd.AddCommand(newImageExportCommand())
	cmd.AddCommand(newImageRemoveCommand())

	return cmd
}

func newImageListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "List registered devcontainer image definitions",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.ImageService{Report: ui.Console{}}
			statuses, err := svc.List()
			if err != nil {
				return err
			}
			if len(statuses) == 0 {
				fmt.Println("No custom image definitions found.")
				fmt.Println("Create one with: devcontainer-cli image create <name> --from-profile <profile>")
				return nil
			}

			fmt.Printf("%-18s %-10s %-8s %-12s %s\n", "NAME", "TAG", "SOURCE", "LOCAL STATUS", "REFERENCE")
			fmt.Println(strings.Repeat("-", 70))
			for _, st := range statuses {
				localStatus := "missing"
				if st.LocalExists {
					localStatus = "available"
					if st.Size != "" {
						localStatus += " (" + st.Size + "B)"
					}
				}
				ref := st.FullTag
				if st.Source == "remote" && st.Ref != "" {
					ref = st.Ref
				}
				fmt.Printf("%-18s %-10s %-8s %-12s %s\n", st.Name, st.Tag, st.Source, localStatus, ref)
			}
			return nil
		},
	}
}

func newImageInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "info <name>",
		Short:        "Show detailed status of an image definition",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.ImageService{Report: ui.Console{}}
			st, err := svc.Info(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Name:         %s\n", st.Name)
			fmt.Printf("Tag:          %s\n", st.Tag)
			fmt.Printf("Full Tag:     %s\n", st.FullTag)
			fmt.Printf("Source:       %s\n", st.Source)
			if st.Ref != "" {
				fmt.Printf("Remote Ref:   %s\n", st.Ref)
			}
			if st.Label != "" {
				fmt.Printf("Label:        %s\n", st.Label)
			}
			fmt.Printf("Local Status: ")
			if st.LocalExists {
				fmt.Println("Available locally")
			} else {
				fmt.Println("Not built or not pulled yet")
			}
			return nil
		},
	}
}

func newImageCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create <name>",
		Short:        "Create a new reusable image definition",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromProfile, _ := cmd.Flags().GetString("from-profile")
			remoteRef, _ := cmd.Flags().GetString("remote")
			tag, _ := cmd.Flags().GetString("tag")
			label, _ := cmd.Flags().GetString("label")

			svc := service.ImageService{Report: ui.Console{}}
			return svc.Create(args[0], service.CreateImageOpts{
				FromProfile: fromProfile,
				RemoteRef:   remoteRef,
				Tag:         tag,
				Label:       label,
			})
		},
	}
	cmd.Flags().String("from-profile", "", "Seed the image modules from an existing profile")
	cmd.Flags().String("remote", "", "Point image definition directly to a remote registry image reference")
	cmd.Flags().String("tag", "latest", "Docker tag for the image")
	cmd.Flags().String("label", "", "Human-readable description label")
	return cmd
}

func newImageBuildCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "build <name>",
		Short:        "Build the Docker image for an image definition",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.ImageService{Report: ui.Console{}}
			return svc.Build(args[0])
		},
	}
}

func newImagePullCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "pull <name>",
		Short:        "Pull a remote image and verify compatibility",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.ImageService{Report: ui.Console{}}
			return svc.Pull(args[0])
		},
	}
}

func newImageExportCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "export <name> <dir>",
		Short:        "Export image Dockerfile and assets to a directory",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.ImageService{Report: ui.Console{}}
			return svc.Export(args[0], args[1])
		},
	}
}

func newImageRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "remove <name>",
		Aliases:      []string{"rm"},
		Short:        "Remove an image definition",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keepDocker, _ := cmd.Flags().GetBool("keep-docker-image")
			svc := service.ImageService{Report: ui.Console{}}
			return svc.Remove(args[0], !keepDocker)
		},
	}
	cmd.Flags().Bool("keep-docker-image", false, "Keep the built Docker image in the daemon, removing only definition")
	return cmd
}
