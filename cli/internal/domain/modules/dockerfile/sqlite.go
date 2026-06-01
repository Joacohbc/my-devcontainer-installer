package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var SqliteModule = &ModuleSpec{
	ID:         types.ModuleSqlite,
	Label:      "SQLite",
	Category:   types.CategoryDB,
	UICategory: types.UICategoryDatabases,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## SQLITE
##
RUN apt-get update && apt-get install -y sqlite3 && %s
`, aptCleanup())
	},
}
