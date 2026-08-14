package service

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

// AgentService backs the 'agent' command group: the agent-facing facade over
// the CLI. Only two of its subcommands carry logic of their own — the rest
// reuse the human commands' handlers — so this service holds exactly those:
// the catalogue dump an agent reads before generating a project, and the
// project teardown that composes destroy with the image cleanup.
type AgentService struct {
	Report Reporter
	Prompt Prompter
}

// AgentInfo is everything an agent needs to compose a valid 'agent create'
// invocation without guessing: the live catalogue plus the conventions the
// generated project follows. It is assembled from the same sources the wizard
// and the generator read, so it can never describe a module the CLI does not
// have.
type AgentInfo struct {
	Version        string             `json:"version"`
	BuildModes     []string           `json:"buildModes"`
	Modules        []AgentModuleInfo  `json:"modules"`
	Services       []AgentServiceInfo `json:"services"`
	Profiles       []AgentProfileInfo `json:"profiles"`
	Skills         []AgentSkillInfo   `json:"skills"`
	Scripts        AgentScriptInfo    `json:"scripts"`
	Assets         []AgentAssetInfo   `json:"assets"`
	RemoteVariants []string           `json:"remoteVariants"`
	Paths          AgentPathsInfo     `json:"paths"`
}

// AgentModuleInfo is one Dockerfile module as an agent sees it.
type AgentModuleInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Category string `json:"category"`
	// Selectable is false for the modules that must never appear in --with: the
	// always-on ones (applied for you) and the internal ones the generator
	// derives (the skills module, added by --skill).
	Selectable  bool                   `json:"selectable"`
	Always      bool                   `json:"always,omitempty"`
	Internal    bool                   `json:"internal,omitempty"`
	Requires    []string               `json:"requires,omitempty"`
	Conflicts   []string               `json:"conflicts,omitempty"`
	Options     []types.ModuleOption   `json:"options,omitempty"`
	RequiresEnv []types.RequiredEnvVar `json:"requiresEnv,omitempty"`
}

// AgentServiceInfo is one compose service as an agent sees it.
type AgentServiceInfo struct {
	ID             string                 `json:"id"`
	Label          string                 `json:"label"`
	Selectable     bool                   `json:"selectable"`
	Always         bool                   `json:"always,omitempty"`
	Internal       bool                   `json:"internal,omitempty"`
	IsDatabase     bool                   `json:"isDatabase,omitempty"`
	RequiresModule string                 `json:"requiresModule,omitempty"`
	RequiresEnv    []types.RequiredEnvVar `json:"requiresEnv,omitempty"`
}

// AgentProfileInfo is one profile: a module bundle, plus whatever scripts,
// skills and ports it carries.
type AgentProfileInfo struct {
	ID      string   `json:"id"`
	Label   string   `json:"label,omitempty"`
	Modules []string `json:"modules,omitempty"`
	// Remote marks the ids that also have a published image, i.e. the only ones
	// valid as a pull target under --mode profiles.
	Remote       bool     `json:"remote"`
	Scripts      []string `json:"scripts,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	SkillsMode   string   `json:"skillsMode,omitempty"`
	Ports        []string `json:"ports,omitempty"`
	ForwardPorts []string `json:"forwardPorts,omitempty"`
}

// AgentSkillInfo is one agent skill installable into the project workspace.
type AgentSkillInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Ref   string `json:"ref"`
	// InstallRef is the single token handed to the Skills CLI, which folds in
	// the skill selector when the ref holds more than one skill.
	InstallRef      string   `json:"installRef"`
	RequiresModules []string `json:"requiresModules,omitempty"`
}

// AgentScriptInfo documents how a user's own script is added to a project —
// the "how do I create a script" half of the catalogue dump.
type AgentScriptInfo struct {
	Flag        string            `json:"flag"`
	Default     string            `json:"default"`
	NamePattern string            `json:"namePattern"`
	Whens       []AgentScriptWhen `json:"whens"`
}

// AgentScriptWhen is one script lifecycle: when it runs and where it lands.
type AgentScriptWhen struct {
	When        string `json:"when"`
	Description string `json:"description"`
	Location    string `json:"location"`
}

// AgentAssetInfo is one built-in script copyable into a container.
type AgentAssetInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// AgentPathsInfo are the paths a project always follows, with <workspace>
// standing in for the project's workspace name.
type AgentPathsInfo struct {
	WorkspacePlaceholder string `json:"workspacePlaceholder"`
	WorkspaceMount       string `json:"workspaceMount"`
	WorkspaceAlias       string `json:"workspaceAlias"`
	DevUserHome          string `json:"devUserHome"`
	ProjectDir           string `json:"projectDir"`
	ConfigFile           string `json:"configFile"`
	PostScriptDir        string `json:"postScriptDir"`
	PostScriptStartDir   string `json:"postScriptStartDir"`
	DevcontainerName     string `json:"devcontainerName"`
	ContextFile          string `json:"contextFile"`
}

// workspacePlaceholder is the stand-in for the project's workspace name in the
// path conventions, which are the same shape for every project.
const workspacePlaceholder = "<workspace>"

// Info assembles the live catalogue. version is the CLI version the caller was
// built with; everything else comes from the catalogue itself plus the user's
// own profile and skill directories, so a user-defined entry shows up here
// exactly as it would in the wizard.
func (s AgentService) Info(version string) AgentInfo {
	return AgentInfo{
		Version:        version,
		BuildModes:     buildModeIDs(),
		Modules:        agentModules(),
		Services:       agentServices(),
		Profiles:       agentProfiles(),
		Skills:         agentSkills(),
		Scripts:        agentScriptInfo(),
		Assets:         agentAssets(),
		RemoteVariants: append([]string{}, types.RemoteVariants...),
		Paths:          agentPaths(),
	}
}

func buildModeIDs() []string {
	out := make([]string, len(types.BuildModes))
	for i, m := range types.BuildModes {
		out[i] = string(m)
	}
	return out
}

func agentModules() []AgentModuleInfo {
	out := make([]AgentModuleInfo, 0, len(catalog.DockerfileModules))
	for _, m := range catalog.DockerfileModules {
		out = append(out, AgentModuleInfo{
			ID:          string(m.ID),
			Label:       m.Label,
			Category:    string(m.Category),
			Selectable:  !m.Always && !m.Internal,
			Always:      m.Always,
			Internal:    m.Internal,
			Requires:    moduleIDStrings(m.Requires),
			Conflicts:   moduleIDStrings(m.Conflicts),
			Options:     m.Options,
			RequiresEnv: m.RequiresEnv,
		})
	}
	return out
}

func agentServices() []AgentServiceInfo {
	out := make([]AgentServiceInfo, 0, len(catalog.ComposeServices))
	for _, svc := range catalog.ComposeServices {
		out = append(out, AgentServiceInfo{
			ID:             string(svc.ID),
			Label:          svc.Label,
			Selectable:     !svc.Always && !svc.Internal,
			Always:         svc.Always,
			Internal:       svc.Internal,
			IsDatabase:     svc.IsDatabase,
			RequiresModule: string(svc.RequiresModule),
			RequiresEnv:    svc.RequiresEnv,
		})
	}
	return out
}

func agentProfiles() []AgentProfileInfo {
	profiles := catalog.All(domain.ProfileDirs()...)
	out := make([]AgentProfileInfo, 0, len(profiles))
	for _, p := range profiles {
		scripts := make([]string, 0, len(p.Scripts))
		for _, sc := range p.Scripts {
			scripts = append(scripts, fmt.Sprintf("%s:%s", sc.File, sc.ResolvedWhen()))
		}
		skills := make([]string, 0, len(p.Skills))
		for _, id := range p.Skills {
			skills = append(skills, string(id))
		}
		out = append(out, AgentProfileInfo{
			ID:           p.ID,
			Label:        p.Label,
			Modules:      p.Modules,
			Remote:       p.Remote,
			Scripts:      scripts,
			Skills:       skills,
			SkillsMode:   string(p.SkillsMode),
			Ports:        p.Ports,
			ForwardPorts: p.ForwardPorts,
		})
	}
	return out
}

func agentSkills() []AgentSkillInfo {
	all := catalog.AllAgentSkills(domain.SkillDirs()...)
	out := make([]AgentSkillInfo, 0, len(all))
	for _, sk := range all {
		out = append(out, AgentSkillInfo{
			ID:              string(sk.ID),
			Label:           sk.Label,
			Ref:             sk.Ref,
			InstallRef:      sk.InstallRef(),
			RequiresModules: moduleIDStrings(sk.RequiresModules),
		})
	}
	return out
}

func agentScriptInfo() AgentScriptInfo {
	descriptions := map[types.ScriptWhen]AgentScriptWhen{
		types.ScriptWhenBuild: {
			Description: "Baked into the image: run as devuser in a build layer, so its result persists in the image and a rebuild re-runs it.",
			Location:    "a RUN layer in the generated Dockerfile",
		},
		types.ScriptWhenStart: {
			Description: "Run once per container start by the entrypoint, in the background. Use it for anything that depends on the running container rather than the image.",
			Location:    types.PostScriptStartDir,
		},
		types.ScriptWhenManual: {
			Description: "Only copied into the container; nobody runs it for you.",
			Location:    types.PostScriptDir,
		},
	}
	whens := make([]AgentScriptWhen, 0, len(types.ScriptWhens))
	for _, w := range types.ScriptWhens {
		entry := descriptions[w]
		entry.When = string(w)
		whens = append(whens, entry)
	}
	return AgentScriptInfo{
		Flag:        "--script <path>[:" + strings.Join(scriptWhenNames(), "|") + "]",
		Default:     string(types.DefaultScriptWhen),
		NamePattern: domain.CustomScriptNamePattern,
		Whens:       whens,
	}
}

func scriptWhenNames() []string {
	out := make([]string, len(types.ScriptWhens))
	for i, w := range types.ScriptWhens {
		out[i] = string(w)
	}
	return out
}

func agentAssets() []AgentAssetInfo {
	copyable := assets.CopyableAssets()
	out := make([]AgentAssetInfo, 0, len(copyable))
	for _, a := range copyable {
		out = append(out, AgentAssetInfo{Name: a.Name, Label: a.Label})
	}
	return out
}

func agentPaths() AgentPathsInfo {
	return AgentPathsInfo{
		WorkspacePlaceholder: workspacePlaceholder,
		WorkspaceMount:       types.WorkspaceDir(workspacePlaceholder),
		WorkspaceAlias:       types.WorkspaceAlias(workspacePlaceholder),
		DevUserHome:          types.DevUserHome,
		ProjectDir:           ".dc_" + workspacePlaceholder + "/",
		ConfigFile:           "devcontainer.config.json",
		PostScriptDir:        types.PostScriptDir,
		PostScriptStartDir:   types.PostScriptStartDir,
		DevcontainerName:     workspacePlaceholder + "-" + sshdefaults.ServiceName,
		ContextFile:          types.DevUserHome + "/CONTEXT.md",
	}
}

func moduleIDStrings(ids []types.ModuleID) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

// AgentCleanOptions selects how far 'agent clean' reaches.
type AgentCleanOptions struct {
	// All widens the sweep past this project to every managed resource on the
	// machine — other projects' containers, images, networks and volumes
	// included.
	All    bool
	DryRun bool
	Yes    bool
	// Interactive allows the confirmation prompt. False (the agent default)
	// means a missing Yes is an error rather than a question.
	Interactive bool
}

// Clean tears down one project and removes what it left behind. The project's
// containers, network and named volumes go with the compose down -v that
// DestroyService already runs; what destroy does not touch — the image built
// for this project — is removed here, which is the whole reason this composes
// two services instead of calling one.
//
// image is the project's configured image; it is only removed when it is a
// locally built one, since a pulled ghcr.io image is shared with every other
// project on the same profile.
func (s AgentService) Clean(target DestroyTarget, image string, opts AgentCleanOptions) error {
	if !opts.Yes && !opts.Interactive {
		return fmt.Errorf("agent clean is irreversible; pass --yes to confirm in non-interactive mode")
	}

	cleanOpts := CleanOptions{
		DryRun:      opts.DryRun,
		All:         opts.All,
		Yes:         opts.Yes,
		Interactive: opts.Interactive,
	}
	prune := PruneService{Report: s.Report, Prompt: s.Prompt}
	// The image is resolved before the destroy, so a dry run and a real run
	// report the same target: destroy drops the project's catalog entry, and a
	// later lookup would have nothing left to match against.
	projectImage := s.resolvableProjectImage(prune, image)

	if opts.DryRun {
		s.Report.Info("Dry run — nothing is removed.")
		s.Report.Info("Would destroy '%s': the containers, network and volumes of its stack, plus %s and %s.",
			target.Workspace, target.ProjectDir, target.ConfigPath)
		s.Report.Info("Would remove its managed SSH host block and the host keys pinned for it.")
	} else if err := (DestroyService{Report: s.Report}).Run(target); err != nil {
		return err
	}

	if projectImage != "" {
		if _, err := prune.CleanImages([]string{projectImage}, cleanOpts); err != nil {
			return err
		}
	}

	if opts.All {
		return prune.CleanAll(cleanOpts)
	}
	return nil
}

// resolvableProjectImage returns the project's image only when removing it is
// both correct and possible: it must be one this CLI built (a pulled image is
// shared with every other project on the same profile) and still present
// locally. Anything else returns "", because CleanImages treats an unknown
// reference as an error and a project whose image was already gone must not
// fail the whole cleanup.
func (s AgentService) resolvableProjectImage(prune PruneService, image string) string {
	if image == "" || !domain.IsLocalImage(image) {
		return ""
	}
	managed, _ := prune.SelectImages(true)
	for _, img := range managed {
		if img.Ref == image {
			return image
		}
	}
	s.Report.Info("Image %s is not present locally; nothing to remove for it.", image)
	return ""
}
