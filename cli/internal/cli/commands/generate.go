package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	flagRegistry       = "registry"
	flagWith           = "with"
	flagService        = "service"
	flagServices       = "services"
	flagImage          = "image"
	flagWorkspace      = "workspace"
	flagPorts          = "ports"
	flagVolumes        = "volumes"
	flagSharedConfig   = "shared-config"
	flagProfile        = "profile"
	flagPreset         = "preset"
	flagScript         = "script"
	flagSkill          = "skill"
	flagSkillsMode     = "skills-mode"
	flagForwardPorts   = "forward-ports"
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
	f.String(flagMode, "", "Build mode: custom (default, full local build) or profiles (pull a prebuilt image)")
	f.String(flagRegistry, "", "Container registry prefix for remote images (overrides global)")
	f.String(flagWith, "", "Comma-separated dockerfile modules (e.g. nodejs,golang,rust)")
	f.String(flagService, "", "Comma-separated compose services (e.g. mongo,postgres,redis)")
	f.String(flagServices, "", "Alias for --service")
	f.String(flagImage, "", "Image name (default: derived from fingerprint for mode=custom)")
	f.String(flagWorkspace, "", "Workspace name (default: current dir name)")
	f.String(flagPorts, "", "Ports to publish on the devcontainer (e.g. 8080:80,5432:5432); bound to 127.0.0.1 unless an IP is given; 'none' clears them")
	f.String(flagVolumes, "", "Extra volume mounts on the devcontainer (e.g. myvol:/data,./cache:/cache); 'none' clears them")
	f.String(flagForwardPorts, "", "Ports 'port-forward' tunnels when called with no argument (e.g. 3000,8080:80, reverse:5432 for a host port reachable inside the container); 'none' clears them")
	f.Bool(flagSharedConfig, true, "Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out")
	f.String(flagProfile, "", "Apply a profile: a module bundle for mode=custom (any profile, e.g. 'scraper'), or the pull target for mode=profiles (must be [remote]-tagged). See 'config profile list'.")
	f.String(flagPreset, "", "Deprecated alias for --profile")
	_ = f.MarkDeprecated(flagPreset, "use --profile instead")
	f.StringArray(flagScript, nil, "Custom script to add, as <path>[:build|start|manual] (default build); repeatable. build bakes it into the image, start runs it once per container, manual only copies it to ~/post-script/")
	f.String(flagSkill, "", "Comma-separated agent skills installed into the project workspace; implies the nodejs module. See 'config skill list' (built-in + your own)")
	f.String(flagSkillsMode, "", "How the project's agent skills get installed: manual (default — you run 'install-skills') or auto (on every container start, writing into the workspace unprompted)")
	f.Bool(flagNoInteractive, false, "Fail if any value is missing instead of prompting")
	f.Bool(flagNonInteractive, false, "Alias for --no-interactive")
	f.Bool(flagForcePrompt, false, "Prompt even if config file exists")
	f.Bool(flagForce, false, "Overwrite existing files without prompting")
	f.Bool(flagBuild, false, "Run 'docker compose build/pull' after generating")
	f.Bool(flagNoBuild, false, "Skip the build/pull step after generating")
	f.BoolP(flagVersion, "v", false, "Print the CLI version")

	completeProfileFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return profileIDs(false), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagProfile, completeProfileFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagPreset, completeProfileFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagSkill, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.AgentSkillIDs(domain.SkillDirs()...)), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc(flagSkillsMode, staticCompletion(string(types.SkillModeAuto), string(types.SkillModeManual)))
	_ = cmd.RegisterFlagCompletionFunc(flagMode, staticCompletion(string(types.BuildModeCustom), string(types.BuildModeProfiles)))
	_ = cmd.RegisterFlagCompletionFunc(flagWith, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ModuleIDs()), cobra.ShellCompDirectiveNoFileComp
	})
	completeServiceFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ServiceIDs()), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagService, completeServiceFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagServices, completeServiceFunc)
}

// remoteVariantSSH is the one --profile value that is not a catalog profile:
// the hand-built full image, publishable and pullable but with no modules,
// scripts or ports of its own to resolve.
const remoteVariantSSH = "ssh"

// profileIDs lists profile ids for completion on the shared --profile flag.
// remoteOnly restricts the list to profiles with a published prebuilt image —
// its only caller for that is 'run', which always pulls, plus "ssh" (the
// hand-built full image, not a catalog profile). Everywhere else every known
// profile is offered, since --profile also selects a mode=custom module
// bundle, where a local-build-only profile like 'scraper' is valid too.
func profileIDs(remoteOnly bool) []string {
	ids := make([]string, 0)
	for _, p := range catalog.All(domain.ProfileDirs()...) {
		if remoteOnly && !p.Remote {
			continue
		}
		ids = append(ids, p.ID)
	}
	if remoteOnly {
		ids = append(ids, remoteVariantSSH)
	}
	return ids
}

// validRemoteVariant reports whether v is usable as --profile's pull target
// under mode=profiles (or 'run', which is always that target): "ssh", or a
// profile id with a published image. A profile that exists but was never
// published (e.g. 'scraper') is rejected here — it would only fail the pull —
// even though it remains valid for --profile under mode=custom.
func validRemoteVariant(v string) bool {
	if v == remoteVariantSSH {
		return true
	}
	p, ok := catalog.Resolve(v, domain.ProfileDirs()...)
	return ok && p.Remote
}

// remoteVariantError builds the mode=profiles / 'run' --profile validation
// error. A profile that exists but has no published image (e.g. 'scraper')
// gets a pointer to the local-build path instead of being reported as merely
// "unknown".
func remoteVariantError(v string) error {
	if p, ok := catalog.Resolve(v, domain.ProfileDirs()...); ok && !p.Remote {
		return fmt.Errorf("profile %q has no published remote image, so mode=profiles can't pull it; use --profile %s with mode=custom to build it locally instead", v, v)
	}
	return fmt.Errorf("unknown profile: %s. Run 'devcontainer-cli config profile list' to see available profiles", v)
}

type genFlags struct {
	interactive  bool
	forcePrompt  bool
	force        bool
	build        *bool // nil = ask
	image        string
	workspace    string
	withModules  []string
	services     []string
	ports        *[]string // nil = not set (keep existing); non-nil overrides
	volumes      *[]string // nil = not set (keep existing); non-nil overrides
	sharedConfig *bool     // nil = not set (keep existing/default-on)
	mode         string
	registry     string
	profile      string
	scripts      []types.CustomScript
	skills       []types.SkillID
	skillsMode   string
	// profileSkills and profileSkillsMode come from the applied profile and lose
	// to the explicit flags above.
	profileSkills     []types.SkillID
	profileSkillsMode types.SkillMode
	forwardPorts      *[]string // nil = not set (keep existing); non-nil overrides
	// profilePorts and profileForwardPorts come from the applied profile and lose
	// to the explicit flags above.
	profilePorts        []string
	profileForwardPorts []string
	// keepWorkspace is set when the workspace name must not be auto-uniquified: the
	// user pinned it with --workspace, or it was already persisted for this project.
	// Derived in initAndConfigure, not parsed from a flag.
	keepWorkspace bool
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

// parseClearableCSV reads a CSV flag where the literal "none" clears the list
// (returns an empty, non-nil slice). Entries are trimmed and stored verbatim.
func parseClearableCSV(get func(string) (string, error), flag string) []string {
	raw, _ := get(flag)
	items := splitCSV(raw)
	if len(items) == 1 && items[0] == "none" {
		return []string{}
	}
	return items
}

// parsePortsFlag reads the --ports CSV. "none" (or an empty value) clears the
// published ports; otherwise each entry is a docker port spec stored verbatim
// (the generator binds specs without an explicit IP to 127.0.0.1).
func parsePortsFlag(get func(string) (string, error)) []string {
	return parseClearableCSV(get, flagPorts)
}

// validateForwardPorts checks tunnel specs with the parser that will open them,
// rather than with the compose validator: their middle field is a compose
// service name ("5432:postgres:5432"), which is not a port, and they may carry
// a "reverse:" direction prefix, which compose knows nothing about.
func validateForwardPorts(specs []string) error {
	for _, spec := range specs {
		if _, err := parsePortMapping(spec, "", false); err != nil {
			return fmt.Errorf("invalid forward port %q: %w", spec, err)
		}
	}
	return nil
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
	g.registry, _ = f.GetString(flagRegistry)
	g.profile, _ = f.GetString(flagProfile)
	if g.profile == "" {
		// --preset is the deprecated spelling; it resolves to the same thing,
		// whichever role the mode ends up giving it (a module bundle for
		// mode=custom, a pull target for mode=profiles).
		g.profile, _ = f.GetString(flagPreset)
	}

	// "ssh" is not a catalog profile — it is the hand-built full image, a valid
	// pull target under mode=profiles but with no modules/scripts/ports of its
	// own to resolve here. Any other unrecognized id is still an error.
	if g.profile != "" && g.profile != remoteVariantSSH {
		p, ok := catalog.Resolve(g.profile, domain.ProfileDirs()...)
		if !ok {
			return nil, fmt.Errorf("unknown profile: %s", g.profile)
		}
		scripts, serr := domain.ProfileScripts(p)
		if serr != nil {
			return nil, serr
		}
		g.scripts = append(g.scripts, scripts...)
		if serr := domain.ValidateSkills(types.SkillsConfig{Mode: p.SkillsMode, Skills: p.Skills}); serr != nil {
			return nil, fmt.Errorf("profile %q: %w", p.ID, serr)
		}
		g.profileSkills, g.profileSkillsMode = p.Skills, p.SkillsMode
		if verr := domain.ValidatePortSpecs(p.Ports); verr != nil {
			return nil, fmt.Errorf("profile %q: %w", p.ID, verr)
		}
		if verr := validateForwardPorts(p.ForwardPorts); verr != nil {
			return nil, fmt.Errorf("profile %q: %w", p.ID, verr)
		}
		g.profilePorts, g.profileForwardPorts = p.Ports, p.ForwardPorts
	}

	if f.Changed(flagSkill) {
		raw, _ := f.GetString(flagSkill)
		for _, id := range splitCSV(raw) {
			g.skills = append(g.skills, types.SkillID(id))
		}
	}
	g.skillsMode, _ = f.GetString(flagSkillsMode)
	if err := domain.ValidateSkills(types.SkillsConfig{Mode: types.SkillMode(g.skillsMode), Skills: g.skills}); err != nil {
		return nil, err
	}

	specs, _ := f.GetStringArray(flagScript)
	for _, spec := range specs {
		s, serr := domain.ParseScriptSpec(spec)
		if serr != nil {
			return nil, serr
		}
		g.scripts = append(g.scripts, s)
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
	if f.Changed(flagPorts) {
		ports := parsePortsFlag(f.GetString)
		g.ports = &ports
	}
	if f.Changed(flagVolumes) {
		volumes := parseClearableCSV(f.GetString, flagVolumes)
		g.volumes = &volumes
	}
	if f.Changed(flagForwardPorts) {
		forward := parseClearableCSV(f.GetString, flagForwardPorts)
		if err := validateForwardPorts(forward); err != nil {
			return nil, err
		}
		g.forwardPorts = &forward
	}
	if f.Changed(flagSharedConfig) {
		v, _ := f.GetBool(flagSharedConfig)
		g.sharedConfig = &v
	}

	if g.mode != "" {
		if g.mode != string(types.BuildModeCustom) && g.mode != string(types.BuildModeProfiles) {
			return nil, fmt.Errorf("invalid --mode: %s. Expected one of: %s", g.mode, strings.Join([]string{string(types.BuildModeCustom), string(types.BuildModeProfiles)}, ", "))
		}
	}
	// --profile is validated against the [remote] tag only once the effective
	// mode is known (validateConfig): it is shared with mode=custom, where any
	// profile — including a [local]-only one like 'scraper' — is valid.
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
	// Only a brand-new project with no pinned name gets its workspace auto-uniquified;
	// an explicit --workspace or an already-persisted project keeps its name stable.
	flags.keepWorkspace = flags.workspace != "" || existing != nil

	// Resolve the profile early so its modules populate our flags. Under
	// mode=custom a profile carries no compose services, so applying one clears
	// the DB services (applyGenFlags skips that under mode=profiles, where
	// --profile instead just names the pull target).
	if flags.profile != "" {
		// Apply profile values to config as base/defaults early so they are pre-selected if prompts are forced
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
	} else if flags.interactive && flags.mode == string(types.BuildModeProfiles) && flags.profile == "" &&
		(config.Remote == nil || config.Remote.Variant == "") {
		picked, perr := console.Select("Profile:", service.ProfileChoices(domain.ProfileDirs()...), service.Option{Value: "nodejs"})
		if perr != nil {
			return nil, perr
		}
		flags.profile = picked.Value
	}

	// The full wizard already asks about skills via skillsStep; outside it,
	// still ask when nothing already decided them — no --skill, and no
	// --profile (which is its own quick shortcut and already skips the
	// build-mode/variant prompts the same way; a profile's own skills, if any,
	// stand as the answer) — so a bare 'devcontainer-cli --with nodejs' does
	// not silently skip the question the wizard would have asked. Setting
	// config.Skills directly (not flags.skills) is what lets an explicit
	// "none" answer clear an existing selection: applyGenFlags only overrides
	// config.Skills.Skills when flags.skills/profileSkills is non-empty, so it
	// leaves this alone.
	if !needsPrompts && flags.interactive && flags.profile == "" && len(flags.skills) == 0 {
		if effectiveBuildMode(config, flags) == types.BuildModeCustom && len(catalog.AllAgentSkills(domain.SkillDirs()...)) > 0 {
			selected, serr := svc.SelectSkills(config.Skills, console)
			if serr != nil {
				return nil, serr
			}
			config.Skills = selected
		}
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

	// Keep workspace names unique across projects so their (daemon-global) container,
	// network and volume names don't collide. A pinned or already-persisted name is
	// respected — we only warn — while an auto-derived name is disambiguated.
	if unique := domain.UniqueWorkspaceName(cwd, config.Workspace); unique != config.Workspace {
		if flags.keepWorkspace {
			console.Warn("Workspace name %q is already used by another project; container/network names may collide.", config.Workspace)
		} else {
			console.Info("Workspace name %q is taken by another project; using %q instead.", config.Workspace, unique)
			config.Workspace = unique
		}
	}

	// A skill whose tooling is absent still installs, and then tells an agent to
	// run something the image does not have. Report it instead of adding modules
	// the user did not ask for.
	if missing := domain.MissingSkillModules(config); len(missing) > 0 {
		ids := make([]string, 0, len(missing))
		for _, id := range missing {
			ids = append(ids, string(id))
		}
		console.Warn("The selected skills expect module(s) this project does not have: %s. Add them with --with, or the skills will describe tools that are not installed.",
			strings.Join(ids, ", "))
	}

	if config.Mode == types.BuildModeProfiles && config.Remote == nil {
		return fmt.Errorf("mode=profiles requires --profile (a [remote]-tagged profile id — run 'devcontainer-cli config profile list' to see them, or 'ssh' for the full image)")
	}
	// "ssh" names the hand-built full image rather than a catalog profile, so
	// outside mode=profiles there is no bundle behind it to build from.
	if config.Mode != types.BuildModeProfiles && flags.profile == remoteVariantSSH {
		return fmt.Errorf("profile %q is a pull target only (the hand-built full image); use --mode %s to pull it, or pick a profile that carries modules", remoteVariantSSH, types.BuildModeProfiles)
	}
	// --profile is shared with mode=custom, where any profile (including a
	// [local]-only one like 'scraper') is valid — so this can only be checked
	// once the effective mode is known, here rather than at flag-parse time.
	if config.Mode == types.BuildModeProfiles && config.Remote != nil && config.Remote.Variant != "" && !validRemoteVariant(config.Remote.Variant) {
		return remoteVariantError(config.Remote.Variant)
	}
	if config.Mode == types.BuildModeProfiles && config.Image == "" && config.Remote != nil && config.Remote.Variant != "" {
		config.Image = domain.ResolveRemoteImage(config.Remote.Variant, domain.ResolveRegistry(flags.registry, config.Remote.Registry))
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
		used := svc.UsedSubnets(domain.WorkspaceNetworkName(config.Workspace))
		resolved, err := resolveSubnet(config.Compose.Subnet, used, flags.interactive)
		if err != nil {
			return err
		}
		if resolved != config.Compose.Subnet {
			if config.Compose.Subnet == domain.DefaultSubnet {
				console.Info("Subnet %s is taken by another Docker network; using %s instead.", config.Compose.Subnet, resolved)
			} else {
				console.Warn("Subnet %s is taken by another Docker network; using %s instead.", config.Compose.Subnet, resolved)
			}
			config.Compose.Subnet = resolved
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

// materializeCustomScripts copies each configured custom script from its source
// (the profile directory it was resolved from) into the build dir under its
// namespaced name, and returns those names.
//
// A script with no Source is one persisted by an earlier run: the copy in the
// build dir is the source of truth, so it is only reported, never re-fetched.
// That is what makes a generated project reproducible after the profile it came
// from has been edited or deleted.
func materializeCustomScripts(config *types.DevcontainerConfig, buildDir string) ([]string, error) {
	names, err := domain.CollectCustomScriptFiles(config)
	if err != nil {
		return nil, err
	}
	for _, s := range config.Dockerfile.Scripts {
		if s.Source == "" {
			continue
		}
		data, rerr := domain.ReadCustomScript(s)
		if rerr != nil {
			return nil, rerr
		}
		if werr := os.WriteFile(filepath.Join(buildDir, s.BuildFile()), data, 0o755); werr != nil {
			return nil, werr
		}
	}
	return names, nil
}

// prepareBuildDir constructs the build target filesystem structure and validates dependencies.
func prepareBuildDir(cwd string, config *types.DevcontainerConfig, paths project.Paths) (map[string]string, []string, error) {
	buildDir := paths.BuildDir
	skipBuildArtifacts := config.Mode == types.BuildModeProfiles

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, nil, err
	}

	var copyFiles, postScriptFiles, customScripts []string
	if !skipBuildArtifacts {
		copyFiles, _ = domain.CollectRequiredCopyFiles(config)
		postScriptFiles, _ = domain.CollectRequiredPostScriptFiles(config)

		// The user's own scripts are copied in before anything is validated: they
		// come from a profile directory rather than the embedded FS, and once they
		// are in the build dir the existing plumbing treats them like any other
		// referenced file — ValidateRequiredFiles looks there first, Preflight
		// skips what is already present, and their contents feed the fingerprint.
		var err error
		customScripts, err = materializeCustomScripts(config, buildDir)
		if err != nil {
			return nil, nil, err
		}

		referenced := slices.Concat(copyFiles, postScriptFiles, customScripts)
		if missing := assets.ValidateRequiredFiles(referenced, buildDir); len(missing) > 0 {
			return nil, nil, fmt.Errorf("missing required script(s) (not embedded in binary or %s): %s", buildDir, strings.Join(missing, ", "))
		}
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
	for _, f := range slices.Concat(copyFiles, postScriptFiles, customScripts) {
		if data, rerr := os.ReadFile(filepath.Join(buildDir, f)); rerr == nil {
			copyContents[f] = string(data)
		}
	}

	// The shared-config catalogue is rendered from types.SharedConfigEntries
	// rather than kept as a second copy in shell. It is written here for the
	// same reason CONTEXT.md is (generated, so Preflight would not find it) and
	// folded into copyContents for the same reason too: adding an entry changes
	// what the image's entrypoint links, and two projects must not share an
	// image whose baked catalogue disagrees with the CLI that built it.
	if !skipBuildArtifacts {
		table := types.RenderSharedConfigTable()
		if werr := os.WriteFile(filepath.Join(buildDir, types.SharedConfigTableFileName), []byte(table), 0o644); werr != nil {
			return nil, nil, werr
		}
		copyContents[types.SharedConfigTableFileName] = table
	}

	// CONTEXT.md is generated rather than materialized from the embedded assets,
	// so it is written here (after the build dir exists, before the fingerprint
	// is computed) instead of going through Preflight. Folding it into
	// copyContents is what makes the fingerprint react to a change that touches
	// only the compose side — adding a database service alters CONTEXT.md but not
	// the Dockerfile, and two projects must not then share one image.
	if !skipBuildArtifacts {
		contextDoc, cerr := domain.GenerateContext(config)
		if cerr != nil {
			return nil, nil, cerr
		}
		if contextDoc != "" {
			if werr := os.WriteFile(filepath.Join(buildDir, types.ContextFileName), []byte(contextDoc), 0o644); werr != nil {
				return nil, nil, werr
			}
			copyContents[types.ContextFileName] = contextDoc
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

	// The shared tool-config volume is declared external in the compose file, so
	// create it now (best-effort) before any `docker compose up`.
	if types.SharedConfigEnabled(config) {
		if err := svc.EnsureSharedConfigVolume(); err != nil {
			console.Warn("Could not ensure shared-config volume: %v", err)
		}
	}

	if _, err := (service.GitRepoService{Report: console, Prompt: console}).EnsureRepo(cwd, flags.interactive); err != nil {
		return err
	}

	// Kept interactive-only: it asks before appending to a file the user owns,
	// and --no-interactive means no prompts. A repository created just above by
	// an explicit opt-in still gets its entries on the next interactive run.
	if flags.interactive {
		if err := maybeUpdateGitignore(cwd, config.Workspace); err != nil {
			return err
		}
	}

	printLayoutMessage(config.Workspace, len(postScriptFiles) > 0)

	isRemote := config.Mode == types.BuildModeProfiles
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
		// A failed build is a failed command. It used to be reported as a
		// warning with a zero exit, which reads as success to anything that is
		// not a human watching the scroll — a script, or 'agent create' — and
		// sends it on to use an image that was never produced. The generated
		// files and the saved config above survive either way, so returning the
		// error costs nothing and loses no work.
		if berr := svc.Build(paths.ComposeFile, isRemote); berr != nil {
			return berr
		}
	}
	domain.RecordProject(cwd, config, "")
	if build == nil || !*build {
		console.Done()
	}
	console.NewLine()
	console.Info("SSH access: run 'devcontainer-cli ssh --setup' to generate a key and register the host automatically.")
	return nil
}

func generateService() service.GenerateService {
	return service.GenerateService{Report: console}
}

// mergeCustomScripts adds the incoming scripts to the existing ones, replacing
// an entry with the same build-dir name so re-running with the same profile
// re-copies the script (picking up an edit) instead of duplicating it.
func mergeCustomScripts(existing, incoming []types.CustomScript) []types.CustomScript {
	out := append([]types.CustomScript{}, existing...)
	for _, s := range incoming {
		replaced := false
		for i, cur := range out {
			if cur.BuildFile() == s.BuildFile() {
				out[i], replaced = s, true
				break
			}
		}
		if !replaced {
			out = append(out, s)
		}
	}
	return out
}

// effectiveBuildMode is the mode this invocation ends up in: --mode when it was
// passed, and otherwise the one the project is already persisted with — so a
// regenerate that does not re-pass the flag still reads as the mode it is in.
func effectiveBuildMode(config *types.DevcontainerConfig, flags *genFlags) types.BuildMode {
	if flags.mode != "" {
		return types.BuildMode(flags.mode)
	}
	return config.Mode
}

// applyProfileBundle pre-fills the flags a mode=custom --profile stands for: its
// modules, and the empty service list every profile means (a profile carries
// none). Under mode=profiles the same flag names the pull target instead and
// must touch neither — nor may an id with nothing behind it in the catalog
// ("ssh", which validateConfig rejects under this mode), since clearing the
// services for a bundle that contributed no modules either would drop the
// project's databases for nothing.
func applyProfileBundle(config *types.DevcontainerConfig, flags *genFlags) {
	if flags.profile == "" || effectiveBuildMode(config, flags) == types.BuildModeProfiles {
		return
	}
	profile, isKnownProfile := catalog.Resolve(flags.profile, domain.ProfileDirs()...)
	if !isKnownProfile {
		return
	}
	if flags.withModules == nil {
		flags.withModules = profile.Modules
	}
	if flags.services == nil {
		flags.services = []string{}
	}
}

func applyGenFlags(config *types.DevcontainerConfig, flags *genFlags) {
	applyProfileBundle(config, flags)

	// Scripts come from the profile and from --script; they are persisted in the
	// config so a regenerated project no longer depends on either.
	if len(flags.scripts) > 0 {
		config.Dockerfile.Scripts = mergeCustomScripts(config.Dockerfile.Scripts, flags.scripts)
	}

	// An explicit --skill/--skills-mode wins over the profile's, like --with does
	// over the profile's modules.
	if len(flags.skills) > 0 {
		config.Skills.Skills = flags.skills
	} else if len(flags.profileSkills) > 0 {
		config.Skills.Skills = flags.profileSkills
	}
	if flags.skillsMode != "" {
		config.Skills.Mode = types.SkillMode(flags.skillsMode)
	} else if flags.profileSkillsMode != "" {
		config.Skills.Mode = flags.profileSkillsMode
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
		var services []types.SelectedService
		for _, id := range flags.services {
			services = append(services, types.SelectedService{ID: types.ServiceID(id), Options: service.ServiceOptionsOf(config.Compose.Services, id)})
		}
		config.Compose.Services = services
	}
	// An explicit --ports/--forward-ports wins over the profile's, like --with
	// does over its modules.
	if flags.ports != nil {
		config.Compose.Ports = *flags.ports
	} else if len(flags.profilePorts) > 0 {
		config.Compose.Ports = flags.profilePorts
	}
	if flags.forwardPorts != nil {
		config.ForwardPorts = *flags.forwardPorts
	} else if len(flags.profileForwardPorts) > 0 {
		config.ForwardPorts = flags.profileForwardPorts
	}
	if flags.volumes != nil {
		config.Compose.Volumes = *flags.volumes
	}
	if flags.sharedConfig != nil {
		config.Compose.SharedConfig = flags.sharedConfig
	}
	if flags.mode == string(types.BuildModeProfiles) || config.Mode == types.BuildModeProfiles {
		variant := flags.profile
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
	} else if config.Mode == types.BuildModeProfiles && config.Remote != nil {
		config.Image = domain.ResolveRemoteImage(config.Remote.Variant, domain.ResolveRegistry(flags.registry, config.Remote.Registry))
	}

	// Last, because --with rewrites the module list above and the skills module
	// has to survive that: it is derived from the selected skills, not chosen.
	domain.ApplySelectedSkills(config)
}

func writeOutput(filePath, content string, force bool) bool {
	if _, err := os.Stat(filePath); err == nil && !assets.IsGeneratedFile(filePath) && !force {
		return false
	}
	os.WriteFile(filePath, []byte(content), 0o644)
	return true
}

// resolveSubnet returns the subnet the project should use, moving off a taken
// one. A subnet nobody chose (the built-in default) is reassigned in both
// modes: no decision is being taken away from the user, since the CLI already
// knows the free range and picking it needs no prompt — the same way an
// auto-derived workspace name is disambiguated. A subnet the user pinned is
// theirs, so outside interactive mode a clash is reported instead of quietly
// changing what they wrote.
func resolveSubnet(subnet string, used []domain.CidrRange, interactive bool) (string, error) {
	clash := domain.SubnetConflict(subnet, used)
	if clash == nil {
		return subnet, nil
	}
	free := domain.FindFreeSubnet(subnet, used)
	if free == subnet {
		return "", fmt.Errorf("subnet %s overlaps with existing Docker network %s and no free subnet is available. Remove the conflicting network", subnet, domain.FormatCidr(*clash))
	}
	if subnet != domain.DefaultSubnet && !interactive {
		return "", fmt.Errorf("subnet %s overlaps with existing Docker network %s. Set subnet to %s in devcontainer.config.json or remove the conflicting network", subnet, domain.FormatCidr(*clash), free)
	}
	return free, nil
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
