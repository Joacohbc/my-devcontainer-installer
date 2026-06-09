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
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## PNPM (devuser)
##
RUN apt-get update && apt-get install -y --no-install-recommends libatomic1 && \
    su - devuser -c 'export PNPM_HOME="$HOME/.local/share/pnpm" && wget -qO- https://get.pnpm.io/install.sh | ENV="$HOME/.profile" SHELL="$(which zsh)" zsh - && PNPM_BIN="$(find "$PNPM_HOME" -name pnpm -type f 2>/dev/null | head -n1)" && "$PNPM_BIN" config set store-dir "$HOME/.local/share/pnpm/store" --global' && \
    %s
%s
`, aptCleanup(), emitShellInit(".pnpm_init.sh", []string{
			`export PNPM_HOME="$HOME/.local/share/pnpm"`,
			`export PATH="$PNPM_HOME/bin:$PNPM_HOME:$PATH"`,
		}))
	},
}
