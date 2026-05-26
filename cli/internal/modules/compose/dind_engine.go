package compose

const DindEngineHost = "docker-dind"
const DindEnginePort = 2375
const DindEngineNetwork = "engine-network"
const DindImage = "docker:28-dind-rootless"

var DindEngineService = &ServiceSpec{
	ID:       DindEngineHost,
	Label:    `Rootless Docker-in-Docker engine (isolated sandbox for the "dind" access mode)`,
	Internal: true,
	Volumes:  []string{"dind_data"},
	Render: func(ctx RenderContext) *ServiceDef {
		return &ServiceDef{
			Image:         DindImage,
			ContainerName: DindEngineHost,
			Restart:       "unless-stopped",
			Privileged:    true,
			Environment:   []string{"DOCKER_TLS_CERTDIR="},
			Volumes:       []string{"dind_data:/home/rootless/.local/share/docker"},
			Networks:      []string{DindEngineNetwork},
		}
	},
}
