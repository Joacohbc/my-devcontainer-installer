package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var RustModule = &ModuleSpec{
	ID:         types.ModuleRust,
	Label:      "Rust (rustup, latest version)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## RUST
##
RUN apt-get update && apt-get install -y --no-install-recommends build-essential && %s
RUN su - devuser -c "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path"
%s
`, aptCleanup(), emitShellInit(".rust_init.sh", []string{
			`export PATH="$HOME/.cargo/bin:$PATH"`,
		}))
	},
}
