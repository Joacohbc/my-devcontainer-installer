package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

var RedisService = &ServiceSpec{
	ID:      "redis",
	Label:   "Redis",
	Volumes: []string{"redis_data"},
	Options: []core.ModuleOption{
		{
			ID:    "version",
			Label: "Redis version",
			Type:  core.ModuleOptionSelect,
			Choices: []core.ModuleOptionChoice{
				{Value: "7.4-alpine", Label: "Redis 7.4 (BSD-licensed last)"},
				{Value: "8.6-alpine", Label: "Redis 8.6 (latest, SSPL/RSALv2)"},
				{Value: "8.0-alpine", Label: "Redis 8.0"},
			},
			Default: "7.4-alpine",
		},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version, _ := ctx.Options["version"].(string)
		if version == "" {
			version = "7.4-alpine"
		}
		return &ServiceDef{
			Image:         fmt.Sprintf("redis:%s", version),
			ContainerName: "redis",
			Volumes:       []string{"redis_data:/data"},
			Networks:      []string{"local-network"},
		}
	},
}
