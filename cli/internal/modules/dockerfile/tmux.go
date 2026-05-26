package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/core"

var TmuxModule = &DockerfileModuleSpec{
	ID:       "tmux",
	Label:    "Tmux",
	Category: core.CategoryInfra,
	Render: func(opts map[string]any) string {
		return `##
## TMUX
##
RUN apt-get update && apt-get install -y tmux
`
	},
}
