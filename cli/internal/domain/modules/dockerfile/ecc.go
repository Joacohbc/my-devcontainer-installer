package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// EccProfiles are the module bundles ECC's own installer resolves
// (manifests/install-profiles.json upstream). 'developer' is its default
// engineering profile and the one this module defaults to.
var EccProfiles = []types.ModuleOptionChoice{
	{Value: "developer", Label: "developer (default: rules, agents, commands, hooks, framework/db/orchestration)"},
	{Value: "core", Label: "core (rules, agents, commands, hooks, platform configs)"},
	{Value: "minimal", Label: "minimal (no hook runtime)"},
	{Value: "security", Label: "security (core + security guidance)"},
	{Value: "research", Label: "research (core + research/content workflows)"},
	{Value: "full", Label: "full (every classified module)"},
}

var EccModule = &ModuleSpec{
	ID:         types.ModuleEcc,
	Label:      "ECC (Everything Claude Code - agent harness optimization & skills)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	Requires:   []types.ModuleID{types.ModuleNodejs},
	Options: []types.ModuleOption{
		{
			ID:      "profile",
			Label:   "ECC install profile",
			Type:    types.ModuleOptionSelect,
			Choices: EccProfiles,
			Default: "developer",
		},
	},
	// The installer reads this rather than taking an argument, because the
	// entrypoint runs start.d scripts with no arguments.
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{Assignments: []EnvVar{{
			Name:  "DEVCONTAINER_ECC_PROFILE",
			Value: EnvValue(types.StringOpt(opts, "profile", "developer")),
		}}}
	},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-ecc.sh"}
	},
	PostScriptAutoStart:  true,
	PostScriptStartOrder: 90,
	Context: func(opts map[string]any) *types.ContextSection {
		profile := types.StringOpt(opts, "profile", "developer")
		return agentCtx("ECC", "Agent harness optimization system: agents, rules, commands, hooks, MCP configs and skills, plus AgentShield security scanning.", true,
			"Installed project-scoped into the workspace (`/workspaces/<workspace>`) by ECC's own",
			"installer, with the **"+profile+"** profile. Claude Code gets `./.claude/`",
			"(agents, `rules/ecc`, commands, hooks, skills, scripts); Antigravity gets `./.agent/`",
			"with its rules and workflows, and OpenCode `~/.opencode/` — each only when its CLI",
			"is in this image.",
			"",
			"The global CLIs are `ecc` (the installer and its subcommands — `ecc doctor`,",
			"`ecc consult`, ...) and `agentshield`; `ecc-universal`/`ecc-agentshield` are the npm",
			"package names, not commands. Re-run or change the selection with",
			"`ecc --target claude-project --profile <name>` from the workspace root.",
			"",
			"Codex is deliberately not wired: its ECC target writes to `~/.codex`, which is a",
			"symlink into the shared config volume, so it would leak this project's ECC into",
			"every other container.")
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## ECC — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
