package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"

var SqliteModule = &DockerfileModuleSpec{
	ID:       "sqlite",
	Label:    "SQLite",
	Category: core.CategoryDB,
	Render: func(opts map[string]any) string {
		return `##
## SQLITE
##
RUN apt-get update && apt-get install -y sqlite3
`
	},
}
