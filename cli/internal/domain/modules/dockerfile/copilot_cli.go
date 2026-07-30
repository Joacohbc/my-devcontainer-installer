package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CopilotCliModule = &ModuleSpec{
	ID:         types.ModuleCopilotCli,
	Label:      "GitHub Copilot CLI (standalone binary)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-copilot.sh"}
	},
	PostScriptAutoStart: true,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("GitHub Copilot CLI", "Run it with `copilot`.", true,
			"It authenticates through the GitHub CLI, so `gh auth status` must be green",
			"first.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## GitHub Copilot CLI — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
