package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var RedisService = &ServiceSpec{
	ID:         types.ServiceRedis,
	Label:      "Redis",
	UICategory: types.UICategoryDatabases,
	Volumes:    []string{"redis_data"},
	Options: []types.ModuleOption{
		{
			ID:    "version",
			Label: "Redis version",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "7.4-alpine", Label: "Redis 7.4 (BSD-licensed last)"},
				{Value: "8.6-alpine", Label: "Redis 8.6 (latest, SSPL/RSALv2)"},
				{Value: "8.0-alpine", Label: "Redis 8.0"},
			},
			Default: "7.4-alpine",
		},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version := types.StringOpt(ctx.Options, "version", "7.4-alpine")
		return &ServiceDef{
			Image:         fmt.Sprintf("redis:%s", version),
			ContainerName: "redis",
			Volumes:       []string{"redis_data:/data"},
			Networks:      []string{"local-network"},
		}
	},
}
