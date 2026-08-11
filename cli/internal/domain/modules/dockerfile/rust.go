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
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Rust",
			Body: ctxBody(
				"Installed with rustup for `devuser`; `cargo` and `rustc` come from",
				"`~/.cargo/bin`. Manage toolchains with `rustup`, never with `apt`.",
				"`build-essential` is present, so crates with C dependencies compile.",
			),
		}
	},
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{PathEntries: []PathEntry{"$HOME/.cargo/bin"}}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## RUST
##
RUN apt-get update && apt-get install -y --no-install-recommends build-essential && \
    su - devuser -c "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path" && \
    %s
`, aptCleanup())
	},
}
