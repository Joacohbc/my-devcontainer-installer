package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// defaultRedisVersion is the image tag used when no version is selected. It is
// shared by the option default, the rendered compose service and the
// ~/CONTEXT.md section, so the document can never name a version the container
// was not built with.
const defaultRedisVersion = "7.4-alpine"

var RedisService = &ServiceSpec{
	ID:         types.ServiceRedis,
	Label:      "Redis",
	UICategory: types.UICategoryDatabases,
	IsDatabase: true,
	Volumes:    []string{"redis_data"},
	Options: []types.ModuleOption{
		{
			ID:    "version",
			Label: "Redis version",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: defaultRedisVersion, Label: "Redis 7.4 (BSD-licensed last)"},
				{Value: "8.6-alpine", Label: "Redis 8.6 (latest, SSPL/RSALv2)"},
				{Value: "8.0-alpine", Label: "Redis 8.0"},
			},
			Default: defaultRedisVersion,
		},
	},
	Context: func(ctx RenderContext) *types.ContextSection {
		version := types.StringOpt(ctx.Options, "version", defaultRedisVersion)
		return &types.ContextSection{
			Title: "Redis (sibling container)",
			Body: ctxBody(
				"Running as a separate container on the project network — **not** on",
				"`localhost`. Reach it at host `redis`, port `6379`, image `redis:"+version+"`.",
				"",
				"No password is set.",
				"",
				"    redis-cli -h redis   # needs the redis-client module",
				"",
				"Its data lives in the `redis_data` volume, so it survives a restart but is",
				"deleted by `devcontainer-cli down -v` and `destroy`.",
			),
		}
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version := types.StringOpt(ctx.Options, "version", defaultRedisVersion)
		return &ServiceDef{
			Image:         fmt.Sprintf("redis:%s", version),
			ContainerName: "redis",
			Volumes:       []string{"redis_data:/data"},
			Networks:      []string{"local-network"},
		}
	},
}
