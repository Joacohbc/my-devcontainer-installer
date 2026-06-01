package commands

import (
	"fmt"
	"strings"

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
	cmd.Flags().StringSlice("volumes", nil, "Volume mounts (e.g. myvol:/workspace); repeatable or comma-separated")
	cmd.Flags().StringSlice("ports", nil, "Port mappings (e.g. 2222:22); repeatable or comma-separated")
	cmd.Flags().String("registry", "", "Registry prefix override")
	addInteractiveFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("variant", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return types.RemoteVariants, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runQuickRun(cmd *cobra.Command, _ []string) error {
	variant, _ := cmd.Flags().GetString("variant")
	name, _ := cmd.Flags().GetString("name")
	volumes, _ := cmd.Flags().GetStringSlice("volumes")
	ports, _ := cmd.Flags().GetStringSlice("ports")
	registry, _ := cmd.Flags().GetString("registry")
	interactive := interactiveFlag(cmd)

	// filter empty strings that may come from default flag value
	volumes = filterEmpty(volumes)
	ports = filterEmpty(ports)

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
			if c.Value == "nodejs" {
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

	if interactive && !cmd.Flags().Changed("volumes") {
		raw, err := console.AskDefault("Volumes to mount (e.g. myvol:/workspace,data:/data — leave empty to skip):", "", nil)
		if err != nil {
			return err
		}
		volumes = filterEmpty(strings.Split(raw, ","))
	}

	if interactive && !cmd.Flags().Changed("ports") {
		raw, err := console.AskDefault("Port mappings to expose (e.g. 2222:22,8080:80 — leave empty to skip):", "", nil)
		if err != nil {
			return err
		}
		ports = filterEmpty(strings.Split(raw, ","))
	}

	reg := domain.ResolveRegistry(registry, "")
	image := domain.ResolveRemoteImage(variant, reg)
	containerName := name
	if containerName == "" {
		containerName = "dc-" + variant
	}

	console.Header("\nQuick run: %s", image)
	console.NewLine()
	console.Printf(console.Subtle("  Container : %s\n"), containerName)
	if len(volumes) > 0 {
		console.Printf(console.Subtle("  Volumes   : %s\n"), strings.Join(volumes, ", "))
	}
	if len(ports) > 0 {
		console.Printf(console.Subtle("  Ports     : %s\n"), strings.Join(ports, ", "))
	}
	console.NewLine()

	svc := service.RunService{Report: console}
	if err := svc.Run(service.QuickRunSpec{
		Variant:       variant,
		ContainerName: containerName,
		Image:         image,
		Volumes:       volumes,
		Ports:         ports,
	}); err != nil {
		return err
	}

	console.NewLine()
	printRunNextSteps(containerName)
	return nil
}

func filterEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func printRunNextSteps(containerName string) {
	console.Bar()
	console.Success("Container running.")
	console.NewLine()
	console.Println(console.Bold("Next: set up SSH access"))
	console.Printf(console.Subtle("   $ devcontainer-cli setup-ssh --container %s\n"), containerName)
	console.NewLine()
	console.Println(console.Bold("Post-install scripts (baked into the image, run on demand):"))
	console.Printf(console.Subtle("   $ docker exec -it %s ls ~/post-script\n"), containerName)
	console.Printf(console.Subtle("   $ docker exec -it -u devuser %s bash ~/post-script/login-github-cli.sh\n"), containerName)
	console.Bar()
	console.NewLine()
}
