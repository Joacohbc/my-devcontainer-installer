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
		ID:       "fullstack-node",
		Label:    "Node.js full-stack (Node + Go + Postgres + Redis)",
		Modules:  []string{"nodejs", "golang"},
		Services: []string{"postgres", "redis"},
	},
	{
		ID:       "python-data",
		Label:    "Python data (Python + Postgres)",
		Modules:  []string{"python"},
		Services: []string{"postgres"},
	},
	{
		ID:       "node-mongo",
		Label:    "Node.js + MongoDB",
		Modules:  []string{"nodejs"},
		Services: []string{"mongo"},
	},
	{
		ID:       "go-only",
		Label:    "Go only",
		Modules:  []string{"golang"},
		Services: nil,
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
