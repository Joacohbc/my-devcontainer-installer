package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var PythonModule = &ModuleSpec{
	ID:         types.ModulePython,
	Label:      "Python (python3 + pip, optional uv)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Options: []types.ModuleOption{
		{
			ID:      "uv",
			Label:   "Install uv (Astral, for devuser)",
			Type:    types.ModuleOptionConfirm,
			Default: true,
		},
	},
	Context: func(opts map[string]any) *types.ContextSection {
		if !types.BoolOpt(opts, "uv", true) {
			return &types.ContextSection{
				Title: "Python",
				Body: ctxBody(
					"`python3` and `pip` are installed. `uv` was **not** selected for this image,",
					"so use `pip install --user X`; `pip` is the real pip here, not a wrapper.",
				),
			}
		}
		return &types.ContextSection{
			Title: "Python — use `uv`, not `pip`",
			Body: ctxBody(
				"`uv` is the package manager in this container.",
				"",
				"| Instead of | Use |",
				"|---|---|",
				"| `pip install X` | `uv pip install X` |",
				"| `python script.py` | `uv run script.py` |",
				"| `pipx install X` | `uv tool install X` |",
				"| creating a venv | nothing — see below |",
				"",
				"`pip` and `pip3` are shell functions forwarding to `uv pip`, so the old",
				"commands keep working. **Do not create a virtualenv**: the container is the",
				"isolation boundary, and `UV_SYSTEM_PYTHON=1` is exported so `uv pip` targets",
				"the system interpreter directly.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		uv := types.BoolOpt(opts, "uv", true)
		if !uv {
			return fmt.Sprintf(`##
## PYTHON
##
RUN apt-get update && apt-get install -y python3 python3-pip && %s
`, aptCleanup())
		}
		// python3 + pip and uv (Astral installer for devuser) install in one RUN
		// layer; cleanup runs last so the apt cache never lands in the layer.
		return fmt.Sprintf(`##
## PYTHON
##
RUN apt-get update && apt-get install -y python3 python3-pip && \
    su - devuser -c 'curl -LsSf https://astral.sh/uv/install.sh | sh' && \
    %s
%s
`, aptCleanup(), emitShellInit(".python_init.sh", []string{`export PATH="$HOME/.local/bin:$PATH"`}))
	},
}
