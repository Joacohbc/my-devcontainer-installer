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
	// Ref is the single argument the Skills CLI resolves. It is either an
	// owner/repo shorthand, when the repo is one skill, or the URL of a skill's
	// own directory when the repo holds several — the CLI accepts both, so a
	// multi-skill repo needs no second argument and no invented syntax.
	Ref string
	// RequiresModules are the Dockerfile modules the skill's tooling needs, so a
	// project selecting a skill it cannot run is reported rather than discovered
	// once an agent tries to follow it.
	RequiresModules []types.ModuleID
	// Context is the skill's entry in the generated ~/CONTEXT.md.
	Context func() *types.ContextSection
}

// firecrawl/cli holds ten skills; the reference pins the one that teaches the
// CLI itself, so an unattended install cannot pull in the other nine.
var FirecrawlSkill = &Spec{
	ID:              types.SkillFirecrawl,
	Label:           "Firecrawl (scrape/crawl/map/search via the firecrawl CLI)",
	Ref:             "https://github.com/firecrawl/cli/tree/main/skills/firecrawl-cli",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Firecrawl skill",
			Body: "Teaches how to drive the `firecrawl` CLI to search, scrape, map and crawl\n" +
				"pages. Reaching the Firecrawl API needs `FIRECRAWL_API_KEY` in the\n" +
				"environment; without it only the local commands work.",
		}
	},
}

var AgentBrowserSkill = &Spec{
	ID:              types.SkillAgentBrowser,
	Label:           "Agent Browser (browser automation over CDP, for agents)",
	Ref:             "vercel-labs/agent-browser",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Agent Browser skill",
			Body: "Teaches how to drive the `agent-browser` CLI: navigate, fill forms, click\n" +
				"and extract from a real browser. It is a thin stub that fetches its\n" +
				"instructions from the installed CLI, so the two never disagree.",
		}
	},
}

// Anthropic's own webapp-testing skill: Python Playwright driving headless
// chromium, which is what the python and chrome modules put in the image.
var WebappTestingSkill = &Spec{
	ID:              types.SkillWebappTesting,
	Label:           "Webapp testing (Python Playwright on headless Chromium)",
	Ref:             "https://github.com/anthropics/skills/tree/main/skills/webapp-testing",
	RequiresModules: []types.ModuleID{types.ModulePython, types.ModuleChrome},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Webapp testing skill",
			Body: "Teaches driving a page with Python Playwright: screenshot first, inspect\n" +
				"the DOM, then act, waiting for `networkidle` before reading dynamic\n" +
				"content. Launch the system browser headless — there is no display, and\n" +
				"the binary is `/usr/bin/chromium`.",
		}
	},
}

// All is the ordered catalogue of installable skills.
var All = []*Spec{
	FirecrawlSkill,
	AgentBrowserSkill,
	WebappTestingSkill,
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
