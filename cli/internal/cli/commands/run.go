package commands

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	register(newRunCommand())
}

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Spin up a container from a prebuilt or named image (no project files)",
		Long: `devcontainer-cli run — start a one-off devcontainer from a prebuilt remote
image or one of your image definitions, without generating a Dockerfile,
compose file or any project files.

This is the fast path for a throwaway environment: it runs the image directly
with 'docker run'. The image comes from exactly one of:
  --profile <id>   the published ghcr.io image of a [remote] profile (pulled)
  --image <name>   an image definition ('image list'): a remote definition is
                   pulled, a build definition must be built first with
                   'devcontainer-cli image build <name>'

Interactively it prompts for the source (profile or image, when you have image
definitions), the container name, volumes, ports, whether to mount the shared
config volume and which AI installer scripts to copy; with --no-interactive
every value must come from a flag (--profile or --image becomes required,
--name falls back to dc-<profile> or dc-<image>).

The container name (default: dc-<profile> or dc-<image>) is validated as you
type it: if it already belongs to a container a previous 'run' started from the
same profile or image, that container is reused as-is (started if stopped, left
alone if already running). If it belongs to any other container — one started
from a different source, a devcontainer project container, or an unrelated
container entirely — the prompt rejects it and asks again (and --name fails the
same way non-interactively); pick a different name or remove the existing
container first.

--profile takes exactly one profile id with a published image — run
'devcontainer-cli profile list' and look for the [remote] tag. A [local]
profile (one of your own profiles) has no matching ghcr.io/devcontainer-<id>
image and is rejected here; build it instead with 'devcontainer-cli workspace
init --profile <id>' and the default mode=custom, which generates project
files and builds. --profile and --image are mutually exclusive, and --registry
only applies to --profile (a definition carries its own reference).`,
		Example: `  # Interactive: pick a profile and options
  devcontainer-cli run

  # Non-interactive SSH box with a named volume and a forwarded port
  devcontainer-cli run --profile base --name dev --volumes work:/workspace --ports 2222:22

  # Expose ports on the LAN
  devcontainer-cli run --profile nodejs --ports 8080:80 --expose-all

  # Run one of your image definitions (build it first if it is a local one)
  devcontainer-cli image build android
  devcontainer-cli run --image android --no-interactive`,
		SilenceUsage: true,
		RunE:         runQuickRun,
	}
	cmd.Flags().String("profile", "", "Single [remote]-tagged profile id to run as a prebuilt image, e.g. base, nodejs, python (it or --image is required with --no-interactive); see 'profile list'")
	cmd.Flags().String("name", "", "Container name (default: dc-<profile> or dc-<image>); if it belongs to a quick-run container started from the same source it is reused/restarted, but a name already used by any other container is rejected")
	cmd.Flags().StringSlice("volumes", nil, "Volume mounts (e.g. myvol:/workspace); repeatable or comma-separated")
	cmd.Flags().StringSlice("ports", nil, "Port mappings (e.g. 2222:22); bound to 127.0.0.1 unless --expose-all; repeatable or comma-separated")
	cmd.Flags().Bool("expose-all", false, "Publish ports on all interfaces (0.0.0.0) instead of binding to 127.0.0.1")
	cmd.Flags().Bool("shared-config", true, "Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out")
	cmd.Flags().String("registry", "", "Registry prefix override for --profile")
	addInteractiveFlag(cmd)
	addRunImageFlag(cmd)

	_ = cmd.RegisterFlagCompletionFunc("profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return profileIDs(true), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// addRunImageFlag adds --image to a command built on runQuickRun ('run' and
// 'agent run'), whose source resolution reads it.
func addRunImageFlag(cmd *cobra.Command) {
	cmd.Flags().String(flagImage, "", "Image definition to run instead of a profile (see 'image list'); a build definition must be built first, a remote one is pulled and verified")
	_ = cmd.RegisterFlagCompletionFunc(flagImage, completeImageNames)
}

type runOptions struct {
	// kind is where the image comes from: service.QuickRunProfile or
	// service.QuickRunImage.
	kind string
	// variant names the source: the profile id, or the image definition name.
	variant string
	// image is the resolved reference of an image definition; empty for a
	// profile, whose published image is resolved against --registry.
	image         string
	containerName string
	volumes       []string
	ports         []string
	exposeAll     bool
	sharedConfig  bool
	aiScripts     []string
}

// source is the quick-run label value of the container these options run.
func (o runOptions) source() string {
	return service.QuickRunSource(o.kind, o.variant)
}

func runQuickRun(cmd *cobra.Command, _ []string) error {
	svc := service.RunService{Report: console}

	opts, err := resolveRunOptions(cmd, svc)
	if err != nil {
		return err
	}

	image := opts.image
	if image == "" {
		registry, _ := cmd.Flags().GetString("registry")
		image = domain.ResolveRemoteImage(opts.variant, domain.ResolveRegistry(registry, ""))
	}

	printRunSummary(image, opts)

	if err := svc.Run(service.QuickRunSpec{
		Source:        opts.source(),
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

	kind, variant, image, err := resolveRunSource(cmd, interactive, console, console)
	if err != nil {
		return runOptions{}, err
	}
	opts.kind, opts.variant, opts.image = kind, variant, image

	name, _ := cmd.Flags().GetString("name")
	if interactive && !cmd.Flags().Changed("name") {
		name, err = promptContainerName(svc, variant, opts.source())
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

	opts.aiScripts, err = resolveRunAIScripts(interactive)
	if err != nil {
		return runOptions{}, err
	}
	return opts, nil
}

// resolveRunSource picks what 'run' starts from: --image (an image definition,
// returned with its runnable reference, pulled and verified when remote) or
// --profile. With neither flag it asks, offering the image choice only when
// definitions exist and --registry (a profile-only flag) is unset. It returns
// the source kind, its name and, for an image, the reference to run.
func resolveRunSource(cmd *cobra.Command, interactive bool, p service.Prompter, report service.Reporter) (string, string, string, error) {
	imageName, _ := cmd.Flags().GetString(flagImage)
	profile, _ := cmd.Flags().GetString(flagProfile)
	registry, _ := cmd.Flags().GetString("registry")
	if imageName != "" && profile != "" {
		return "", "", "", fmt.Errorf("--profile and --image are mutually exclusive: pass --profile <id> for a prebuilt profile image or --image <name> for an image definition")
	}
	if imageName != "" && registry != "" {
		return "", "", "", fmt.Errorf("--registry applies only to --profile; an image definition carries its own reference, so drop --registry")
	}
	if imageName == "" && profile == "" {
		if !interactive {
			return "", "", "", fmt.Errorf("--profile or --image required in non-interactive mode")
		}
		if registry == "" {
			picked, err := promptRunImage(p, report)
			if err != nil {
				return "", "", "", err
			}
			imageName = picked
		}
	}
	if imageName == "" {
		variant, err := resolveRunProfile(cmd, p)
		return service.QuickRunProfile, variant, "", err
	}
	ref, err := service.ImageService{Report: report}.Ready(imageName)
	if err != nil {
		return "", "", "", err
	}
	return service.QuickRunImage, imageName, ref, nil
}

// promptRunImage asks whether to run a profile or an image definition, and
// which definition, asking again while the chosen one cannot run yet; an
// empty answer means a profile. Without definitions it asks nothing.
func promptRunImage(p service.Prompter, report service.Reporter) (string, error) {
	defs, _ := domain.LoadImageDefs(domain.UserImageDir())
	if len(defs) == 0 {
		return "", nil
	}
	sources := []service.Option{
		{Value: service.QuickRunProfile, Label: "profile — a prebuilt image published for a profile"},
		{Value: service.QuickRunImage, Label: "image — one of your image definitions"},
	}
	source, err := p.Select("Source:", sources, sources[0])
	if err != nil || source.Value != service.QuickRunImage {
		return "", err
	}
	choices := service.ImageDefChoices(defs)
	for {
		picked, err := p.Select("Image:", choices, choices[0])
		if err != nil {
			return "", err
		}
		if _, err := (service.ImageService{}).Available(picked.Value); err != nil {
			report.Warn("%v", err)
			continue
		}
		return picked.Value, nil
	}
}

// resolveRunProfile reads --profile, validating it against the
// [remote]-tagged profiles the run command can actually pull; with no
// --profile (only reachable interactively) it asks for one.
func resolveRunProfile(cmd *cobra.Command, p service.Prompter) (string, error) {
	profile, _ := cmd.Flags().GetString("profile")
	if profile == "" {
		return promptVariant(p)
	}
	if len(splitCSV(profile)) > 1 {
		return "", fmt.Errorf("'run' accepts exactly one profile, got %q (run pulls a prebuilt image; multiple profiles can only be combined with mode=custom)", profile)
	}
	if !validRemoteVariant(profile) {
		return "", remoteVariantError(profile)
	}
	return profile, nil
}

func promptVariant(p service.Prompter) (string, error) {
	choices := service.ProfileChoices(domain.ProfileDirs()...)
	var initial service.Option
	for _, c := range choices {
		if c.Value == "nodejs" {
			initial = c
			break
		}
	}
	picked, err := p.Select("Profile:", choices, initial)
	if err != nil {
		return "", err
	}
	return picked.Value, nil
}

func promptContainerName(svc service.RunService, variant, source string) (string, error) {
	raw, err := console.AskDefault("Container name:", "dc-"+variant, func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("name cannot be empty")
		}
		return svc.ValidateContainerName(strings.TrimSpace(s), source)
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

func resolveRunAIScripts(interactive bool) ([]string, error) {
	if interactive {
		return promptAIScripts()
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
	console.Info("   $ devcontainer-cli access shell -c %s", containerName)
	console.NewLine()
	console.Header("Or set up SSH access")
	console.Info("   $ devcontainer-cli access ssh --setup --container %s", containerName)
	console.NewLine()
	console.Header("Post-install scripts (baked into the image, run on demand):")
	console.Info("   $ devcontainer-cli access shell -c %s -- ls ~/post-script", containerName)
	console.Info("   $ devcontainer-cli access shell -c %s --user devuser -- bash ~/post-script/login-github-cli.sh", containerName)
	console.Bar()
	console.NewLine()
}
