package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var PnpmModule = &ModuleSpec{
	ID:       "pnpm",
	Label:    "pnpm (for devuser)",
	Category: types.CategoryRuntime,
	Requires: []string{"nodejs"},
	Render: func(opts map[string]any) string {
		return `##
## PNPM (devuser)
##
RUN apt-get update && apt-get install -y --no-install-recommends libatomic1 && rm -rf /var/lib/apt/lists/*
RUN su - devuser -c 'wget -qO- https://get.pnpm.io/install.sh | ENV="$HOME/.profile" SHELL="$(which zsh)" zsh -'
`
	},
}
