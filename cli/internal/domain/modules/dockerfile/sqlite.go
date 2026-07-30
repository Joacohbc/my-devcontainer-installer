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
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "SQLite",
			Body: ctxBody(
				"`sqlite3` is installed. A database file under `/workspace` survives the",
				"container; one written anywhere else does not.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## SQLITE
##
RUN apt-get update && apt-get install -y sqlite3 && %s
`, aptCleanup())
	},
}
