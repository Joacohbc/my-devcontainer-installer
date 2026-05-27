package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var ClaudeCodeModule = &ModuleSpec{
	ID:       "claude-code",
	Label:    "Claude Code (native standalone installer)",
	Category: types.CategoryInfra,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-claude-code.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Claude Code — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
