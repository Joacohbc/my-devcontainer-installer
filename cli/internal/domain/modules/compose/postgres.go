package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var PostgresService = &ServiceSpec{
	ID:         "postgres",
	Label:      "PostgreSQL",
	UICategory: types.UICategoryDatabases,
	Volumes:    []string{"postgres_data"},
	Options: []types.ModuleOption{
		{
			ID:    "version",
			Label: "PostgreSQL version",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
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
		user := ctx.DefaultDBUser
		if user == "" {
			user = "devuser"
		}
		pass := ctx.DefaultDBPassword
		if pass == "" {
			pass = "devpass"
		}
		return &ServiceDef{
			Image:         fmt.Sprintf("postgres:%s", version),
			ContainerName: "postgres",
			Environment: map[string]string{
				"POSTGRES_USER":     user,
				"POSTGRES_PASSWORD": pass,
				"POSTGRES_DB":       "devdb",
			},
			Volumes:  []string{"postgres_data:/var/lib/postgresql/data"},
			Networks: []string{"local-network"},
		}
	},
}
