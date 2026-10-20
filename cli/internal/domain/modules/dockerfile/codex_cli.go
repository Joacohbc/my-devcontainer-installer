package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CodexCliModule = &ModuleSpec{
	ID:         types.ModuleCodexCli,
	Label:      "Codex CLI (@openai/codex)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	Requires:   []types.ModuleID{types.ModuleNodejs},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-codex-cli.sh"}
	},
	PostScriptAutoStart: true,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Codex CLI", "Run it with `codex` from the project directory.", true,
			"Config lives in `~/.codex`, a symlink into the shared volume.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Codex CLI — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
