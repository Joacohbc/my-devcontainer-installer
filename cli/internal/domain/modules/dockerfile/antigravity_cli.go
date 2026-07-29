package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var AntigravityCliModule = &ModuleSpec{
	ID:         types.ModuleAntigravityCli,
	Label:      "Antigravity CLI (native standalone installer)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-antigravity.sh"}
	},
	PostScriptAutoStart: true,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Antigravity CLI", "Run it with `antigravity`.", true,
			"Settings, plugins and skills live under `~/.gemini/antigravity-cli`;",
			"`~/.gemini` and `~/.antigravity` are symlinks into the shared volume.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Antigravity CLI — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
