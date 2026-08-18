package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// EccModule puts ECC's CLIs on PATH and stops there. It deliberately does NOT
// run `ecc --target <t>` for you, and therefore has no profile option:
//
// every ECC install target writes into the project (`.claude/`, `.agent/`) or
// into the home, i.e. into files the user owns and very likely tracks in git.
// Doing that unattended on first boot means a container start silently rewrites
// the user's checkout with a module bundle nobody chose. The install is one
// command away once the tools are here, and running it is the user's call —
// the same reason the skills module defaults to `manual`.
var EccModule = &ModuleSpec{
	ID:         types.ModuleEcc,
	Label:      "ECC (Everything Claude Code - agent harness optimization & skills)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	Requires:   []types.ModuleID{types.ModuleNodejs},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-ecc.sh"}
	},
	PostScriptAutoStart:  true,
	PostScriptStartOrder: 90,
	Context: func(opts map[string]any) *types.ContextSection {
		return agentCtx("ECC", "Agent harness optimization system: agents, rules, commands, hooks, MCP configs and skills, plus AgentShield security scanning.", true,
			"Only the CLIs are installed: `ecc` (the installer and its subcommands —",
			"`ecc doctor`, `ecc consult`, ...) and `agentshield`. `ecc-universal`/",
			"`ecc-agentshield` are the npm package names, not commands.",
			"",
			"**No content is laid down until you ask for it.** Every ECC target writes",
			"into files you own, so nothing runs unattended. From the workspace root:",
			"",
			"```",
			"ecc --target claude-project --profile developer   # ./.claude/",
			"ecc --target antigravity    --profile developer   # ./.agent/",
			"```",
			"",
			"`ecc --target <t> --dry-run` shows the plan first. Profiles are `developer`",
			"(upstream's default), `core`, `minimal`, `security`, `research` and `full`.",
			"",
			"Prefer a project target: the home ones (`claude`, `codex`, ...) write to",
			"`~/.claude` / `~/.codex`, which are symlinks into the shared config volume,",
			"so they would leak this project's ECC into every other container.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## ECC — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
