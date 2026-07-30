package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CodexCliModule = &ModuleSpec{
	ID:         types.ModuleCodexCli,
	Label:      "Codex CLI (npx, no global install)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	Requires:   []types.ModuleID{types.ModuleNodejs},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-codex-cli.sh"}
	},
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Codex CLI", "Run it with `npx @openai/codex` — there is no globally installed binary.", false,
			"`~/post-script/install-codex-cli.sh` is the interactive launcher; it is not",
			"auto-started because it needs a terminal. Config lives in `~/.codex`, a",
			"symlink into the shared volume.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Codex CLI — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
