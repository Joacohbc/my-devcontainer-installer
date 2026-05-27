package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var TmuxModule = &ModuleSpec{
	ID:       "tmux",
	Label:    "Tmux",
	Category: types.CategoryInfra,
	Render: func(opts map[string]any) string {
		return `##
## TMUX
##
RUN apt-get update && apt-get install -y tmux
`
	},
}
