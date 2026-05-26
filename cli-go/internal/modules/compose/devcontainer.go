package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

const SSHServiceName = "devcontainer-ssh"
const HostDockerSocket = "/var/run/docker.sock"

type DockerSocketMode string

const (
	DockerSocketNone   DockerSocketMode = "none"
	DockerSocketSocket DockerSocketMode = "socket"
	DockerSocketDind   DockerSocketMode = "dind"
)

var DevcontainerService = &ServiceSpec{
	ID:    "devcontainer",
	Label: SSHServiceName + " (main)",
	Always: true,
	Options: []core.ModuleOption{
		{
			ID:    "dockerSocket",
			Label: "Docker access mode",
			Type:  core.ModuleOptionSelect,
			Choices: []core.ModuleOptionChoice{
				{Value: "none", Label: "none — no Docker access (safest)"},
				{Value: "socket", Label: "socket — mount the host /var/run/docker.sock (full host Docker, root-equivalent ⚠️)"},
				{Value: "dind", Label: "dind — isolated rootless Docker-in-Docker engine (sandbox)"},
			},
			Default:        "none",
			RequiresModule: "dod",
		},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		modeStr, _ := ctx.Options["dockerSocket"].(string)
		if modeStr == "" {
			modeStr = "none"
		}
		mode := DockerSocketMode(modeStr)

		dbServices := []string{"mongo", "redis", "postgres"}
		depends := []string{}
		for _, id := range ctx.EnabledServiceIDs {
			for _, db := range dbServices {
				if id == db {
					depends = append(depends, id)
				}
			}
		}

		volumes := []string{
			"../..:/workspace",
			"devcontainer_etc:/etc",
			"devcontainer_root:/root",
			"devcontainer_home:/home",
		}
		if mode == DockerSocketSocket {
			volumes = append(volumes, HostDockerSocket+":"+HostDockerSocket)
		}

		networks := []string{"local-network"}
		if mode == DockerSocketDind {
			networks = append(networks, DindEngineNetwork)
		}

		svc := &ServiceDef{
			Image:         ctx.ImageName,
			Build:         ".",
			ContainerName: SSHServiceName,
			Command:       "sleep infinity",
			Restart:       "unless-stopped",
			Volumes:       volumes,
			Networks:      networks,
		}

		if mode == DockerSocketDind {
			svc.Environment = []string{fmt.Sprintf("DOCKER_HOST=tcp://%s:%d", DindEngineHost, DindEnginePort)}
			depends = append(depends, DindEngineHost)
		}

		if len(depends) > 0 {
			svc.DependsOn = depends
		}

		return svc
	},
}
