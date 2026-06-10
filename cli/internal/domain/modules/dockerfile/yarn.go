package dockerfile

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var YarnModule = &ModuleSpec{
	ID:         types.ModuleYarn,
	Label:      "Yarn (via Corepack, for devuser)",
	Category:   types.CategoryRuntime,
	UICategory: types.UICategoryLanguages,
	Requires:   []types.ModuleID{types.ModuleNodejs},
	Render: func(opts map[string]any) string {
		// Yarn ships with Node via Corepack. We source the node init script
		// (.nodejs_init.sh, written by the nodejs module for both nvm and fnm)
		// so `node`/`corepack` are on PATH at build time, then enable Corepack
		// and pin the stable Yarn release. The Corepack shims live in the
		// node-manager bin dir, so they are picked up by the same shell init at
		// runtime — no extra shell-init fragment is needed here.
		return `##
## YARN (devuser, via Corepack)
##
RUN su - devuser -c '. "$HOME/.nodejs_init.sh" && corepack enable && corepack prepare yarn@stable --activate'
`
	},
}
