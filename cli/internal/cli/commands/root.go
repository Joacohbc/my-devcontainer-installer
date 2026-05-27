package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/sshhelp"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the full command tree. The root command itself runs the
// default "generate" flow when invoked with no subcommand.
func NewRootCommand(v string) *cobra.Command {
	version = v
	root := &cobra.Command{
		Use:   "devcontainer-cli",
		Short: "Generate Dockerfile + docker-compose.yml for devcontainers",
		Long:  "devcontainer CLI — generate Dockerfile + docker-compose.yml and manage devcontainer environments.",
		// Run the generate flow when no subcommand is given.
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runGenerate,
	}
	addGenerateFlags(root)
	for _, c := range subcommands {
		root.AddCommand(c)
	}
	return root
}

func addGenerateFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("mode", "", "Build mode: local-cached (default), remote")
	f.String("variant", "", "Remote image variant (e.g. ssh, nodejs, python)")
	f.String("registry", "", "Container registry prefix for remote images (overrides global)")
	f.String("with", "", "Comma-separated dockerfile modules (e.g. nodejs,golang,tmux)")
	f.String("service", "", "Comma-separated compose services (e.g. mongo,postgres,tunnel)")
	f.String("services", "", "Alias for --service")
	f.String("image", "", "Image name (default: derived from fingerprint for local-cached)")
	f.String("workspace", "", "Workspace name (default: current dir name)")
	f.Bool("no-interactive", false, "Fail if any value is missing instead of prompting")
	f.Bool("non-interactive", false, "Alias for --no-interactive")
	f.Bool("force-prompt", false, "Prompt even if config file exists")
	f.Bool("force", false, "Overwrite existing files without prompting")
	f.Bool("build", false, "Run 'docker compose build/pull' after generating")
	f.Bool("no-build", false, "Skip the build/pull step after generating")
	f.BoolP("version", "v", false, "Print the CLI version")

	// Dynamic completions
	_ = cmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"local-cached", "remote"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("variant", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return types.RemoteVariants, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("with", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var moduleIDs []string
		for _, m := range catalog.DockerfileModules {
			moduleIDs = append(moduleIDs, m.ID)
		}
		return completeCSV(toComplete, moduleIDs), cobra.ShellCompDirectiveNoFileComp
	})
	completeServiceFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var serviceIDs []string
		for _, s := range catalog.ComposeServices {
			serviceIDs = append(serviceIDs, s.ID)
		}
		return completeCSV(toComplete, serviceIDs), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc("service", completeServiceFunc)
	_ = cmd.RegisterFlagCompletionFunc("services", completeServiceFunc)
}

type genFlags struct {
	interactive bool
	forcePrompt bool
	force       bool
	build       *bool // nil = ask
	image       string
	workspace   string
	withModules []string
	services    []string
	mode        string
	variant     string
	registry    string
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseGenFlags(cmd *cobra.Command) (*genFlags, error) {
	f := cmd.Flags()
	noI, _ := f.GetBool("no-interactive")
	nonI, _ := f.GetBool("non-interactive")
	g := &genFlags{
		interactive: !(noI || nonI),
	}
	g.forcePrompt, _ = f.GetBool("force-prompt")
	g.force, _ = f.GetBool("force")
	g.image, _ = f.GetString("image")
	g.workspace, _ = f.GetString("workspace")
	g.mode, _ = f.GetString("mode")
	g.variant, _ = f.GetString("variant")
	g.registry, _ = f.GetString("registry")

	if f.Changed("build") {
		t := true
		g.build = &t
	}
	if f.Changed("no-build") {
		val := false
		g.build = &val
	}

	if f.Changed("with") {
		w, _ := f.GetString("with")
		g.withModules = splitCSV(w)
	}
	if f.Changed("service") || f.Changed("services") {
		s, _ := f.GetString("service")
		if s == "" {
			s, _ = f.GetString("services")
		}
		g.services = splitCSV(s)
	}

	if g.mode != "" {
		if g.mode != string(types.BuildModeLocalCached) && g.mode != string(types.BuildModeRemote) {
			return nil, fmt.Errorf("invalid --mode: %s. Expected one of: %s", g.mode, strings.Join([]string{string(types.BuildModeLocalCached), string(types.BuildModeRemote)}, ", "))
		}
	}
	if g.variant != "" {
		if _, err := types.ParseVariant(g.variant); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func runGenerate(cmd *cobra.Command, _ []string) error {
	if v, _ := cmd.Flags().GetBool("version"); v {
		fmt.Println(version)
		return nil
	}

	flags, err := parseGenFlags(cmd)
	if err != nil {
		return err
	}

	color.New(color.FgGreen, color.Bold).Print("\nDevContainer Dockerfile Builder\n\n")

	cwd, _ := os.Getwd()
	existing, _ := domain.LoadConfig(cwd)
	var config *types.DevcontainerConfig
	if existing != nil {
		config = existing
	} else {
		config = domain.DefaultConfig(cwd)
	}

	needsPrompts := flags.forcePrompt ||
		(existing == nil && flags.interactive && len(flags.withModules) == 0 && flags.mode == "")

	if needsPrompts {
		config, err = buildConfigFromPrompts(config)
		if err != nil {
			return err
		}
	} else if flags.interactive && flags.mode == string(types.BuildModeRemote) && flags.variant == "" &&
		(config.Remote == nil || config.Remote.Variant == "") {
		picked, perr := prompt.Select("Image variant:", variantChoices(), "ssh")
		if perr != nil {
			return perr
		}
		flags.variant = picked
	}

	applyGenFlags(config, flags)

	if config.Workspace == "" {
		config.Workspace = domain.ResolveWorkspace(cwd, config)
	}
	if !domain.IsValidDockerName(config.Workspace) {
		return fmt.Errorf("invalid workspace name: %s", config.Workspace)
	}

	if config.Mode == types.BuildModeRemote && config.Image == "" && config.Remote != nil && config.Remote.Variant != "" {
		config.Image = domain.ResolveRemoteImage(config.Remote.Variant, domain.ResolveRegistry(flags.registry, config.Remote.Registry))
	}
	if config.Mode == types.BuildModeRemote && config.Remote == nil {
		return fmt.Errorf("mode=remote requires --variant (one of: %s)", strings.Join(types.RemoteVariants, ", "))
	}

	if !flags.interactive {
		if !domain.IsValidImageName(config.Image) {
			return fmt.Errorf("invalid image name: %s", config.Image)
		}
		if config.Compose.Subnet != "" && !domain.IsValidCidr(config.Compose.Subnet) {
			return fmt.Errorf("invalid CIDR: %s", config.Compose.Subnet)
		}
	}

	if config.Compose.Subnet != "" {
		used := domain.ListUsedSubnets(captureFunc())
		if clash := domain.SubnetConflict(config.Compose.Subnet, used); clash != nil {
			free := domain.FindFreeSubnet(config.Compose.Subnet, used)
			if flags.interactive {
				color.Yellow("Subnet %s overlaps with existing Docker network %s. Using %s.", config.Compose.Subnet, domain.FormatCidr(*clash), free)
				config.Compose.Subnet = free
			} else {
				return fmt.Errorf("subnet %s overlaps with existing Docker network %s. Set subnet to %s in devcontainer.config.json or remove the conflicting network", config.Compose.Subnet, domain.FormatCidr(*clash), free)
			}
		}
	}

	conflicts := domain.FindConflicts(config, docker.IsDockerAvailable(), captureFunc())
	if len(conflicts) > 0 {
		color.Yellow("\nDocker name conflicts detected:")
		for _, c := range conflicts {
			color.Yellow("   - %s '%s' already exists (project: %s)", c.Kind, c.Name, c.Owner)
		}
		if flags.interactive {
			ok, perr := prompt.Confirm("Continue anyway?", false)
			if perr != nil {
				return perr
			}
			if !ok {
				return fmt.Errorf("aborted due to name conflicts. Change workspace name or remove existing resources")
			}
		} else {
			return fmt.Errorf("docker name conflicts. Change workspace name or remove existing resources")
		}
	}

	paths := project.ProjectPaths(cwd, config.Workspace)
	buildDir := paths.BuildDir
	skipBuildArtifacts := config.Mode == types.BuildModeRemote

	if !skipBuildArtifacts {
		copyFiles, _ := domain.CollectRequiredCopyFiles(config)
		postScripts, _ := domain.CollectRequiredPostScriptFiles(config)
		referenced := append(append([]string{}, copyFiles...), postScripts...)
		if missing := assets.ValidateRequiredFiles(referenced, buildDir); len(missing) > 0 {
			return fmt.Errorf("missing required script(s) (not embedded in binary or %s): %s", buildDir, strings.Join(missing, ", "))
		}
	}

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}

	var copyFiles, postScriptFiles []string
	if !skipBuildArtifacts {
		copyFiles, _ = domain.CollectRequiredCopyFiles(config)
		if len(copyFiles) > 0 {
			pre := assets.Preflight(copyFiles, buildDir)
			if len(pre.Copied) > 0 {
				color.New(color.FgWhite).Printf("Copied build helpers → .dc_%s/build/: %s\n", config.Workspace, strings.Join(pre.Copied, ", "))
			}
			if len(pre.Missing) > 0 {
				return fmt.Errorf("missing required scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
		postScriptFiles, _ = domain.CollectRequiredPostScriptFiles(config)
		if len(postScriptFiles) > 0 {
			pre := assets.Preflight(postScriptFiles, buildDir)
			if len(pre.Copied) > 0 {
				color.New(color.FgWhite).Printf("Copied post-install scripts → .dc_%s/build/: %s\n", config.Workspace, strings.Join(pre.Copied, ", "))
			}
			if len(pre.Missing) > 0 {
				return fmt.Errorf("missing required post-install scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
	}

	dockerfileContent, err := domain.GenerateDockerfile(config)
	if err != nil {
		return err
	}

	cachedImageHit := false
	if config.Mode == types.BuildModeLocalCached && dockerfileContent != "" {
		copyContents := map[string]string{}
		for _, f := range append(append([]string{}, copyFiles...), postScriptFiles...) {
			onDisk := filepath.Join(buildDir, f)
			if data, rerr := os.ReadFile(onDisk); rerr == nil {
				copyContents[f] = string(data)
			}
		}
		var moduleIDs []string
		for _, m := range config.Dockerfile.Modules {
			moduleIDs = append(moduleIDs, m.ID)
		}
		fp := domain.ComputeFingerprint(dockerfileContent, copyContents, moduleIDs)
		config.Fingerprint = fp
		config.Image = domain.FingerprintTag(fp)
		if domain.LocalImageExists(config.Image, captureFunc()) {
			cachedImageHit = true
			color.New(color.FgWhite).Printf("Using cached image %s (fingerprint %s)\n", config.Image, fp[:12])
		}
	}

	if dockerfileContent == "" {
		color.New(color.FgWhite).Printf("Skipped Dockerfile (mode=%s).\n", config.Mode)
	} else {
		ow, oerr := maybeOverwrite(paths.DockerfilePath, "Dockerfile", flags.interactive, flags.force)
		if oerr != nil {
			return oerr
		}
		if !ow {
			color.Yellow("Skipped Dockerfile.")
		} else {
			writeOutput(paths.DockerfilePath, dockerfileContent, true)
			color.Green("Dockerfile generated.")
		}
	}

	composeContent, err := domain.GenerateCompose(config)
	if err != nil {
		return err
	}
	ow, oerr := maybeOverwrite(paths.ComposeFile, "docker-compose.yml", flags.interactive, flags.force)
	if oerr != nil {
		return oerr
	}
	if !ow {
		color.Yellow("Skipped docker-compose.yml.")
	} else {
		writeOutput(paths.ComposeFile, composeContent, true)
		color.Green("docker-compose.yml generated.")
	}

	if len(config.Env) > 0 || config.Compose.Subnet != "" {
		_, statErr := os.Stat(paths.EnvPath)
		if statErr == nil && flags.interactive {
			ok, cerr := prompt.Confirm(".env exists. Overwrite?", false)
			if cerr != nil {
				return cerr
			}
			if ok {
				os.WriteFile(paths.EnvPath, []byte(domain.GenerateEnv(config)), 0o644)
			}
		} else {
			os.WriteFile(paths.EnvPath, []byte(domain.GenerateEnv(config)), 0o644)
		}
		color.Green(".env written.")
	}

	if err := domain.SaveConfig(config, cwd); err != nil {
		return err
	}
	color.Green("Saved devcontainer.config.json")

	printLayoutMessage(config.Workspace, len(postScriptFiles) > 0)

	isRemote := config.Mode == types.BuildModeRemote
	build := flags.build
	if cachedImageHit && build == nil {
		val := false
		build = &val
		color.Green("✓ Local-cached image is up to date — skipping build.")
	}
	if build == nil && flags.interactive {
		label := "Run 'docker compose build' now?"
		if isRemote {
			label = "Run 'docker compose pull' now?"
		}
		ans, cerr := prompt.Confirm(label, true)
		if cerr != nil {
			return cerr
		}
		build = &ans
	}

	if build != nil && *build {
		banner := "\nBuilding...\n"
		action := "build"
		if isRemote {
			banner = "\nPulling image...\n"
			action = "pull"
		}
		color.Yellow(banner)
		status, _ := docker.DockerCompose(paths.ComposeFile, []string{action}, nil)
		if status == 0 {
			domain.RecordProject(cwd, config, "")
			sshhelp.Print(config.Workspace)
		}
	} else {
		domain.RecordProject(cwd, config, "")
		color.New(color.FgGreen, color.Bold).Print("\nDone.\n\n")
		sshhelp.Print(config.Workspace)
	}
	return nil
}

func applyGenFlags(config *types.DevcontainerConfig, flags *genFlags) {
	if flags.mode != "" {
		config.Mode = types.BuildMode(flags.mode)
	}
	if flags.workspace != "" {
		config.Workspace = flags.workspace
	}
	if flags.withModules != nil {
		var modules []types.SelectedModule
		for _, id := range flags.withModules {
			found := false
			for _, m := range config.Dockerfile.Modules {
				if m.ID == id {
					modules = append(modules, m)
					found = true
					break
				}
			}
			if !found {
				modules = append(modules, types.SelectedModule{ID: id, Options: map[string]any{}})
			}
		}
		config.Dockerfile.Modules = modules
	}
	if flags.services != nil {
		var services []any
		for _, id := range flags.services {
			services = append(services, types.SelectedModule{ID: id, Options: serviceOptionsOf(config.Compose.Services, id)})
		}
		config.Compose.Services = services
	}
	if flags.mode == string(types.BuildModeRemote) || config.Mode == types.BuildModeRemote {
		variant := flags.variant
		if variant == "" && config.Remote != nil {
			variant = config.Remote.Variant
		}
		if variant == "" {
			variant = "ssh"
		}
		perProject := ""
		if config.Remote != nil {
			perProject = config.Remote.Registry
		}
		if flags.registry != "" {
			perProject = flags.registry
		}
		config.Remote = &types.RemoteConfig{Variant: variant, Registry: perProject}
	}
	if flags.image != "" {
		config.Image = flags.image
	} else if config.Mode == types.BuildModeRemote && config.Remote != nil {
		config.Image = domain.ResolveRemoteImage(config.Remote.Variant, domain.ResolveRegistry(flags.registry, config.Remote.Registry))
	}
}

func writeOutput(filePath, content string, force bool) bool {
	if _, err := os.Stat(filePath); err == nil && !assets.IsGeneratedFile(filePath) && !force {
		return false
	}
	os.WriteFile(filePath, []byte(content), 0o644)
	return true
}

func maybeOverwrite(filePath, label string, interactive, force bool) (bool, error) {
	if force {
		return true, nil
	}
	if _, err := os.Stat(filePath); err != nil {
		return true, nil
	}
	if assets.IsGeneratedFile(filePath) {
		return true, nil
	}
	if !interactive {
		return false, fmt.Errorf("%s exists and is not auto-generated. Use --force or remove it", label)
	}
	return prompt.Confirm(fmt.Sprintf("%s exists and was not generated by this CLI. Overwrite?", label), false)
}

func printLayoutMessage(workspace string, hasPostScripts bool) {
	gray := color.New(color.FgWhite)
	bold := color.New(color.Bold)
	bar := gray.Sprint(strings.Repeat("─", 64))
	root := ".dc_" + workspace
	fmt.Println("\n" + bar)
	color.New(color.FgCyan, color.Bold).Printf("Generated layout under %s/\n", root)
	fmt.Println(bar)
	fmt.Printf("  %s        Dockerfile, docker-compose.yml, .env, helper .sh\n", bold.Sprint("build/"))
	fmt.Printf("               %s\n", gray.Sprintf("→ docker compose -f %s/build/docker-compose.yml up -d", root))
	if hasPostScripts {
		fmt.Printf("  %s  Baked into the image, run them inside the container\n", bold.Sprint("post-script"))
		fmt.Printf("               %s\n", gray.Sprint("→ ~/post-script/<script>.sh"))
	}
	fmt.Println(bar + "\n")
}
