package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var SqliteModule = &ModuleSpec{
	ID:         "sqlite",
	Label:      "SQLite",
	Category:   types.CategoryDB,
	UICategory: types.UICategoryDatabases,
	Render: func(opts map[string]any) string {
		return `##
## SQLITE
##
RUN apt-get update && apt-get install -y sqlite3
`
	},
}
