package commands

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
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
		choices := make([]prompt.Choice, len(types.RemoteVariants))
		for i, v := range types.RemoteVariants {
			choices[i] = prompt.Choice{Value: v, Label: types.VariantLabels[v]}
		}
		picked, err := prompt.Select("Image variant:", choices, "ssh")
		if err != nil {
			return err
		}
		variant = picked
	}

	reg := domain.ResolveRegistry(registry, "")
	image := domain.ResolveRemoteImage(variant, reg)
	containerName := name
	if containerName == "" {
		containerName = "dc-" + variant
	}

	color.New(color.FgCyan, color.Bold).Printf("\nQuick run: %s\n\n", image)
	gray := color.New(color.FgWhite)
	gray.Printf("  Container : %s\n", containerName)
	if volume != "" {
		gray.Printf("  Volume    : %s → /workspace\n", volume)
	}
	if port != 0 {
		gray.Printf("  Port      : %d:22\n", port)
	}
	println()

	_, stdout, _, _ := docker.DockerCapture([]string{"inspect", "-f", "{{.State.Status}}", containerName})
	state := strings.TrimSpace(strings.ReplaceAll(stdout, " ", ""))
	if state == "running" {
		color.Green("✓ Container '%s' is already running.", containerName)
		printRunNextSteps(containerName)
		return nil
	}
	if state == "exited" || state == "created" || state == "paused" {
		color.Yellow("↻ Starting existing container '%s'...", containerName)
		status, err := docker.DockerInherit([]string{"start", containerName})
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("docker start failed")
		}
		printRunNextSteps(containerName)
		return nil
	}

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"--label", types.LabelManaged + "=true",
		"--label", types.LabelQuickRun + "=" + variant,
	}
	if volume != "" {
		_, _ = docker.DockerInherit([]string{"volume", "create", volume})
		args = append(args, "-v", volume+":/workspace")
	}
	if port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d:22", port))
	}
	args = append(args, image, "sleep", "infinity")

	color.Yellow("Pulling and starting container...\n")
	status, err := docker.DockerInherit(args)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker run failed")
	}

	println()
	printRunNextSteps(containerName)
	return nil
}

func printRunNextSteps(containerName string) {
	gray := color.New(color.FgWhite)
	bold := color.New(color.Bold)
	bar := gray.Sprint(strings.Repeat("─", 64))
	fmt.Println(bar)
	color.New(color.FgGreen, color.Bold).Println("Container running.")
	fmt.Println()
	bold.Println("Next: set up SSH access")
	gray.Printf("   $ devcontainer-cli setup-ssh --container %s\n", containerName)
	fmt.Println()
	bold.Println("Post-install scripts (baked into the image, run on demand):")
	gray.Printf("   $ docker exec -it %s ls ~/post-script\n", containerName)
	gray.Printf("   $ docker exec -it -u devuser %s bash ~/post-script/login-github-cli.sh\n", containerName)
	fmt.Println(bar)
	fmt.Println()
}
