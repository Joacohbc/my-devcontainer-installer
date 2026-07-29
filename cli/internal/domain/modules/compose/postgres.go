package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var PostgresService = &ServiceSpec{
	ID:         types.ServicePostgres,
	Label:      "PostgreSQL",
	UICategory: types.UICategoryDatabases,
	IsDatabase: true,
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
	Context: func(ctx RenderContext) *types.ContextSection {
		version := types.StringOpt(ctx.Options, "version", "17-alpine")
		user, password := dbCredentials(ctx)
		return &types.ContextSection{
			Title: "PostgreSQL (sibling container)",
			Body: ctxBody(
				"Running as a separate container on the project network — **not** on",
				"`localhost`. Reach it at host `postgres`, port `5432`.",
				"",
				"- image: `postgres:"+version+"`",
				"- database: `devdb`",
				"- user: `"+user+"`",
				"- password: `"+password+"`",
				"",
				"    psql -h postgres -U "+user+" devdb   # needs the postgres-client module",
				"",
				"Its data lives in the `postgres_data` volume, so it survives a restart but",
				"is deleted by `devcontainer-cli down -v` and `destroy`.",
			),
		}
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version := types.StringOpt(ctx.Options, "version", "17-alpine")
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
