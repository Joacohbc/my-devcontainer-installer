package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// defaultMongoVersion is the image tag used when no version is selected. It is
// shared by the option default, the rendered compose service and the
// ~/CONTEXT.md section, so the document can never name a version the container
// was not built with.
const defaultMongoVersion = "8.0"

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
				{Value: defaultMongoVersion, Label: "MongoDB 8.0 (LTS)"},
				{Value: "8.3", Label: "MongoDB 8.3 (latest)"},
				{Value: "7.0", Label: "MongoDB 7.0"},
			},
			Default: defaultMongoVersion,
		},
	},
	Context: func(ctx RenderContext) *types.ContextSection {
		version := types.StringOpt(ctx.Options, "version", defaultMongoVersion)
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
		version := types.StringOpt(ctx.Options, "version", defaultMongoVersion)
		user, password := dbCredentials(ctx)
		return &ServiceDef{
			Image:         fmt.Sprintf("mongo:%s", version),
			ContainerName: "mongo",
			Environment: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": user,
				"MONGO_INITDB_ROOT_PASSWORD": password,
			},
			Volumes:  []string{"mongo_data:/data/db"},
			Networks: []string{"local-network"},
		}
	},
}
