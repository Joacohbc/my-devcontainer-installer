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
	ModulePostgresClient ModuleID = "postgres-client"
	ModuleRedisClient    ModuleID = "redis-client"
	ModuleMongoClient    ModuleID = "mongo-client"
	ModuleNodejs         ModuleID = "nodejs"
	ModulePnpm           ModuleID = "pnpm"
	ModuleBun            ModuleID = "bun"
	ModuleClaudeCode     ModuleID = "claude-code"
	ModuleOpencode       ModuleID = "opencode"
	ModuleCodexCli       ModuleID = "codex-cli"
	ModuleAntigravityCli ModuleID = "antigravity-cli"
	ModuleCopilotCli     ModuleID = "copilot-cli"
	ModuleGraphify       ModuleID = "graphify"
	ModuleCaveman        ModuleID = "caveman"
	ModuleTmux           ModuleID = "tmux"
	ModuleDod            ModuleID = "dod"
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

// WorkspaceRoot is the parent dir the project is mounted under. Each project
// gets a unique /workspaces/<workspace> path (entrypoint aliases /workspace to
// it) so the path-keyed history of Claude Code/Antigravity does not collide in
// the shared config volume.
const WorkspaceRoot = "/workspaces"

// WorkspaceDir returns the in-container mount path for a workspace.
func WorkspaceDir(workspace string) string {
	return WorkspaceRoot + "/" + workspace
}

const DefaultSSHHostPort = 2222

// SSHKeyName is the filename of the single shared SSH key reused by every
// devcontainer (local and remote). The managed key lives under the CLI global
// config dir; sshdefaults.KeyName aliases this value.
const SSHKeyName = "id_devcontainer"

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
	Services []any  `json:"services" yaml:"services"`
	Subnet   string `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	// PersistVolumes are user-chosen named-volume mounts for the devcontainer,
	// as "<volume>:</absolute/path>" specs (e.g. "cache:/var/cache"). Volume
	// names are workspace-prefixed like every other named volume. Nil or empty
	// means none — there are NO default persistence volumes. The legacy ids
	// "etc", "root" and "home" written by older CLIs are still accepted and
	// translated to their historical mounts.
	PersistVolumes *[]string `json:"persistVolumes,omitempty" yaml:"persistVolumes,omitempty"`
	// Ports are the docker port mappings published on the devcontainer service
	// (e.g. "8080:80"). Specs without an explicit host IP are bound to 127.0.0.1
	// in the generated compose so they are not reachable from the LAN; include an
	// IP (e.g. "0.0.0.0:8080:80") to override. Empty/nil means no published ports.
	Ports []string `json:"ports,omitempty" yaml:"ports,omitempty"`
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
}

const ConfigFile = "devcontainer.config.json"
const GeneratedHeader = `# ==============================================================================
# THIS WAS GENERATED BY DEVCONTAINER-CLI — IF YOU MODIFY IT MANUALLY, ANY
# CHANGES WILL BE LOST WHEN REGENERATED. EDIT devcontainer.config.json INSTEAD.
# ==============================================================================`

func NormalizeServices(services []any) []SelectedModule {
	result := make([]SelectedModule, 0, len(services))
	for _, s := range services {
		switch v := s.(type) {
		case string:
			result = append(result, SelectedModule{ID: ModuleID(v), Options: map[string]any{}})
		case map[string]any:
			raw, err := json.Marshal(v)
			if err != nil {
				continue
			}
			var sm SelectedModule
			if err := json.Unmarshal(raw, &sm); err != nil {
				continue
			}
			if sm.Options == nil {
				sm.Options = map[string]any{}
			}
			result = append(result, sm)
		case SelectedModule:
			if v.Options == nil {
				v.Options = map[string]any{}
			}
			result = append(result, v)
		}
	}
	return result
}
