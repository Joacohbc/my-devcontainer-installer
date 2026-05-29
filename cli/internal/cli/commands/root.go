package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/sshhelp"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				logger.SetLevel(log.DebugLevel)
				return nil
			}
			raw, _ := cmd.Flags().GetString("log-level")
			lvl, err := logger.ParseLevel(raw)
			if err != nil {
				return err
			}
			logger.SetLevel(lvl)
			return nil
		},
	}
	root.PersistentFlags().Bool("verbose", false, "Enable debug logging (shortcut for --log-level debug)")
	root.PersistentFlags().String("log-level", "warn", "Log level: debug|info|warn|error")

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
	f.String("preset", "", "Apply a preset (modules + services). See 'preset list'.")
	f.Bool("no-interactive", false, "Fail if any value is missing instead of prompting")
	f.Bool("non-interactive", false, "Alias for --no-interactive")
	f.Bool("force-prompt", false, "Prompt even if config file exists")
	f.Bool("force", false, "Overwrite existing files without prompting")
	f.Bool("build", false, "Run 'docker compose build/pull' after generating")
	f.Bool("no-build", false, "Skip the build/pull step after generating")
	f.BoolP("version", "v", false, "Print the CLI version")

	// Dynamic completions
	_ = cmd.RegisterFlagCompletionFunc("preset", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ids := make([]string, 0)
		for _, p := range catalog.All(presetsDir()) {
			ids = append(ids, p.ID)
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	})
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

func presetsDir() string {
	return filepath.Join(domain.GlobalConfigDir(), "presets")
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
	preset      string
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
	g.preset, _ = f.GetString("preset")

	if g.preset != "" {
		if _, ok := catalog.Resolve(g.preset, presetsDir()); !ok {
			return nil, fmt.Errorf("unknown preset: %s", g.preset)
		}
	}

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

	ui.Header("\nDevContainer Dockerfile Builder")
	fmt.Println()

	cwd, err := currentDir()
	if err != nil {
		return err
	}
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
		config, err = runGenerateWizard(config)
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
				logger.Std().Warn("subnet overlap", "wanted", config.Compose.Subnet, "conflict", domain.FormatCidr(*clash), "using", free)
				config.Compose.Subnet = free
			} else {
				return fmt.Errorf("subnet %s overlaps with existing Docker network %s. Set subnet to %s in devcontainer.config.json or remove the conflicting network", config.Compose.Subnet, domain.FormatCidr(*clash), free)
			}
		}
	}

	conflicts := domain.FindConflicts(config, docker.IsDockerAvailable(), captureFunc())
	if len(conflicts) > 0 {
		ui.Yellow("\nDocker name conflicts detected:")
		for _, c := range conflicts {
			ui.Yellow("   - %s '%s' already exists (project: %s)", c.Kind, c.Name, c.Owner)
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

	var copyFiles, postScriptFiles []string
	if !skipBuildArtifacts {
		copyFiles, _ = domain.CollectRequiredCopyFiles(config)
		postScriptFiles, _ = domain.CollectRequiredPostScriptFiles(config)
		referenced := append(append([]string{}, copyFiles...), postScriptFiles...)
		if missing := assets.ValidateRequiredFiles(referenced, buildDir); len(missing) > 0 {
			return fmt.Errorf("missing required script(s) (not embedded in binary or %s): %s", buildDir, strings.Join(missing, ", "))
		}
	}

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}

	if !skipBuildArtifacts {
		if len(copyFiles) > 0 {
			pre := assets.Preflight(copyFiles, buildDir)
			if len(pre.Copied) > 0 {
				logger.Std().Debug("copied build helpers", "workspace", config.Workspace, "files", strings.Join(pre.Copied, ", "))
			}
			if len(pre.Missing) > 0 {
				return fmt.Errorf("missing required scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
		if len(postScriptFiles) > 0 {
			pre := assets.Preflight(postScriptFiles, buildDir)
			if len(pre.Copied) > 0 {
				logger.Std().Debug("copied post-install scripts", "workspace", config.Workspace, "files", strings.Join(pre.Copied, ", "))
			}
			if len(pre.Missing) > 0 {
				return fmt.Errorf("missing required post-install scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
	}

	copyContents := map[string]string{}
	for _, f := range append(append([]string{}, copyFiles...), postScriptFiles...) {
		if data, rerr := os.ReadFile(filepath.Join(buildDir, f)); rerr == nil {
			copyContents[f] = string(data)
		}
	}

	svc := generateService()
	plan, err := svc.Plan(config, copyContents)
	if err != nil {
		return err
	}
	config.Fingerprint = plan.Fingerprint
	config.Image = plan.Image
	if plan.CachedImageHit {
		logger.Std().Debug("cache hit", "image", config.Image, "fp", plan.Fingerprint[:12])
	}

	if plan.Dockerfile == "" {
		fmt.Printf(ui.Subtle("Skipped Dockerfile (mode=%s).\n"), config.Mode)
	} else {
		ow, oerr := maybeOverwrite(paths.DockerfilePath, "Dockerfile", flags.interactive, flags.force)
		if oerr != nil {
			return oerr
		}
		if !ow {
			ui.Yellow("Skipped Dockerfile.")
		} else {
			writeOutput(paths.DockerfilePath, plan.Dockerfile, true)
			ui.Green("Dockerfile generated.")
		}
	}

	ow, oerr := maybeOverwrite(paths.ComposeFile, "docker-compose.yml", flags.interactive, flags.force)
	if oerr != nil {
		return oerr
	}
	if !ow {
		ui.Yellow("Skipped docker-compose.yml.")
	} else {
		writeOutput(paths.ComposeFile, plan.Compose, true)
		ui.Green("docker-compose.yml generated.")
	}

	if len(config.Env) > 0 || config.Compose.Subnet != "" {
		_, statErr := os.Stat(paths.EnvPath)
		if statErr == nil && flags.interactive {
			ok, cerr := prompt.Confirm(".env exists. Overwrite?", false)
			if cerr != nil {
				return cerr
			}
			if ok {
				os.WriteFile(paths.EnvPath, []byte(plan.Env), 0o644)
			}
		} else {
			os.WriteFile(paths.EnvPath, []byte(plan.Env), 0o644)
		}
		ui.Green(".env written.")
	}

	if err := domain.SaveConfig(config, cwd); err != nil {
		return err
	}
	ui.Green("Saved devcontainer.config.json")

	printLayoutMessage(config.Workspace, len(postScriptFiles) > 0)

	isRemote := config.Mode == types.BuildModeRemote
	build := flags.build
	if plan.CachedImageHit && build == nil {
		skip := false
		build = &skip
		ui.Green("✓ Local-cached image is up to date — skipping build.")
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
		if berr := svc.Build(paths.ComposeFile, isRemote); berr != nil {
			ui.Yellow("%s", berr.Error())
			return nil
		}
	}
	domain.RecordProject(cwd, config, "")
	if build == nil || !*build {
		ui.Done()
	}
	sshhelp.Print(config.Workspace)
	return nil
}

func generateService() service.GenerateService {
	return service.GenerateService{
		Report:  consoleReporter{},
		Capture: captureFunc(),
		Compose: func(composeFile string, args []string) int {
			status, _ := docker.DockerCompose(composeFile, args, nil)
			return status
		},
	}
}

func applyGenFlags(config *types.DevcontainerConfig, flags *genFlags) {
	if flags.preset != "" {
		p, _ := catalog.Resolve(flags.preset, presetsDir())
		if flags.withModules == nil && len(p.Modules) > 0 {
			flags.withModules = p.Modules
		}
		if flags.services == nil && len(p.Services) > 0 {
			flags.services = p.Services
		}
		if flags.mode == "" && p.Mode != "" {
			flags.mode = string(p.Mode)
		}
	}

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
	root := ".dc_" + workspace
	fmt.Println()
	ui.Bar()
	ui.Header(fmt.Sprintf("Generated layout under %s/", root))
	ui.Bar()
	fmt.Printf("  %s        Dockerfile, docker-compose.yml, .env, helper .sh\n", ui.Bold("build/"))
	fmt.Printf("               %s\n", ui.Subtle(fmt.Sprintf("→ docker compose -f %s/build/docker-compose.yml up -d", root)))
	if hasPostScripts {
		fmt.Printf("  %s  Baked into the image, run them inside the container\n", ui.Bold("post-script"))
		fmt.Printf("               %s\n", ui.Subtle("→ ~/post-script/<script>.sh"))
	}
	ui.Bar()
	fmt.Println()
}
