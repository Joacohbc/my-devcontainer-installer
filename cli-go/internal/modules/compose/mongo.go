package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

var MongoService = &ServiceSpec{
	ID:      "mongo",
	Label:   "MongoDB",
	Volumes: []string{"mongo_data"},
	Options: []core.ModuleOption{
		{
			ID:    "version",
			Label: "MongoDB version",
			Type:  core.ModuleOptionSelect,
			Choices: []core.ModuleOptionChoice{
				{Value: "8.0", Label: "MongoDB 8.0 (LTS)"},
				{Value: "8.3", Label: "MongoDB 8.3 (latest)"},
				{Value: "7.0", Label: "MongoDB 7.0"},
			},
			Default: "8.0",
		},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version, _ := ctx.Options["version"].(string)
		if version == "" {
			version = "8.0"
		}
		return &ServiceDef{
			Image:         fmt.Sprintf("mongo:%s", version),
			ContainerName: "mongo",
			Environment: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": "devuser",
				"MONGO_INITDB_ROOT_PASSWORD": "devpass",
			},
			Volumes:  []string{"mongo_data:/data/db"},
			Networks: []string{"local-network"},
		}
	},
}
