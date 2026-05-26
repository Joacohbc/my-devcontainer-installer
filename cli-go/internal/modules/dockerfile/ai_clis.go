package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

var allAiCliIDs = []string{
	"claude-code",
	"opencode",
	"codex-cli",
	"antigravity-cli",
	"copilot-cli",
}

var scriptByTool = map[string]string{
	"claude-code":     "install-claude-code.sh",
	"opencode":        "install-opencode.sh",
	"codex-cli":       "install-codex-cli.sh",
	"antigravity-cli": "install-antigravity.sh",
	"copilot-cli":     "install-copilot.sh",
}

var AiClisModule = &DockerfileModuleSpec{
	ID:       "ai-clis",
	Label:    "AI CLIs install scripts (Claude Code, OpenCode, Codex CLI, Antigravity, Copilot CLI)",
	Category: core.CategoryInfra,
	Requires: []string{"pnpm", "github-cli"},
	Options: []core.ModuleOption{
		{
			ID:    "tools",
			Label: "AI CLIs to ship install scripts for",
			Type:  core.ModuleOptionMultiselect,
			Choices: []core.ModuleOptionChoice{
				{Value: "claude-code", Label: "Claude Code (native standalone installer)"},
				{Value: "opencode", Label: "OpenCode (opencode.ai installer)"},
				{Value: "codex-cli", Label: "Codex CLI (npx, no global install)"},
				{Value: "antigravity-cli", Label: "Antigravity CLI (native standalone installer)"},
				{Value: "copilot-cli", Label: "GitHub Copilot CLI (standalone binary)"},
			},
			Default: allAiCliIDs,
		},
	},
	PostScriptFiles: func(opts map[string]any) []string {
		tools := normalizeAiTools(opts["tools"])
		scripts := make([]string, 0, len(tools))
		for _, t := range tools {
			if script, ok := scriptByTool[t]; ok {
				scripts = append(scripts, script)
			}
		}
		return scripts
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## AI CLIs — install scripts shipped under %s\n##\n", core.PostScriptDir)
	},
}

func normalizeAiTools(raw any) []string {
	arr, ok := raw.([]any)
	if !ok {
		return allAiCliIDs
	}
	rawSet := make(map[string]bool, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			rawSet[s] = true
		}
	}
	result := make([]string, 0, len(arr))
	for _, id := range allAiCliIDs {
		if rawSet[id] {
			result = append(result, id)
		}
	}
	return result
}
