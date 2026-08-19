package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var BrowserHarnessModule = &ModuleSpec{
	ID:         types.ModuleBrowserHarness,
	Label:      "Browser Harness (browser-use/browser-harness — CDP browser control)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	Requires:   []types.ModuleID{types.ModulePython, types.ModuleChrome, types.ModuleFfmpeg},
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{
			Assignments: []EnvVar{
				{Name: "CHROME_PATH", Value: EnvValue("/usr/bin/chromium")},
			},
		}
	},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-browser-harness.sh"}
	},
	PostScriptAutoStart:  true,
	PostScriptStartOrder: 90,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("Browser Harness", "Direct local browser control via CDP (Chrome DevTools Protocol) with self-healing helpers and video recording support.", true,
			"`browser-harness` is installed and wired into detected agents (Claude Code, Codex, Antigravity, Copilot, OpenCode).",
			"",
			"Chromium (`/usr/bin/chromium`) and FFmpeg are installed locally for headless browser automation and video generation.",
			"",
			"Run scripts via heredoc:",
			"```bash",
			"browser-harness <<'PY'",
			"print(page_info())",
			"PY",
			"```",
			"",
			"Diagnostics and recordings:",
			"- `browser-harness --doctor` : check CDP connectivity.",
			"- `browser-harness recordings` : inspect or manage session recording traces.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Browser Harness — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
