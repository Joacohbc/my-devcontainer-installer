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

// github-cli and zellij are listed first in every preset so their Dockerfile
// layers are shared with the base cache image (devcontainer-base), maximising
// Docker layer cache hits across all variant builds in CI.
var BuiltinPresets = []Preset{
	{
		ID:    "base",
		Label: "Base (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
		},
	},
	{
		ID:    "nodejs",
		Label: "Node.js (pnpm, GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
		},
	},
	{
		ID:    "bun",
		Label: "Bun (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleBun),
		},
	},
	{
		ID:    "java-temurin",
		Label: "Java Temurin (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleJavaTemurin),
		},
	},
	{
		ID:    "python",
		Label: "Python (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModulePython),
		},
	},
	{
		ID:    "go",
		Label: "Go (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleGolang),
		},
	},
	{
		ID:    "node-go",
		Label: "Node.js + Go (pnpm, GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModuleGolang),
		},
	},
	{
		ID:    "node-python",
		Label: "Node.js + Python (pnpm, GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModulePython),
		},
	},
	{
		ID:    "node-java-temurin",
		Label: "Node.js + Java Temurin (pnpm, GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModuleJavaTemurin),
		},
	},
	{
		ID:    "bun-go",
		Label: "Bun + Go (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleBun),
			string(types.ModuleGolang),
		},
	},
	{
		ID:    "bun-python",
		Label: "Bun + Python (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleBun),
			string(types.ModulePython),
		},
	},
	{
		ID:    "bun-java-temurin",
		Label: "Bun + Java Temurin (GitHub CLI, Zellij)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleZellij),
			string(types.ModuleBun),
			string(types.ModuleJavaTemurin),
		},
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
