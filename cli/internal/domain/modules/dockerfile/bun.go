package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var BunModule = &ModuleSpec{
	ID:       "bun",
	Label:    "Bun",
	Category: types.CategoryRuntime,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## BUN
##
RUN su - devuser -c "curl -fsSL https://bun.sh/install | bash"
%s
`, emitShellInit(".bun_init.sh", []string{
			`export BUN_INSTALL="$HOME/.bun"`,
			`export PATH="$BUN_INSTALL/bin:$PATH"`,
		}))
	},
}
