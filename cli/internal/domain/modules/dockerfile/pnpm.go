package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var PnpmModule = &ModuleSpec{
	ID:         types.ModulePnpm,
	Label:      "pnpm (for devuser)",
	Category:   types.CategoryRuntime,
	UICategory: types.UICategoryLanguages,
	Requires:   []types.ModuleID{types.ModuleNodejs},
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "JavaScript / TypeScript — use `pnpm`, not `npm`",
			Body: ctxBody(
				"| Instead of | Use |",
				"|---|---|",
				"| `npm install` | `pnpm install` |",
				"| `npm install X` | `pnpm add X` |",
				"| `npm run X` | `pnpm run X` |",
				"| `npx X` | `pnpm dlx X` |",
				"",
				"`npm` and `npx` are already aliased to `pnpm` and `pnpm dlx`.",
				"",
				"**Respect a lockfile that is already in the repo.** If the project has a",
				"`package-lock.json` and no `pnpm-lock.yaml`, run `command npm …` instead of",
				"silently switching the project's package manager.",
			),
		}
	},
	// Both $PNPM_HOME and $PNPM_HOME/bin are on PATH because the global bin dir
	// moved between pnpm releases and pnpm aborts every command when its
	// configured one is missing from PATH.
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{
			Assignments: []EnvVar{{Name: "PNPM_HOME", Value: "$HOME/.local/share/pnpm"}},
			PathEntries: []PathEntry{"$HOME/.local/share/pnpm/bin", "$HOME/.local/share/pnpm"},
		}
	},
	Render: func(opts map[string]any) string {
		// The install step exports PATH itself because the image environment is
		// declared at the end of the Dockerfile, after every module ran. The
		// store-dir must be set through
		// `pnpm config set --global` (not by writing a file): pnpm >= 11
		// reads ~/.config/pnpm/config.yaml and ignores the legacy rc format.
		return fmt.Sprintf(`##
## PNPM (devuser)
##
RUN apt-get update && apt-get install -y --no-install-recommends libatomic1 && \
    su - devuser -c 'export PNPM_HOME="$HOME/.local/share/pnpm" && wget -qO- https://get.pnpm.io/install.sh | ENV="$HOME/.profile" SHELL="$(which zsh)" zsh - && export PATH="$PNPM_HOME/bin:$PNPM_HOME:$PATH" && pnpm config set store-dir "$HOME/.local/share/pnpm/store" --global' && \
    %s
`, aptCleanup())
	},
}
