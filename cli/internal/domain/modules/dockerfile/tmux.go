package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var TmuxModule = &ModuleSpec{
	ID:         types.ModuleTmux,
	Label:      "Tmux",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## TMUX
##
RUN apt-get update && apt-get install -y tmux && %s
`, aptCleanup())
	},
}
