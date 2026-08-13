package catalog

import (
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/skills"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// userSkillManifest is the on-disk shape of a user-defined agent skill: the
// same fields skills.Spec carries, minus the Go-only Context func — a plain
// title/body pair stands in for it, the same idea as a module's Context
// section. A skill has no scripts/directory shape like a profile does: its
// install source is already a git ref the Skills CLI resolves, not files this
// repo ships, so a flat <id>.yml is the only shape.
type userSkillManifest struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
	// Ref is the source the Skills CLI resolves: an owner/repo shorthand or a
	// repository URL. Skill names the one skill to take when Ref holds several
	// (see skills.Spec.Skill for why this matters).
	Ref             string   `yaml:"ref"`
	Skill           string   `yaml:"skill,omitempty"`
	RequiresModules []string `yaml:"requires_modules,omitempty"`
	// Context becomes the skill's entry in the generated ~/CONTEXT.md, same
	// section shape a Dockerfile module renders. Omit it and the skill simply
	// has nothing to say there — not an error.
	Context *struct {
		Title string `yaml:"title"`
		Body  string `yaml:"body"`
	} `yaml:"context,omitempty"`
}

func (m userSkillManifest) toSpec() *skills.Spec {
	requires := make([]types.ModuleID, 0, len(m.RequiresModules))
	for _, id := range m.RequiresModules {
		requires = append(requires, types.ModuleID(id))
	}
	spec := &skills.Spec{
		ID:              types.SkillID(m.ID),
		Label:           m.Label,
		Ref:             m.Ref,
		Skill:           m.Skill,
		RequiresModules: requires,
	}
	if m.Context != nil && m.Context.Body != "" {
		title, body := m.Context.Title, m.Context.Body
		spec.Context = func() *types.ContextSection {
			return &types.ContextSection{Title: title, Body: body}
		}
	}
	return spec
}

// LoadUserSkills reads every user-defined agent skill in dir (one <id>.yml or
// <id>.yaml each). An entry missing its id or ref — the two fields nothing
// else can stand in for — is skipped rather than erroring, the same tolerance
// LoadUserProfiles gives a malformed profile: one bad file must not break
// every other skill or profile the wizard would otherwise offer.
func LoadUserSkills(dir string) []*skills.Spec {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []*skills.Spec
	for _, e := range entries {
		if e.IsDir() || !hasYAMLSuffix(e.Name()) {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var m userSkillManifest
		if yaml.Unmarshal(data, &m) != nil || m.ID == "" || m.Ref == "" {
			continue
		}
		out = append(out, m.toSpec())
	}
	return out
}
