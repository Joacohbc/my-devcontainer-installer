// Package skills is the catalogue of agent skills a project can install, the
// third kind of catalogue entry next to Dockerfile modules and compose
// services.
//
// A skill is not image content. It is installed by the Skills CLI into the
// project's own directory at runtime, which is why a Spec carries a reference
// for `npx skills add` rather than anything the generator renders into a layer.
package skills

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

// Spec describes one installable agent skill.
type Spec struct {
	ID    types.SkillID
	Label string
	// Ref is what the Skills CLI resolves, e.g. "firecrawl/cli".
	Ref string
	// RequiresModules are the Dockerfile modules the skill's tooling needs, so
	// selecting a skill for a project that lacks them is a resolvable state
	// rather than a skill that installs and then cannot do anything.
	RequiresModules []types.ModuleID
	// Context is the skill's entry in the generated ~/CONTEXT.md.
	Context func() *types.ContextSection
}

var FirecrawlSkill = &Spec{
	ID:              types.SkillFirecrawl,
	Label:           "Firecrawl (scrape/crawl/extract via the firecrawl CLI)",
	Ref:             "firecrawl/cli",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Firecrawl skill",
			Body: "Teaches how to drive the `firecrawl` CLI to scrape, crawl and extract\n" +
				"structured data from web pages. Reaching the Firecrawl API needs\n" +
				"`FIRECRAWL_API_KEY` in the environment; without it only the local\n" +
				"commands work.",
		}
	},
}

// All is the ordered catalogue of installable skills.
var All = []*Spec{
	FirecrawlSkill,
}

// Get returns the spec for an id, or nil when the id is unknown.
func Get(id types.SkillID) *Spec {
	for _, s := range All {
		if s.ID == id {
			return s
		}
	}
	return nil
}
