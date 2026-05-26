package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

var PostgresService = &ServiceSpec{
	ID:      "postgres",
	Label:   "PostgreSQL",
	Volumes: []string{"postgres_data"},
	Options: []core.ModuleOption{
		{
			ID:    "version",
			Label: "PostgreSQL version",
			Type:  core.ModuleOptionSelect,
			Choices: []core.ModuleOptionChoice{
				{Value: "17-alpine", Label: "PostgreSQL 17"},
				{Value: "18-alpine", Label: "PostgreSQL 18 (latest)"},
				{Value: "16-alpine", Label: "PostgreSQL 16"},
			},
			Default: "17-alpine",
		},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version, _ := ctx.Options["version"].(string)
		if version == "" {
			version = "17-alpine"
		}
		return &ServiceDef{
			Image:         fmt.Sprintf("postgres:%s", version),
			ContainerName: "postgres",
			Environment: map[string]string{
				"POSTGRES_USER":     "devuser",
				"POSTGRES_PASSWORD": "devpass",
				"POSTGRES_DB":       "devdb",
			},
			Volumes:  []string{"postgres_data:/var/lib/postgresql/data"},
			Networks: []string{"local-network"},
		}
	},
}
