package compose

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

type ServiceSpec struct {
	ID             types.ServiceID
	Label          string
	UICategory     types.UICategory
	Always         bool
	IsDatabase     bool
	RequiresModule types.ModuleID
	Internal       bool
	Options        []types.ModuleOption
	RequiresEnv    []types.RequiredEnvVar
	Volumes        []string
	Render         func(ctx RenderContext) *ServiceDef
}

type RenderContext struct {
	ImageName         string
	Options           map[string]any
	DefaultDBUser     string
	DefaultDBPassword string
	// Ports are the resolved docker port mappings (e.g. "127.0.0.1:8080:80") to
	// publish on the devcontainer service. Only the devcontainer service reads it.
	Ports []string
	// SharedConfigMount is the global shared tool-config volume spec (e.g.
	// "devcontainer-shared-config:/mnt/shared-config"). Empty means disabled.
	// Only the devcontainer service reads it.
	SharedConfigMount string
	// WorkspaceDir is the in-container path the project is mounted at (e.g.
	// "/workspaces/myproj"). Empty defaults to "/workspace". Making it unique
	// per project keeps the path-keyed history of Claude Code/Antigravity from
	// colliding in the shared config volume. Only the devcontainer service reads it.
	WorkspaceDir string
}

type ComposeDoc struct {
	Name     string                 `yaml:"name"`
	Services map[string]*ServiceDef `yaml:"services"`
	Networks map[string]*NetworkDef `yaml:"networks"`
	Volumes  map[string]*VolumeDef  `yaml:"volumes"`
}

// BuildDef is the long-form compose `build:` block, used when the build needs
// build args (e.g. the host UID/GID baked into a local-cached image). A bare
// context string is also valid for `Build`, but local-cached always carries args.
type BuildDef struct {
	Context string            `yaml:"context"`
	Args    map[string]string `yaml:"args,omitempty"`
}

type ServiceDef struct {
	Image         string   `yaml:"image,omitempty"`
	Build         any      `yaml:"build,omitempty"`
	ContainerName string   `yaml:"container_name"`
	Hostname      string   `yaml:"hostname,omitempty"`
	Command       string   `yaml:"command,omitempty"`
	Restart       string   `yaml:"restart,omitempty"`
	Privileged    bool     `yaml:"privileged,omitempty"`
	Environment   any      `yaml:"environment,omitempty"`
	Volumes       []string `yaml:"volumes,omitempty"`
	Ports         []string `yaml:"ports,omitempty"`
	// Networks is any because compose accepts two shapes: a plain []string of
	// network names, or a map keyed by name (used to pin the devcontainer's
	// ipv4_address). See remapServiceNetworks in the generator.
	Networks  any               `yaml:"networks,omitempty"`
	DependsOn []string          `yaml:"depends_on,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type NetworkDef struct {
	Driver string            `yaml:"driver,omitempty"`
	IPAM   *IPAMConfig       `yaml:"ipam,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

type VolumeDef struct {
	External bool              `yaml:"external,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty"`
}

type IPAMConfig struct {
	Config []IPAMSubnet `yaml:"config"`
}

type IPAMSubnet struct {
	Subnet string `yaml:"subnet"`
}
