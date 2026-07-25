package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ModuleID string

const (
	ModuleBase           ModuleID = "base"
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
	ModuleCleanup        ModuleID = "cleanup"
)

type ServiceID string

const (
	ServiceDevcontainer ServiceID = "devcontainer"
	ServiceMongo        ServiceID = "mongo"
	ServiceRedis        ServiceID = "redis"
	ServicePostgres     ServiceID = "postgres"
	ServiceTunnel       ServiceID = "tunnel"
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
// gets a unique /workspaces/<workspace> path (entrypoint aliases /workspace to
// it) so the path-keyed history of Claude Code/Antigravity does not collide in
// the shared config volume.
const WorkspaceRoot = "/workspaces"

// WorkspaceDir returns the in-container mount path for a workspace.
func WorkspaceDir(workspace string) string {
	return WorkspaceRoot + "/" + workspace
}

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
	BuildModeLocalCached BuildMode = "local-cached"
	BuildModeRemote      BuildMode = "remote"
)

var BuildModes = []BuildMode{BuildModeLocalCached, BuildModeRemote}

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

var VariantLabels = map[string]string{
	"nodejs":            "nodejs — Node.js only",
	"bun":               "bun — Bun only",
	"java-temurin":      "java-temurin — Java Temurin only",
	"python":            "python — Python only",
	"go":                "go — Go only",
	"node-go":           "node-go — Node.js + Go",
	"node-python":       "node-python — Node.js + Python",
	"node-java-temurin": "node-java-temurin — Node.js + Java Temurin",
	"bun-go":            "bun-go — Bun + Go",
	"bun-python":        "bun-python — Bun + Python",
	"bun-java-temurin":  "bun-java-temurin — Bun + Java Temurin",
}

func ParseVariant(v string) (string, error) {
	for _, rv := range RemoteVariants {
		if rv == v {
			return v, nil
		}
	}
	return "", fmt.Errorf("invalid --variant: %s. Expected one of: %s", v, strings.Join(RemoteVariants, ", "))
}

type RemoteConfig struct {
	Variant  string `json:"variant" yaml:"variant"`
	Registry string `json:"registry,omitempty" yaml:"registry,omitempty"`
}

type DockerfileConfig struct {
	Modules []SelectedModule `json:"modules" yaml:"modules"`
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
	Mode        BuildMode         `json:"mode" yaml:"mode"`
	Image       string            `json:"image" yaml:"image"`
	Workspace   string            `json:"workspace" yaml:"workspace"`
	Dockerfile  DockerfileConfig  `json:"dockerfile" yaml:"dockerfile"`
	Compose     ComposeConfig     `json:"compose" yaml:"compose"`
	Env         map[string]string `json:"env" yaml:"env"`
	Remote      *RemoteConfig     `json:"remote,omitempty" yaml:"remote,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
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
