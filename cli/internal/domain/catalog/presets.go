package catalog

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

type Preset struct {
	ID       string          `yaml:"id"`
	Label    string          `yaml:"label,omitempty"`
	Modules  []string        `yaml:"modules,omitempty"`
	Services []string        `yaml:"services,omitempty"`
	Mode     types.BuildMode `yaml:"mode,omitempty"`
	Source   string          `yaml:"-"`
}

var BuiltinPresets = []Preset{
	{
		ID:      "nodejs",
		Label:   "Node.js (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"nodejs", "pnpm", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "bun",
		Label:   "Bun (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"bun", "pnpm", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "java-temurin",
		Label:   "Java Temurin (GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"java-temurin", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "python",
		Label:   "Python (GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"python", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "go",
		Label:   "Go (GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"go", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "node-go",
		Label:   "Node.js + Go (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"nodejs", "pnpm", "go", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "node-python",
		Label:   "Node.js + Python (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"nodejs", "pnpm", "python", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "node-java-temurin",
		Label:   "Node.js + Java Temurin (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"nodejs", "pnpm", "java-temurin", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "bun-go",
		Label:   "Bun + Go (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"bun", "pnpm", "go", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "bun-python",
		Label:   "Bun + Python (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"bun", "pnpm", "python", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
	{
		ID:      "bun-java-temurin",
		Label:   "Bun + Java Temurin (pnpm, GitHub CLI, Tmux & AI Tools: Claude Code, OpenCode, Codex, Antigravity, Copilot)",
		Modules: []string{"bun", "pnpm", "java-temurin", "github-cli", "claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "tmux"},
	},
}

func Resolve(id string, userDir string) (Preset, bool) {
	for _, p := range LoadUserPresets(userDir) {
		if p.ID == id {
			p.Source = "user"
			return p, true
		}
	}
	for _, p := range BuiltinPresets {
		if p.ID == id {
			p.Source = "builtin"
			return p, true
		}
	}
	return Preset{}, false
}

func All(userDir string) []Preset {
	seen := map[string]bool{}
	var out []Preset
	for _, p := range LoadUserPresets(userDir) {
		seen[p.ID] = true
		p.Source = "user"
		out = append(out, p)
	}
	for _, p := range BuiltinPresets {
		if seen[p.ID] {
			continue
		}
		p.Source = "builtin"
		out = append(out, p)
	}
	return out
}

func LoadUserPresets(dir string) []Preset {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Preset
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var p Preset
		if err := yaml.Unmarshal(data, &p); err != nil {
			continue
		}
		if p.ID == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
