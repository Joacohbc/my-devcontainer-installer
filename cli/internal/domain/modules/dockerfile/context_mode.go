package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var ContextModeModule = &ModuleSpec{
	ID:         types.ModuleContextMode,
	Label:      "Context Mode (context-window sandboxing + session persistence)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	// context-mode ships as a global npm package (Node >= 22.5, falls back
	// gracefully on older Node).
	Requires: []types.ModuleID{types.ModuleNodejs},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-context-mode.sh"}
	},
	// Wires itself into whatever agents are present, so it must run after the
	// agent installers (claude/antigravity/copilot/opencode).
	PostScriptAutoStart:  true,
	PostScriptStartOrder: 90,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Context Mode", "Sandboxes tool output and persists session state to cut context-window usage; check with `ctx stats` / `ctx doctor`.", true,
			"Wires into Claude Code via its plugin marketplace and GitHub Copilot CLI via",
			"its plugin manager, since both expose a scriptable install command. Every",
			"other supported platform needs a hand-edited MCP/hook config per its own",
			"docs; the `context-mode`/`ctx` binaries are still on PATH for that.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Context Mode — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
