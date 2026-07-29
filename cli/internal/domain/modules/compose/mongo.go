package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var MongoService = &ServiceSpec{
	ID:         types.ServiceMongo,
	Label:      "MongoDB",
	UICategory: types.UICategoryDatabases,
	IsDatabase: true,
	Volumes:    []string{"mongo_data"},
	Options: []types.ModuleOption{
		{
			ID:    "version",
			Label: "MongoDB version",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "8.0", Label: "MongoDB 8.0 (LTS)"},
				{Value: "8.3", Label: "MongoDB 8.3 (latest)"},
				{Value: "7.0", Label: "MongoDB 7.0"},
			},
			Default: "8.0",
		},
	},
	Context: func(ctx RenderContext) *types.ContextSection {
		version := types.StringOpt(ctx.Options, "version", "8.0")
		user, password := dbCredentials(ctx)
		return &types.ContextSection{
			Title: "MongoDB (sibling container)",
			Body: ctxBody(
				"Running as a separate container on the project network — **not** on",
				"`localhost`. Reach it at host `mongo`, port `27017`.",
				"",
				"- image: `mongo:"+version+"`",
				"- root user: `"+user+"`",
				"- root password: `"+password+"`",
				"",
				"    mongosh \"mongodb://"+user+":"+password+"@mongo:27017/?authSource=admin\"   # needs the mongo-client module",
				"",
				"Its data lives in the `mongo_data` volume, so it survives a restart but is",
				"deleted by `devcontainer-cli down -v` and `destroy`.",
			),
		}
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version := types.StringOpt(ctx.Options, "version", "8.0")
		user := ctx.DefaultDBUser
		if user == "" {
			user = "devuser"
		}
		pass := ctx.DefaultDBPassword
		if pass == "" {
			pass = "devpass"
		}
		return &ServiceDef{
			Image:         fmt.Sprintf("mongo:%s", version),
			ContainerName: "mongo",
			Environment: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": user,
				"MONGO_INITDB_ROOT_PASSWORD": pass,
			},
			Volumes:  []string{"mongo_data:/data/db"},
			Networks: []string{"local-network"},
		}
	},
}
