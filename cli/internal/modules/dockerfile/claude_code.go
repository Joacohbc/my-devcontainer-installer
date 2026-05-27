package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
)

var ClaudeCodeModule = &DockerfileModuleSpec{
	ID:       "claude-code",
	Label:    "Claude Code (native standalone installer)",
	Category: core.CategoryInfra,
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-claude-code.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Claude Code — install script shipped under %s\n##\n", core.PostScriptDir)
	},
}
