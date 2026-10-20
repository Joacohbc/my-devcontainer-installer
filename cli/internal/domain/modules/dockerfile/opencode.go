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
	// The installer drops the binary in ~/.opencode/bin. Declared here rather
	// than exported from ~/.profile so `agent exec -- opencode` and zsh
	// sessions find it too.
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{PathEntries: []PathEntry{"$HOME/.opencode/bin"}}
	},
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("OpenCode", "Run it with `opencode`.", true,
			"It installs to `~/.opencode/bin`, which is on the image PATH.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## OpenCode — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
