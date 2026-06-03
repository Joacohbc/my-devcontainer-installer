package compose

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

type ServiceSpec struct {
	ID             types.ServiceID
	Label          string
	UICategory     types.UICategory
	Always         bool
	RequiresModule types.ModuleID
	Internal       bool
	Options        []types.ModuleOption
	RequiresEnv    []types.RequiredEnvVar
	Volumes        []string
	Render         func(ctx RenderContext) *ServiceDef
}

type RenderContext struct {
	ImageName         string
	EnabledServiceIDs []string
	Options           map[string]any
	DefaultDBUser     string
	DefaultDBPassword string
	// PersistVolumeMounts are the optional persistence volume mounts (e.g.
	// "devcontainer_etc:/etc") to add to the devcontainer service, on top of the
	// always-present workspace bind mount. Only the devcontainer service reads it.
	PersistVolumeMounts []string
}

type ComposeDoc struct {
	Name     string                 `yaml:"name"`
	Services map[string]*ServiceDef `yaml:"services"`
	Networks map[string]*NetworkDef `yaml:"networks"`
	Volumes  map[string]*VolumeDef  `yaml:"volumes"`
}

type ServiceDef struct {
	Image         string            `yaml:"image,omitempty"`
	Build         string            `yaml:"build,omitempty"`
	ContainerName string            `yaml:"container_name"`
	Command       string            `yaml:"command,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Privileged    bool              `yaml:"privileged,omitempty"`
	Environment   any               `yaml:"environment,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Networks      any               `yaml:"networks,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
}

type NetworkDef struct {
	Driver string            `yaml:"driver,omitempty"`
	IPAM   *IPAMConfig       `yaml:"ipam,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

type VolumeDef struct {
	Labels map[string]string `yaml:"labels,omitempty"`
}

type IPAMConfig struct {
	Config []IPAMSubnet `yaml:"config"`
}

type IPAMSubnet struct {
	Subnet string `yaml:"subnet"`
}
