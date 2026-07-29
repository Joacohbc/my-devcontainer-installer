package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var OpencodeModule = &ModuleSpec{
	ID:         types.ModuleOpencode,
	Label:      "OpenCode (opencode.ai installer)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-opencode.sh"}
	},
	PostScriptAutoStart: true,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("OpenCode", "Run it with `opencode`.", true,
			"It installs to `~/.opencode/bin`, which its installer adds to `~/.profile`",
			"only — if `opencode` is not found in a zsh shell, that is why; call it by",
			"its full path or re-export the PATH entry.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## OpenCode — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
