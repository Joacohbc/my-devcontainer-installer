package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var BunModule = &ModuleSpec{
	ID:         types.ModuleBun,
	Label:      "Bun",
	Category:   types.CategoryRuntime,
	UICategory: types.UICategoryLanguages,
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Bun",
			Body: ctxBody(
				"Installed at `~/.bun` and on PATH. Use it for projects that already have a",
				"`bun.lockb`; for everything else the JS package manager here is pnpm.",
			),
		}
	},
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{
			Assignments: []EnvVar{{Name: "BUN_INSTALL", Value: "$HOME/.bun"}},
			PathEntries: []PathEntry{"$HOME/.bun/bin"},
		}
	},
	Render: func(opts map[string]any) string {
		return `##
## BUN
##
RUN su - devuser -c "curl -fsSL https://bun.sh/install | bash"
`
	},
}
