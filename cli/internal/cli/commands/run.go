package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRunCommand()) }

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Spin up a container from a prebuilt remote image (no project files)",
		Long: `devcontainer-cli run — start a one-off devcontainer from a prebuilt remote
image, without generating a Dockerfile, compose file or any project files.

This is the fast path for a throwaway environment: it pulls the ghcr.io image
for the chosen variant and runs it directly with 'docker run'. Interactively it
always prompts for the container name, plus the variant, volumes, ports,
whether to mount the shared config volume and which AI installer scripts to
copy; with --no-interactive every value must come from a flag (--variant
becomes required, --name falls back to dc-<variant>).

The container name (default: dc-<variant>) is validated as you type it: if it
already belongs to a container created by a previous 'run', that container is
reused as-is (started if stopped, left alone if already running). If it
belongs to any other container — a devcontainer project container, or an
unrelated container entirely — the prompt rejects it and asks again (and
--name fails the same way non-interactively); pick a different name or remove
the existing container first.

Variants: ` + strings.Join(types.RemoteVariants, ", ") + `.`,
		Example: `  # Interactive: pick a variant and options
  devcontainer-cli run

  # Non-interactive SSH box with a named volume and a forwarded port
  devcontainer-cli run --variant ssh --name dev --volumes work:/workspace --ports 2222:22

  # Expose ports on the LAN and pre-install the AI CLI scripts
  devcontainer-cli run --variant nodejs --ports 8080:80 --expose-all --copy-ai-scripts`,
		SilenceUsage: true,
		RunE:         runQuickRun,
	}
	cmd.Flags().String("variant", "", "Image variant to run, e.g. ssh, nodejs, python (required with --no-interactive)")
	cmd.Flags().String("name", "", "Container name (default: dc-<variant>); if it belongs to an existing quick-run container it is reused/restarted, but a name already used by any other container is rejected")
	cmd.Flags().StringSlice("volumes", nil, "Volume mounts (e.g. myvol:/workspace); repeatable or comma-separated")
	cmd.Flags().StringSlice("ports", nil, "Port mappings (e.g. 2222:22); bound to 127.0.0.1 unless --expose-all; repeatable or comma-separated")
	cmd.Flags().Bool("expose-all", false, "Publish ports on all interfaces (0.0.0.0) instead of binding to 127.0.0.1")
	cmd.Flags().Bool("shared-config", true, "Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out")
	cmd.Flags().Bool("copy-ai-scripts", false, "Copy the AI/dev tool installer scripts into the container; in non-interactive mode copies all of them")
	cmd.Flags().String("registry", "", "Registry prefix override")
	addInteractiveFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("variant", staticCompletion(types.RemoteVariants...))

	return cmd
}

type runOptions struct {
	variant       string
	containerName string
	volumes       []string
	ports         []string
	exposeAll     bool
	sharedConfig  bool
	aiScripts     []string
}

func runQuickRun(cmd *cobra.Command, _ []string) error {
	svc := service.RunService{Report: console}

	opts, err := resolveRunOptions(cmd, svc)
	if err != nil {
		return err
	}

	registry, _ := cmd.Flags().GetString("registry")
	image := domain.ResolveRemoteImage(opts.variant, domain.ResolveRegistry(registry, ""))

	printRunSummary(image, opts)

	if err := svc.Run(service.QuickRunSpec{
		Variant:       opts.variant,
		ContainerName: opts.containerName,
		Image:         image,
		Volumes:       opts.volumes,
		Ports:         opts.ports,
		ExposeAll:     opts.exposeAll,
		SharedConfig:  opts.sharedConfig,
	}); err != nil {
		return err
	}

	if len(opts.aiScripts) > 0 {
		if err := svc.CopyAIScripts(opts.containerName, opts.aiScripts); err != nil {
			return err
		}
	}

	console.NewLine()
	printRunNextSteps(opts.containerName)
	return nil
}

func resolveRunOptions(cmd *cobra.Command, svc service.RunService) (runOptions, error) {
	interactive := interactiveFlag(cmd)

	opts := runOptions{}
	opts.exposeAll, _ = cmd.Flags().GetBool("expose-all")
	opts.sharedConfig, _ = cmd.Flags().GetBool("shared-config")
	rawVolumes, _ := cmd.Flags().GetStringSlice("volumes")
	rawPorts, _ := cmd.Flags().GetStringSlice("ports")
	opts.volumes = filterEmpty(rawVolumes)
	opts.ports = filterEmpty(rawPorts)

	variant, err := resolveRunVariant(cmd, interactive)
	if err != nil {
		return runOptions{}, err
	}
	opts.variant = variant

	name, _ := cmd.Flags().GetString("name")
	if interactive && !cmd.Flags().Changed("name") {
		name, err = promptContainerName(svc, variant)
		if err != nil {
			return runOptions{}, err
		}
	}
	if name == "" {
		name = "dc-" + variant
	}
	opts.containerName = name

	if interactive && !cmd.Flags().Changed("volumes") {
		opts.volumes, err = promptCommaList("Volumes to mount (e.g. myvol:/workspace,data:/data — leave empty to skip):")
		if err != nil {
			return runOptions{}, err
		}
	}
	if interactive && !cmd.Flags().Changed("ports") {
		opts.ports, err = promptCommaList("Port mappings to expose (e.g. 2222:22,8080:80 — leave empty to skip):")
		if err != nil {
			return runOptions{}, err
		}
	}

	// The shared-config flag defaults to true, so the prompt defaults to Yes; an
	// explicit --shared-config[=false] skips the prompt entirely.
	if interactive && !cmd.Flags().Changed("shared-config") {
		opts.sharedConfig, err = console.ConfirmDefault("Mount the shared config volume (persist logins/sessions across containers)?", true)
		if err != nil {
			return runOptions{}, err
		}
	}

	opts.aiScripts, err = resolveRunAIScripts(cmd, interactive)
	if err != nil {
		return runOptions{}, err
	}
	return opts, nil
}

func resolveRunVariant(cmd *cobra.Command, interactive bool) (string, error) {
	variant, _ := cmd.Flags().GetString("variant")
	if variant != "" {
		if _, err := types.ParseVariant(variant); err != nil {
			return "", err
		}
		return variant, nil
	}
	if !interactive {
		return "", fmt.Errorf("--variant required in non-interactive mode")
	}
	return promptVariant()
}

func promptVariant() (string, error) {
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
		return "", err
	}
	return picked.Value, nil
}

func promptContainerName(svc service.RunService, variant string) (string, error) {
	raw, err := console.AskDefault("Container name:", "dc-"+variant, func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("name cannot be empty")
		}
		return svc.ValidateContainerName(strings.TrimSpace(s))
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(raw), nil
}

func promptCommaList(prompt string) ([]string, error) {
	raw, err := console.AskDefault(prompt, "", nil)
	if err != nil {
		return nil, err
	}
	return filterEmpty(strings.Split(raw, ",")), nil
}

// resolveRunAIScripts decides which installer scripts to copy: interactively it
// asks (unless --copy-ai-scripts was set explicitly and lets the user pick);
// non-interactively --copy-ai-scripts copies all of them.
func resolveRunAIScripts(cmd *cobra.Command, interactive bool) ([]string, error) {
	if interactive && !cmd.Flags().Changed("copy-ai-scripts") {
		return promptAIScripts()
	}
	if copyAIScripts, _ := cmd.Flags().GetBool("copy-ai-scripts"); copyAIScripts {
		return assets.CopyableNames(), nil
	}
	return nil, nil
}

func promptAIScripts() ([]string, error) {
	want, err := console.ConfirmDefault("Copy AI tool installer scripts into the container?", false)
	if err != nil {
		return nil, err
	}
	if !want {
		return nil, nil
	}
	copyable := assets.CopyableAssets()
	choices := make([]service.Option, len(copyable))
	for i, a := range copyable {
		choices[i] = service.Option{Value: a.Name, Label: a.Label}
	}
	picked, err := console.Multiselect("Scripts to copy:", choices, choices)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range picked {
		names = append(names, p.Value)
	}
	return names, nil
}

func printRunSummary(image string, opts runOptions) {
	console.Header("\nQuick run: %s", image)
	console.NewLine()
	console.Info("  Container : %s", opts.containerName)
	if len(opts.volumes) > 0 {
		console.Info("  Volumes   : %s", strings.Join(opts.volumes, ", "))
	}
	if len(opts.ports) > 0 {
		console.Info("  Ports     : %s", strings.Join(opts.ports, ", "))
		if opts.exposeAll {
			console.Info("              (published on all interfaces / 0.0.0.0)")
		} else {
			console.Info("              (bound to 127.0.0.1; pass --expose-all for LAN access)")
		}
	}
	console.NewLine()
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
	console.Header("Next: open a shell")
	console.Info("   $ devcontainer-cli shell -c %s", containerName)
	console.NewLine()
	console.Header("Or set up SSH access")
	console.Info("   $ devcontainer-cli ssh --setup --container %s", containerName)
	console.NewLine()
	console.Header("Post-install scripts (baked into the image, run on demand):")
	console.Info("   $ devcontainer-cli shell -c %s -- ls ~/post-script", containerName)
	console.Info("   $ devcontainer-cli shell -c %s --user devuser -- bash ~/post-script/login-github-cli.sh", containerName)
	console.Bar()
	console.NewLine()
}
