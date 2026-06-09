package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// Flag names for the default generate command flow.
const (
	flagMode           = "mode"
	flagVariant        = "variant"
	flagRegistry       = "registry"
	flagWith           = "with"
	flagService        = "service"
	flagServices       = "services"
	flagImage          = "image"
	flagWorkspace      = "workspace"
	flagPersist        = "persist"
	flagPorts          = "ports"
	flagPreset         = "preset"
	flagNoInteractive  = "no-interactive"
	flagNonInteractive = "non-interactive"
	flagForcePrompt    = "force-prompt"
	flagForce          = "force"
	flagBuild          = "build"
	flagNoBuild        = "no-build"
	flagVersion        = "version"
)

func addGenerateFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String(flagMode, "", "Build mode: local-cached (default), remote")
	f.String(flagVariant, "", "Remote image variant (e.g. ssh, nodejs, python)")
	f.String(flagRegistry, "", "Container registry prefix for remote images (overrides global)")
	f.String(flagWith, "", "Comma-separated dockerfile modules (e.g. nodejs,golang,tmux)")
	f.String(flagService, "", "Comma-separated compose services (e.g. mongo,postgres,tunnel)")
	f.String(flagServices, "", "Alias for --service")
	f.String(flagImage, "", "Image name (default: derived from fingerprint for local-cached)")
	f.String(flagWorkspace, "", "Workspace name (default: current dir name)")
	f.String(flagPersist, "", "Persistence volumes mounted in the devcontainer: comma-separated ids (etc,root,home), 'all', or 'none'")
	f.String(flagPorts, "", "Ports to publish on the devcontainer (e.g. 8080:80,5432:5432); bound to 127.0.0.1 unless an IP is given; 'none' clears them")
	f.String(flagPreset, "", "Apply a preset (modules + services). See 'preset list'.")
	f.Bool(flagNoInteractive, false, "Fail if any value is missing instead of prompting")
	f.Bool(flagNonInteractive, false, "Alias for --no-interactive")
	f.Bool(flagForcePrompt, false, "Prompt even if config file exists")
	f.Bool(flagForce, false, "Overwrite existing files without prompting")
	f.Bool(flagBuild, false, "Run 'docker compose build/pull' after generating")
	f.Bool(flagNoBuild, false, "Skip the build/pull step after generating")
	f.BoolP(flagVersion, "v", false, "Print the CLI version")

	// Dynamic completions
	_ = cmd.RegisterFlagCompletionFunc(flagPreset, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ids := make([]string, 0)
		for _, p := range catalog.All(presetsDir()) {
			ids = append(ids, p.ID)
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc(flagMode, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"local-cached", "remote"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc(flagVariant, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return types.RemoteVariants, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc(flagWith, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var moduleIDs []string
		for _, m := range catalog.DockerfileModules {
			moduleIDs = append(moduleIDs, string(m.ID))
		}
		return completeCSV(toComplete, moduleIDs), cobra.ShellCompDirectiveNoFileComp
	})
	completeServiceFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var serviceIDs []string
		for _, s := range catalog.ComposeServices {
			serviceIDs = append(serviceIDs, string(s.ID))
		}
		return completeCSV(toComplete, serviceIDs), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagService, completeServiceFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagServices, completeServiceFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagPersist, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ids := append([]string{"all", "none"}, types.DefaultPersistVolumeIDs()...)
		return completeCSV(toComplete, ids), cobra.ShellCompDirectiveNoFileComp
	})
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
	persist     *[]string // nil = not set (keep existing/default)
	ports       *[]string // nil = not set (keep existing); non-nil overrides
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

// parsePersistFlag reads and validates the --persist value. "all" expands to
// every volume, "none" (or an empty value) to no volumes; otherwise it is a CSV
// of persistence volume ids.
func parsePersistFlag(get func(string) (string, error)) ([]string, error) {
	raw, _ := get(flagPersist)
	ids := splitCSV(raw)
	switch {
	case len(ids) == 0, len(ids) == 1 && ids[0] == "none":
		return []string{}, nil
	case len(ids) == 1 && ids[0] == "all":
		return types.DefaultPersistVolumeIDs(), nil
	}
	for _, id := range ids {
		if _, ok := types.PersistVolumeSpecByID(id); !ok {
			return nil, fmt.Errorf("invalid --persist volume: %s. Expected ids (%s), 'all', or 'none'", id, strings.Join(types.DefaultPersistVolumeIDs(), ", "))
		}
	}
	return ids, nil
}

// parsePortsFlag reads the --ports CSV. "none" (or an empty value) clears the
// published ports; otherwise each entry is a docker port spec stored verbatim
// (the generator binds specs without an explicit IP to 127.0.0.1).
func parsePortsFlag(get func(string) (string, error)) []string {
	raw, _ := get(flagPorts)
	specs := splitCSV(raw)
	if len(specs) == 1 && specs[0] == "none" {
		return []string{}
	}
	return specs
}

func parseGenFlags(cmd *cobra.Command) (*genFlags, error) {
	f := cmd.Flags()
	noI, _ := f.GetBool(flagNoInteractive)
	nonI, _ := f.GetBool(flagNonInteractive)
	g := &genFlags{
		interactive: !(noI || nonI),
	}
	g.forcePrompt, _ = f.GetBool(flagForcePrompt)
	g.force, _ = f.GetBool(flagForce)
	g.image, _ = f.GetString(flagImage)
	g.workspace, _ = f.GetString(flagWorkspace)
	g.mode, _ = f.GetString(flagMode)
	g.variant, _ = f.GetString(flagVariant)
	g.registry, _ = f.GetString(flagRegistry)
	g.preset, _ = f.GetString(flagPreset)

	if g.preset != "" {
		if _, ok := catalog.Resolve(g.preset, presetsDir()); !ok {
			return nil, fmt.Errorf("unknown preset: %s", g.preset)
		}
	}

	if f.Changed(flagBuild) {
		t := true
		g.build = &t
	}
	if f.Changed(flagNoBuild) {
		val := false
		g.build = &val
	}

	if f.Changed(flagWith) {
		w, _ := f.GetString(flagWith)
		g.withModules = splitCSV(w)
	}
	if f.Changed(flagService) || f.Changed(flagServices) {
		s, _ := f.GetString(flagService)
		if s == "" {
			s, _ = f.GetString(flagServices)
		}
		g.services = splitCSV(s)
	}
	if f.Changed(flagPersist) {
		ids, err := parsePersistFlag(f.GetString)
		if err != nil {
			return nil, err
		}
		g.persist = &ids
	}
	if f.Changed(flagPorts) {
		ports := parsePortsFlag(f.GetString)
		g.ports = &ports
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

// runGenerate orchestrates the default config/generation/build workflow.
func runGenerate(cmd *cobra.Command, _ []string) error {
	if v, _ := cmd.Flags().GetBool(flagVersion); v {
		console.Info("%s", version)
		return nil
	}

	flags, err := parseGenFlags(cmd)
	if err != nil {
		return err
	}

	console.Header("\nDevContainer Dockerfile Builder")
	console.NewLine()

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	svc := generateService()

	// Stage 1: Load and configure options (via prompts if required)
	config, err := initAndConfigure(cwd, flags, svc)
	if err != nil {
		return err
	}

	// Stage 2: Validate the configuration and check for name or subnet overlaps
	if err := validateConfig(cwd, config, flags, svc); err != nil {
		return err
	}

	paths := project.ProjectPaths(cwd, config.Workspace)

	// Stage 3: Prepare the workspace build directory and assets
	copyContents, postScriptFiles, err := prepareBuildDir(cwd, config, paths)
	if err != nil {
		return err
	}

	plan, err := svc.Plan(config, copyContents)
	if err != nil {
		return err
	}
	config.Fingerprint = plan.Fingerprint
	config.Image = plan.Image
	if plan.CachedImageHit {
		logger.Std().Debug("cache hit", "image", config.Image, "fp", plan.Fingerprint[:12])
	}

	// Stage 4: Write Dockerfile, docker-compose.yml, and .env files
	if err := writeGeneratedFiles(config, &plan, paths, flags); err != nil {
		return err
	}

	// Stage 5: Save project status, update .gitignore, and trigger docker compose build/pull
	return saveAndPostProcess(cwd, config, &plan, paths, flags, svc, postScriptFiles)
}

// initAndConfigure initializes config models and resolves variants and wizard flows.
func initAndConfigure(cwd string, flags *genFlags, svc service.GenerateService) (*types.DevcontainerConfig, error) {
	existing, _ := domain.LoadConfig(cwd)
	var config *types.DevcontainerConfig
	if existing != nil {
		config = existing
	} else {
		config = domain.DefaultConfig(cwd)
	}

	// Resolve preset early so its modules, services, and mode populate our flags
	if flags.preset != "" {
		p, _ := catalog.Resolve(flags.preset, presetsDir())
		if flags.withModules == nil {
			flags.withModules = p.Modules
		}
		if flags.services == nil {
			flags.services = p.Services
			if flags.services == nil {
				flags.services = []string{}
			}
		}
		if flags.mode == "" && p.Mode != "" {
			flags.mode = string(p.Mode)
		}
		// Apply preset values to config as base/defaults early so they are pre-selected if prompts are forced
		applyGenFlags(config, flags)
	}

	needsPrompts := flags.forcePrompt ||
		(existing == nil && flags.interactive && len(flags.withModules) == 0 && flags.mode == "")

	if needsPrompts {
		var err error
		config, err = svc.Configure(config, cwd, console)
		if err != nil {
			return nil, err
		}
	} else if flags.interactive && flags.mode == string(types.BuildModeRemote) && flags.variant == "" &&
		(config.Remote == nil || config.Remote.Variant == "") {
		picked, perr := console.Select("Image variant:", service.VariantChoices(), service.Option{Value: "nodejs"})
		if perr != nil {
			return nil, perr
		}
		flags.variant = picked.Value
	}

	applyGenFlags(config, flags)
	return config, nil
}

// validateConfig enforces workspace naming conventions and guards against container name or subnet conflicts.
func validateConfig(cwd string, config *types.DevcontainerConfig, flags *genFlags, svc service.GenerateService) error {
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
		used := svc.UsedSubnets()
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

	conflicts := svc.NameConflicts(config)
	if len(conflicts) > 0 {
		console.Warn("\nDocker name conflicts detected:")
		for _, c := range conflicts {
			console.Warn("   - %s '%s' already exists (project: %s)", c.Kind, c.Name, c.Owner)
		}
		if flags.interactive {
			ok, perr := console.ConfirmDefault("Continue anyway?", false)
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
	return nil
}

// prepareBuildDir constructs the build target filesystem structure and validates dependencies.
func prepareBuildDir(cwd string, config *types.DevcontainerConfig, paths project.Paths) (map[string]string, []string, error) {
	buildDir := paths.BuildDir
	skipBuildArtifacts := config.Mode == types.BuildModeRemote

	var copyFiles, postScriptFiles []string
	if !skipBuildArtifacts {
		copyFiles, _ = domain.CollectRequiredCopyFiles(config)
		postScriptFiles, _ = domain.CollectRequiredPostScriptFiles(config)
		referenced := append(append([]string{}, copyFiles...), postScriptFiles...)
		if missing := assets.ValidateRequiredFiles(referenced, buildDir); len(missing) > 0 {
			return nil, nil, fmt.Errorf("missing required script(s) (not embedded in binary or %s): %s", buildDir, strings.Join(missing, ", "))
		}
	}

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, nil, err
	}

	if !skipBuildArtifacts {
		if len(copyFiles) > 0 {
			pre := assets.Preflight(copyFiles, buildDir)
			if len(pre.Copied) > 0 {
				logger.Std().Debug("copied build helpers", "workspace", config.Workspace, "files", strings.Join(pre.Copied, ", "))
			}
			if len(pre.Missing) > 0 {
				return nil, nil, fmt.Errorf("missing required scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
		if len(postScriptFiles) > 0 {
			pre := assets.Preflight(postScriptFiles, buildDir)
			if len(pre.Copied) > 0 {
				logger.Std().Debug("copied post-install scripts", "workspace", config.Workspace, "files", strings.Join(pre.Copied, ", "))
			}
			if len(pre.Missing) > 0 {
				return nil, nil, fmt.Errorf("missing required post-install scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
	}

	copyContents := map[string]string{}
	for _, f := range append(append([]string{}, copyFiles...), postScriptFiles...) {
		if data, rerr := os.ReadFile(filepath.Join(buildDir, f)); rerr == nil {
			copyContents[f] = string(data)
		}
	}
	return copyContents, postScriptFiles, nil
}

// writeGeneratedFiles outputs generated environment configuration blueprints.
func writeGeneratedFiles(config *types.DevcontainerConfig, plan *service.GeneratePlan, paths project.Paths, flags *genFlags) error {
	if plan.Dockerfile == "" {
		console.Warn("Skipped Dockerfile (mode=%s).", config.Mode)
	} else {
		ow, oerr := maybeOverwrite(paths.DockerfilePath, "Dockerfile", flags.interactive, flags.force)
		if oerr != nil {
			return oerr
		}
		if !ow {
			console.Warn("Skipped Dockerfile.")
		} else {
			writeOutput(paths.DockerfilePath, plan.Dockerfile, true)
			console.Success("Dockerfile generated.")
		}
	}

	ow, oerr := maybeOverwrite(paths.ComposeFile, "docker-compose.yml", flags.interactive, flags.force)
	if oerr != nil {
		return oerr
	}
	if !ow {
		console.Warn("Skipped docker-compose.yml.")
	} else {
		writeOutput(paths.ComposeFile, plan.Compose, true)
		console.Success("docker-compose.yml generated.")
	}

	if len(config.Env) > 0 || config.Compose.Subnet != "" {
		_, statErr := os.Stat(paths.EnvPath)
		if statErr == nil && flags.interactive {
			ok, cerr := console.ConfirmDefault(".env exists. Overwrite?", false)
			if cerr != nil {
				return cerr
			}
			if ok {
				os.WriteFile(paths.EnvPath, []byte(plan.Env), 0o644)
			}
		} else {
			os.WriteFile(paths.EnvPath, []byte(plan.Env), 0o644)
		}
		console.Success(".env written.")
	}
	return nil
}

// saveAndPostProcess finalizes local configurations and optionally triggers local build pipelines.
func saveAndPostProcess(cwd string, config *types.DevcontainerConfig, plan *service.GeneratePlan, paths project.Paths, flags *genFlags, svc service.GenerateService, postScriptFiles []string) error {
	if err := domain.SaveConfig(config, cwd); err != nil {
		return err
	}
	console.Success("Saved devcontainer.config.json")

	if flags.interactive {
		if err := maybeUpdateGitignore(cwd, config.Workspace); err != nil {
			return err
		}
	}

	printLayoutMessage(config.Workspace, len(postScriptFiles) > 0)

	isRemote := config.Mode == types.BuildModeRemote
	build := flags.build
	if plan.CachedImageHit && build == nil {
		skip := false
		build = &skip
		console.Success("✓ Local-cached image is up to date — skipping build.")
	}
	if build == nil && flags.interactive {
		label := "Run 'docker compose build' now?"
		if isRemote {
			label = "Run 'docker compose pull' now?"
		}
		ans, cerr := console.ConfirmDefault(label, true)
		if cerr != nil {
			return cerr
		}
		build = &ans
	}

	if build != nil && *build {
		if berr := svc.Build(paths.ComposeFile, isRemote); berr != nil {
			console.Warn("%s", berr.Error())
			return nil
		}
	}
	domain.RecordProject(cwd, config, "")
	if build == nil || !*build {
		console.Done()
	}
	console.NewLine()
	console.Info("SSH access: run 'devcontainer-cli setup-ssh' to generate a key and register the host automatically.")
	return nil
}

func generateService() service.GenerateService {
	return service.GenerateService{Report: console}
}

func applyGenFlags(config *types.DevcontainerConfig, flags *genFlags) {
	if flags.preset != "" {
		p, _ := catalog.Resolve(flags.preset, presetsDir())
		if flags.withModules == nil {
			flags.withModules = p.Modules
		}
		if flags.services == nil {
			flags.services = p.Services
			if flags.services == nil {
				flags.services = []string{}
			}
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
				if string(m.ID) == id {
					modules = append(modules, m)
					found = true
					break
				}
			}
			if !found {
				modules = append(modules, types.SelectedModule{ID: types.ModuleID(id), Options: map[string]any{}})
			}
		}
		config.Dockerfile.Modules = modules
	}
	if flags.services != nil {
		var services []any
		for _, id := range flags.services {
			services = append(services, types.SelectedModule{ID: types.ModuleID(id), Options: service.ServiceOptionsOf(config.Compose.Services, id)})
		}
		config.Compose.Services = services
	}
	if flags.persist != nil {
		config.Compose.PersistVolumes = flags.persist
	}
	if flags.ports != nil {
		config.Compose.Ports = *flags.ports
	}
	if flags.mode == string(types.BuildModeRemote) || config.Mode == types.BuildModeRemote {
		variant := flags.variant
		if variant == "" && config.Remote != nil {
			variant = config.Remote.Variant
		}
		if variant == "" {
			variant = "nodejs"
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
	return console.ConfirmDefault(fmt.Sprintf("%s exists and was not generated by this CLI. Overwrite?", label), false)
}

// gitignoreEntries are the patterns added when the user accepts the gitignore prompt.
var gitignoreEntries = []string{
	".dc_*/",
	"devcontainer.config.json",
}

// maybeUpdateGitignore checks whether cwd is a git repo and, if so, offers to
// append devcontainer-cli patterns to .gitignore.
func maybeUpdateGitignore(cwd, workspace string) error {
	if _, err := os.Stat(filepath.Join(cwd, ".git")); err != nil {
		return nil // not a git repo
	}

	gitignorePath := filepath.Join(cwd, ".gitignore")

	// Read existing content (file may not exist yet).
	existing := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}

	// Collect entries that are not already present.
	var missing []string
	for _, entry := range gitignoreEntries {
		if !strings.Contains(existing, entry) {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return nil // nothing to add
	}

	_ = workspace // workspace is available if we want per-workspace entries later
	console.NewLine()
	console.Warn("Git repo detected. The following patterns are not in .gitignore:")
	for _, e := range missing {
		console.Warn("  %s", e)
	}

	ok, err := console.ConfirmDefault("Add them to .gitignore?", true)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	block := "\n# devcontainer-cli\n" + strings.Join(missing, "\n") + "\n"
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("could not open .gitignore: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		return fmt.Errorf("could not write .gitignore: %w", err)
	}
	console.Success(".gitignore updated.")
	return nil
}

func printLayoutMessage(workspace string, hasPostScripts bool) {
	root := ".dc_" + workspace
	console.NewLine()
	console.Bar()
	console.Header("Generated layout under %s/", root)
	console.Bar()
	console.Info("  %s        Dockerfile, docker-compose.yml, .env, helper .sh", console.Bold("build/"))
	console.Info("               → docker compose -f %s/build/docker-compose.yml up -d", root)
	if hasPostScripts {
		console.Info("  %s  Baked into the image, run them inside the container", console.Bold("post-script"))
		console.Info("               → ~/post-script/<script>.sh")
	}
	console.Bar()
	console.NewLine()
}
