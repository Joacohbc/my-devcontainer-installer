package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var TmuxModule = &ModuleSpec{
	ID:         types.ModuleTmux,
	Label:      "Tmux",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return `##
## TMUX
##
RUN apt-get update && apt-get install -y tmux
`
	},
}
