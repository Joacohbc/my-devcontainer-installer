package types

import (
	"encoding/json"
	"strings"
)

type ModuleID string

const (
	ModuleBase           ModuleID = "base"
	ModuleAliases        ModuleID = "aliases"
	ModuleGithubCli      ModuleID = "github-cli"
	ModuleJavaTemurin    ModuleID = "java-temurin"
	ModuleJavaOpenjdk    ModuleID = "java-openjdk"
	ModulePython         ModuleID = "python"
	ModuleSqlite         ModuleID = "sqlite"
	ModuleGolang         ModuleID = "go"
	ModulePhp            ModuleID = "php"
	ModuleRust           ModuleID = "rust"
	ModuleCCpp           ModuleID = "c-cpp"
	ModulePostgresClient ModuleID = "postgres-client"
	ModuleRedisClient    ModuleID = "redis-client"
	ModuleMongoClient    ModuleID = "mongo-client"
	ModuleNodejs         ModuleID = "nodejs"
	ModulePnpm           ModuleID = "pnpm"
	ModuleYarn           ModuleID = "yarn"
	ModuleBun            ModuleID = "bun"
	ModuleClaudeCode     ModuleID = "claude-code"
	ModuleOpencode       ModuleID = "opencode"
	ModuleCodexCli       ModuleID = "codex-cli"
	ModuleAntigravityCli ModuleID = "antigravity-cli"
	ModuleCopilotCli     ModuleID = "copilot-cli"
	ModuleGraphify       ModuleID = "graphify"
	ModuleCaveman        ModuleID = "caveman"
	ModuleZellij         ModuleID = "zellij"
	ModuleChrome         ModuleID = "chrome"
	ModuleFfmpeg         ModuleID = "ffmpeg"
	ModuleDod            ModuleID = "dod"
	ModuleNgrok          ModuleID = "ngrok"
	ModuleCloudflared    ModuleID = "cloudflared"
	ModuleSkills         ModuleID = "skills"
	ModuleCleanup        ModuleID = "cleanup"
)

type ServiceID string

const (
	ServiceDevcontainer ServiceID = "devcontainer"
	ServiceMongo        ServiceID = "mongo"
	ServiceRedis        ServiceID = "redis"
	ServicePostgres     ServiceID = "postgres"
)

type ModuleOptionType string

const (
	ModuleOptionSelect      ModuleOptionType = "select"
	ModuleOptionMultiselect ModuleOptionType = "multiselect"
	ModuleOptionInput       ModuleOptionType = "input"
	ModuleOptionConfirm     ModuleOptionType = "confirm"
)

// DevUserHome is the home directory of the non-root user created by the Base
// module inside every generated container.
const DevUserHome = "/home/devuser"
const PostScriptDir = DevUserHome + "/post-script"

// PostScriptStartDir holds the non-interactive installer scripts the entrypoint
// runs automatically (in the background, once per container) on start. Scripts
// are copied here with a numeric "NN-" prefix so the entrypoint's sorted glob
// runs them in the intended order (agents first, then the agent-wiring tools
// graphify/caveman last). Manual/interactive scripts stay under PostScriptDir.
const PostScriptStartDir = PostScriptDir + "/start.d"

// DefaultPostScriptStartOrder is the run-order prefix used for auto-start
// scripts whose module does not set an explicit PostScriptStartOrder.
const DefaultPostScriptStartOrder = 50

// WorkspaceRoot is the parent dir the project is mounted under. Each project
// gets a unique /workspaces/<workspace> path so the path-keyed history of
// Claude Code/Antigravity does not collide in the shared config volume.
const WorkspaceRoot = "/workspaces"

// WorkspaceDir returns the in-container mount path for a workspace.
func WorkspaceDir(workspace string) string {
	return WorkspaceRoot + "/" + workspace
}

// WorkspaceAliasRoot is where the entrypoint puts the short alias of the mount.
// It is a DIRECTORY holding one link per project (/workspace/<workspace> ->
// /workspaces/<workspace>), never a bare link to the project itself: agents key
// their session history by the directory they run in, and the short path is the
// one people actually cd into, so a single shared /workspace would merge every
// project's history — exactly what the per-project mount exists to prevent.
const WorkspaceAliasRoot = "/workspace"

// WorkspaceAlias returns the short per-project path for a workspace.
func WorkspaceAlias(workspace string) string {
	return WorkspaceAliasRoot + "/" + workspace
}

// FallbackDBUser and FallbackDBPassword are the database credentials used when
// the global config sets none. They live here because all three layers need the
// same values and none of them can import the others: domain resolves them from
// the config, service reports them, and the compose services render them into
// the compose file and into ~/CONTEXT.md. A second copy anywhere would let the
// document promise credentials the container was not given.
const (
	FallbackDBUser     = "devuser"
	FallbackDBPassword = "devpass"
)

// SSHKeyName is the filename of the single shared SSH key reused by every
// devcontainer (local and remote). The managed key lives under the CLI global
// config dir; sshdefaults.KeyName aliases this value.
const SSHKeyName = "id_devcontainer"

// SSHKnownHostsName is the filename of the dedicated known_hosts file the CLI
// manages for devcontainers, stored next to the managed key. Generated Host
// blocks point their UserKnownHostsFile at it so container host keys — which
// change on every image rebuild, while the container IP stays the same — never
// collide with the real hosts recorded in ~/.ssh/known_hosts.
const SSHKnownHostsName = "known_hosts"

// SSHConfigName is the filename of the SSH config file the CLI owns, kept next
// to the user's own ~/.ssh/config rather than inside it. Every managed Host
// block is written there and pulled in by a single `Include` directive at the
// top of ~/.ssh/config, so the CLI never rewrites config it did not author.
const SSHConfigName = "devcontainer-cli.config"

type ModuleOptionChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ModuleOption struct {
	ID             string               `json:"id"`
	Label          string               `json:"label"`
	Type           ModuleOptionType     `json:"type"`
	Choices        []ModuleOptionChoice `json:"choices,omitempty"`
	Default        any                  `json:"default,omitempty"`
	RequiresModule string               `json:"requiresModule,omitempty"`
}

type DockerfileCategory string

const (
	CategoryBase    DockerfileCategory = "base"
	CategoryInfra   DockerfileCategory = "infra"
	CategoryLang    DockerfileCategory = "lang"
	CategoryRuntime DockerfileCategory = "runtime"
	CategoryDB      DockerfileCategory = "db"
	CategoryCleanup DockerfileCategory = "cleanup"
)

// UICategory groups selectable modules/services for the interactive generate
// wizard. It is purely presentation-facing and independent of the internal
// DockerfileCategory used by the generator. A single source of truth lives on
// each ModuleSpec/ServiceSpec; the catalog groups by reading it.
type UICategory string

const (
	UICategoryAITools   UICategory = "ai-tools"
	UICategoryLanguages UICategory = "languages"
	UICategoryDatabases UICategory = "databases"
	UICategoryDevTools  UICategory = "dev-tools"
	UICategoryClients   UICategory = "clients"
)

// UICategoryOrder fixes the display order of categories in the wizard.
var UICategoryOrder = []UICategory{
	UICategoryAITools,
	UICategoryLanguages,
	UICategoryDatabases,
	UICategoryDevTools,
	UICategoryClients,
}

// UICategoryLabels maps each category to its user-facing (Spanish) label.
var UICategoryLabels = map[UICategory]string{
	UICategoryAITools:   "IA Tools",
	UICategoryLanguages: "Lenguajes",
	UICategoryDatabases: "Bases de datos",
	UICategoryDevTools:  "Dev Tools",
	UICategoryClients:   "Clientes de bases de datos",
}

type RequiredEnvVar struct {
	Name    string `json:"name" yaml:"name"`
	Prompt  string `json:"prompt" yaml:"prompt"`
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
}

type SelectedModule struct {
	ID      ModuleID       `json:"id" yaml:"id"`
	Options map[string]any `json:"options,omitempty" yaml:"options,omitempty"`
}

// SkillID identifies an agent skill in the catalog.
type SkillID string

const (
	SkillFirecrawl     SkillID = "firecrawl"
	SkillAgentBrowser  SkillID = "agent-browser"
	SkillWebappTesting SkillID = "webapp-testing"
	SkillDataScience   SkillID = "data-science"
	SkillRemotion      SkillID = "remotion"
	SkillN8nWorkflows  SkillID = "n8n-workflows"
)

// SkillMode says who installs the project's agent skills.
type SkillMode string

const (
	// SkillModeManual only provides the command and its alias, leaving the write
	// to the user. It is the default because the installer writes into the
	// bind-mounted workspace — the user's own repository — and doing that
	// unasked on every container start is not a decision to take for them.
	SkillModeManual SkillMode = "manual"
	// SkillModeAuto installs them on every container start, so a fresh container
	// is ready without the user doing anything.
	SkillModeAuto SkillMode = "auto"
)

var SkillModes = []SkillMode{SkillModeManual, SkillModeAuto}

const DefaultSkillMode = SkillModeManual

const (
	// SkillsEnvVar carries the space-separated skill references into the
	// container, and SkillsModeEnvVar the mode. Both are compose environment
	// entries rather than image content: changing which skills a project wants
	// must not rebuild its image.
	SkillsEnvVar     = "DEVCONTAINER_SKILLS"
	SkillsModeEnvVar = "DEVCONTAINER_SKILLS_MODE"

	// SkillsInstallCommand is the installer on PATH inside the container, and
	// SkillsInstallAlias the shell alias pointing at it.
	SkillsInstallCommand = "install-skills"
	SkillsInstallAlias   = "install_skills"

	// SkillRefSeparator splits a skill entry into its source and the name of the
	// one skill to take from it. The entries travel space-separated, so an entry
	// has to stay a single token; this keeps the selector in it without needing a
	// second argument. It appears in neither an owner/repo shorthand nor a
	// repository URL path.
	SkillRefSeparator = "#"
)

// SkillsConfig is the project's agent skills and how they get installed. They
// are project-scoped: the installer writes them into the workspace mount, so
// they belong to the project rather than to the image or the shared volume.
type SkillsConfig struct {
	Mode   SkillMode `json:"mode,omitempty" yaml:"mode,omitempty"`
	Skills []SkillID `json:"skills,omitempty" yaml:"skills,omitempty"`
}

// ResolvedMode is Mode with the default applied.
func (s SkillsConfig) ResolvedMode() SkillMode {
	if s.Mode == "" {
		return DefaultSkillMode
	}
	return s.Mode
}

func (s SkillsConfig) IsEmpty() bool { return len(s.Skills) == 0 }

// ScriptWhen says at which point a custom script runs.
type ScriptWhen string

const (
	// ScriptWhenBuild bakes the script into the image: it is COPYed into the
	// build context and executed by a RUN layer, so whatever it installs is part
	// of the image and its content feeds the fingerprint.
	ScriptWhenBuild ScriptWhen = "build"
	// ScriptWhenStart lands in PostScriptStartDir, which the entrypoint runs
	// once per container on start (state is kept in ~/.post-script-state).
	ScriptWhenStart ScriptWhen = "start"
	// ScriptWhenManual only copies the script into PostScriptDir; the user runs
	// it themselves, like the interactive login helpers.
	ScriptWhenManual ScriptWhen = "manual"
)

// ScriptWhens is every accepted ScriptWhen, in the order they are offered.
var ScriptWhens = []ScriptWhen{ScriptWhenBuild, ScriptWhenStart, ScriptWhenManual}

// DefaultScriptWhen is what an entry that omits `when` gets.
const DefaultScriptWhen = ScriptWhenBuild

// CustomScriptStartOrder runs the user's own start scripts after every
// module-provided auto-start installer, so they can build on what those set up.
const CustomScriptStartOrder = DefaultPostScriptStartOrder + 40

// CustomScriptPrefix namespaces a custom script inside the build directory, so
// a user script can never collide with an embedded asset of the same name.
const CustomScriptPrefix = "custom-"

// CustomScript is one of the user's own scripts, contributed by a profile or by
// `--script`. File is a bare file name (no directory part) resolved against the
// profile directory at generate time and materialized into the build dir; the
// generator only ever sees the copy that already lives there, which is what
// makes a regenerated project reproducible without the profile.
type CustomScript struct {
	File string     `json:"file" yaml:"file"`
	When ScriptWhen `json:"when,omitempty" yaml:"when,omitempty"`
	// Source is where the script is copied from: an absolute host path, or a
	// path inside the embedded built-in profile tree when Embedded is set. It is
	// only set while a profile is being applied — it is never persisted, because
	// the materialized copy in the build dir is the source of truth afterwards.
	Source string `json:"-" yaml:"-"`
	// Embedded marks Source as a path inside the CLI's own embedded profile
	// tree rather than on the host filesystem.
	Embedded bool `json:"-" yaml:"-"`
}

// ResolvedWhen is When with the default applied.
func (c CustomScript) ResolvedWhen() ScriptWhen {
	if c.When == "" {
		return DefaultScriptWhen
	}
	return c.When
}

// BuildFile is the script's name inside the build directory (and therefore in
// the Dockerfile's COPY). Already-prefixed names are left alone so applying a
// profile twice does not stack prefixes.
func (c CustomScript) BuildFile() string {
	if strings.HasPrefix(c.File, CustomScriptPrefix) {
		return c.File
	}
	return CustomScriptPrefix + c.File
}

// SelectedService is a chosen compose service plus its options. It accepts two
// on-disk shapes for backward compatibility: a bare id string ("mongo") or an
// object ({"id":"mongo","options":{...}}). It always marshals back to the object
// form (options omitted when empty), matching what the generator writes.
type SelectedService struct {
	ID      ServiceID      `json:"id" yaml:"id"`
	Options map[string]any `json:"options,omitempty" yaml:"options,omitempty"`
}

func (s *SelectedService) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		s.ID, s.Options = ServiceID(id), map[string]any{}
		return nil
	}
	type rawSelectedService SelectedService
	var raw rawSelectedService
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = SelectedService(raw)
	if s.Options == nil {
		s.Options = map[string]any{}
	}
	return nil
}

func (s *SelectedService) UnmarshalYAML(unmarshal func(any) error) error {
	var id string
	if err := unmarshal(&id); err == nil {
		s.ID, s.Options = ServiceID(id), map[string]any{}
		return nil
	}
	type rawSelectedService SelectedService
	var raw rawSelectedService
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*s = SelectedService(raw)
	if s.Options == nil {
		s.Options = map[string]any{}
	}
	return nil
}

type BuildMode string

const (
	BuildModeCustom   BuildMode = "custom"
	BuildModeProfiles BuildMode = "profiles"
)

var BuildModes = []BuildMode{BuildModeCustom, BuildModeProfiles}

var RemoteVariants = []string{
	"nodejs",
	"bun",
	"java-temurin",
	"python",
	"go",
	"node-go",
	"node-python",
	"node-java-temurin",
	"bun-go",
	"bun-python",
	"bun-java-temurin",
}

type RemoteConfig struct {
	Variant  string `json:"variant" yaml:"variant"`
	Registry string `json:"registry,omitempty" yaml:"registry,omitempty"`
}

type DockerfileConfig struct {
	Modules []SelectedModule `json:"modules" yaml:"modules"`
	// Scripts are the user's own scripts, resolved from a profile (or --script)
	// at generate time and persisted here so regenerating the project no longer
	// depends on the profile that contributed them — exactly like Modules.
	Scripts []CustomScript `json:"scripts,omitempty" yaml:"scripts,omitempty"`
}

type ComposeConfig struct {
	Services []SelectedService `json:"services" yaml:"services"`
	Subnet   string            `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	// Ports are the docker port mappings published on the devcontainer service
	// (e.g. "8080:80"). Specs without an explicit host IP are bound to 127.0.0.1
	// in the generated compose so they are not reachable from the LAN; include an
	// IP (e.g. "0.0.0.0:8080:80") to override. Empty/nil means no published ports.
	Ports []string `json:"ports,omitempty" yaml:"ports,omitempty"`
	// Volumes are extra volume mounts added to the devcontainer service
	// (e.g. "myvol:/data" or "./cache:/cache"). Named-volume sources (those that
	// are not host paths) are also declared in the compose top-level volumes
	// section. Bind-mount sources are resolved relative to the compose file
	// location (.dc_<workspace>/build/). Empty/nil means no extra mounts.
	Volumes []string `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	// SharedConfig toggles mounting the global shared tool-config volume
	// (devcontainer-shared-config) and the entrypoint symlinks into devuser's
	// home. A nil value means "unset" and is treated as enabled (the default,
	// including legacy configs); set it to false to opt out.
	SharedConfig *bool `json:"sharedConfig,omitempty" yaml:"sharedConfig,omitempty"`
}

type DevcontainerConfig struct {
	Mode       BuildMode         `json:"mode" yaml:"mode"`
	Image      string            `json:"image" yaml:"image"`
	Workspace  string            `json:"workspace" yaml:"workspace"`
	Dockerfile DockerfileConfig  `json:"dockerfile" yaml:"dockerfile"`
	Compose    ComposeConfig     `json:"compose" yaml:"compose"`
	Env        map[string]string `json:"env" yaml:"env"`
	Skills     SkillsConfig      `json:"skills,omitempty" yaml:"skills,omitempty"`
	// ForwardPorts are the SSH tunnels `port-forward` opens when called with no
	// argument. They are separate from Compose.Ports: those are published by the
	// stack and reachable as soon as it is up, while these exist only while the
	// command runs.
	ForwardPorts []string      `json:"forwardPorts,omitempty" yaml:"forwardPorts,omitempty"`
	Remote       *RemoteConfig `json:"remote,omitempty" yaml:"remote,omitempty"`
	Fingerprint  string        `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	// BuildUID/BuildGID are the host owner ids baked into a local-cached image so
	// devuser matches the bind-mounted workspace. They are resolved fresh at
	// generate time (never persisted) and feed both the compose build args and the
	// image fingerprint. Zero means "unset" — the Dockerfile ARG defaults apply.
	BuildUID int `json:"-" yaml:"-"`
	BuildGID int `json:"-" yaml:"-"`
}

const ConfigFile = "devcontainer.config.json"
const GeneratedHeader = `# ==============================================================================
# THIS WAS GENERATED BY DEVCONTAINER-CLI — IF YOU MODIFY IT MANUALLY, ANY
# CHANGES WILL BE LOST WHEN REGENERATED. EDIT devcontainer.config.json INSTEAD.
# ==============================================================================`
