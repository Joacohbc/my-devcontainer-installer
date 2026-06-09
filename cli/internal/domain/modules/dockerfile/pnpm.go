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
		// pnpm aborts every command (even `config set`) when its configured
		// global bin dir is not on PATH, and that dir has moved between pnpm
		// releases ($PNPM_HOME vs $PNPM_HOME/bin), so both candidates go on
		// PATH before invoking it. The store-dir must be set through
		// `pnpm config set --global` (not by writing a file): pnpm >= 11
		// reads ~/.config/pnpm/config.yaml and ignores the legacy rc format.
		return fmt.Sprintf(`##
## PNPM (devuser)
##
RUN apt-get update && apt-get install -y --no-install-recommends libatomic1 && \
    su - devuser -c 'export PNPM_HOME="$HOME/.local/share/pnpm" && wget -qO- https://get.pnpm.io/install.sh | ENV="$HOME/.profile" SHELL="$(which zsh)" zsh - && export PATH="$PNPM_HOME/bin:$PNPM_HOME:$PATH" && pnpm config set store-dir "$HOME/.local/share/pnpm/store" --global' && \
    %s
%s
`, aptCleanup(), emitShellInit(".pnpm_init.sh", []string{
			`export PNPM_HOME="$HOME/.local/share/pnpm"`,
			`export PATH="$PNPM_HOME/bin:$PNPM_HOME:$PATH"`,
		}))
	},
}
