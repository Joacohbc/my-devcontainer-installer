package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var HerdrModule = &ModuleSpec{
	ID:         types.ModuleHerdr,
	Label:      "Herdr (herdr.dev CLI agent)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-herdr.sh"}
	},
	PostScriptAutoStart: true,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Herdr", "Run it with `herdr`.", true,
			"It installs to `~/.local/bin/herdr` and stores its configuration under",
			"`~/.config/herdr` (symlinked into the shared volume).")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Herdr — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
