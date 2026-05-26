package dockerfile

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

var DbclientsModule = &DockerfileModuleSpec{
	ID:       "dbclients",
	Label:    "Database clients (psql, redis-cli, mongosh)",
	Category: core.CategoryDB,
	Options: []core.ModuleOption{
		{
			ID:    "clients",
			Label: "Which clients",
			Type:  core.ModuleOptionMultiselect,
			Choices: []core.ModuleOptionChoice{
				{Value: "postgres", Label: "postgresql-client"},
				{Value: "redis", Label: "redis-tools"},
				{Value: "mongo", Label: "mongodb-mongosh"},
			},
			Default: []string{"postgres", "redis", "mongo"},
		},
	},
	Render: func(opts map[string]any) string {
		clients := stringsFromAny(opts["clients"], []string{"postgres", "redis", "mongo"})
		clientSet := make(map[string]bool, len(clients))
		for _, c := range clients {
			clientSet[c] = true
		}

		pkgs := []string{}
		if clientSet["postgres"] {
			pkgs = append(pkgs, "postgresql-client")
		}
		if clientSet["redis"] {
			pkgs = append(pkgs, "redis-tools")
		}

		mongoSetup := ""
		if clientSet["mongo"] {
			mongoSetup = `RUN curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | gpg --dearmor -o /etc/apt/keyrings/mongodb-server-8.0.gpg && \
    echo "deb [ arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/8.0 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-8.0.list
`
			pkgs = append(pkgs, "mongodb-mongosh")
		}

		if len(pkgs) == 0 {
			return ""
		}

		return fmt.Sprintf(`##
## DATABASE CLIENTS
##
%sRUN apt-get update && apt-get install -y \
    %s
`, mongoSetup, strings.Join(pkgs, " \\\n    "))
	},
}
