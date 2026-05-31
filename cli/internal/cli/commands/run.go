package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRunCommand()) }

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Spin up a remote image container without project files",
		Long: "devcontainer-cli run — spin up a container from a remote image without any project files\n\n" +
			"Variants: " + strings.Join(types.RemoteVariants, ", "),
		SilenceUsage: true,
		RunE:         runQuickRun,
	}
	cmd.Flags().String("variant", "", "Image variant (e.g. ssh, nodejs, python)")
	cmd.Flags().String("name", "", "Container name (default: dc-<variant>)")
	cmd.Flags().String("volume", "", "Named volume to mount at /workspace")
	cmd.Flags().Int("port", 0, "Expose container port 22 on host port n")
	cmd.Flags().String("registry", "", "Registry prefix override")
	addInteractiveFlag(cmd)

	// Dynamic completions
	_ = cmd.RegisterFlagCompletionFunc("variant", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return types.RemoteVariants, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runQuickRun(cmd *cobra.Command, _ []string) error {
	variant, _ := cmd.Flags().GetString("variant")
	name, _ := cmd.Flags().GetString("name")
	volume, _ := cmd.Flags().GetString("volume")
	port, _ := cmd.Flags().GetInt("port")
	registry, _ := cmd.Flags().GetString("registry")
	interactive := interactiveFlag(cmd)
	console := ui.Console{}

	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid --port value")
	}

	if variant != "" {
		if _, err := types.ParseVariant(variant); err != nil {
			return err
		}
	} else {
		if !interactive {
			return fmt.Errorf("--variant required in non-interactive mode")
		}
		choices := make([]service.Option, len(types.RemoteVariants))
		for i, v := range types.RemoteVariants {
			choices[i] = service.Option{Value: v, Label: types.VariantLabels[v]}
		}
		var initial service.Option
		for _, c := range choices {
			if c.Value == "ssh" {
				initial = c
				break
			}
		}
		picked, err := console.Select("Image variant:", choices, initial)
		if err != nil {
			return err
		}
		variant = picked.Value
	}

	reg := domain.ResolveRegistry(registry, "")
	image := domain.ResolveRemoteImage(variant, reg)
	containerName := name
	if containerName == "" {
		containerName = "dc-" + variant
	}

	console.Header("\nQuick run: %s", image)
	fmt.Println()
	fmt.Printf(console.Subtle("  Container : %s\n"), containerName)
	if volume != "" {
		fmt.Printf(console.Subtle("  Volume    : %s → /workspace\n"), volume)
	}
	if port != 0 {
		fmt.Printf(console.Subtle("  Port      : %d:22\n"), port)
	}
	println()

	svc := service.RunService{Report: console}
	if err := svc.Run(service.QuickRunSpec{
		Variant:       variant,
		ContainerName: containerName,
		Image:         image,
		Volume:        volume,
		Port:          port,
	}); err != nil {
		return err
	}

	println()
	printRunNextSteps(console, containerName)
	return nil
}

func printRunNextSteps(console ui.Console, containerName string) {
	console.Bar()
	console.Success("Container running.")
	fmt.Println()
	fmt.Println(console.Bold("Next: set up SSH access"))
	fmt.Printf(console.Subtle("   $ devcontainer-cli setup-ssh --container %s\n"), containerName)
	fmt.Println()
	fmt.Println(console.Bold("Post-install scripts (baked into the image, run on demand):"))
	fmt.Printf(console.Subtle("   $ docker exec -it %s ls ~/post-script\n"), containerName)
	fmt.Printf(console.Subtle("   $ docker exec -it -u devuser %s bash ~/post-script/login-github-cli.sh\n"), containerName)
	console.Bar()
	fmt.Println()
}
