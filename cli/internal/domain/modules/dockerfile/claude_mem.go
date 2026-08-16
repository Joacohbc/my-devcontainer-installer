package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var ClaudeMemModule = &ModuleSpec{
	ID:         types.ModuleClaudeMem,
	Label:      "Claude-Mem (persistent cross-session agent memory)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	// claude-mem is an npx-based installer (Node >= 18).
	Requires: []types.ModuleID{types.ModuleNodejs},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-claude-mem.sh"}
	},
	// Wires itself into whatever agents are present, so it must run after the
	// agent installers (claude/antigravity/copilot/opencode).
	PostScriptAutoStart:  true,
	PostScriptStartOrder: 90,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Claude-Mem", "Captures session activity, compresses it, and injects relevant context back into future sessions automatically.", true,
			"It wires itself into every agent CLI present in this container — Claude Code",
			"via its plugin marketplace, OpenCode and Antigravity via their own installers",
			"— so those agents keep memory across sessions with no extra setup.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Claude-Mem — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
