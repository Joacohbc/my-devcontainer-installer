package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var ClaudeCodeModule = &ModuleSpec{
	ID:         types.ModuleClaudeCode,
	Label:      "Claude Code (native standalone installer)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-claude-code.sh"}
	},
	PostScriptAutoStart: true,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Claude Code", "Run it with `claude`.", true,
			"Its config and session history live in `~/.claude` and `~/.claude.json`,",
			"both symlinks into the shared volume, so a login done in any container",
			"already applies here.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Claude Code — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
