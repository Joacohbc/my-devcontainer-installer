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
	// Ref is the source the Skills CLI resolves: an owner/repo shorthand or a
	// repository URL.
	Ref string
	// Skill names the one skill to take when Ref holds several. Selecting it by
	// name (`skills add <ref> --skill <name>`) rather than by pointing Ref at the
	// skill's directory URL is what keeps the reference free of a branch name —
	// a repo that renames master to main would break the URL form.
	Skill string
	// RequiresModules are the Dockerfile modules the skill's tooling needs, so a
	// project selecting a skill it cannot run is reported rather than discovered
	// once an agent tries to follow it.
	RequiresModules []types.ModuleID
	// Context is the skill's entry in the generated ~/CONTEXT.md.
	Context func() *types.ContextSection
}

// firecrawl/cli holds ten skills; the selector pins the one that teaches the
// CLI itself, so an unattended install cannot pull in the other nine.
var FirecrawlSkill = &Spec{
	ID:              types.SkillFirecrawl,
	Label:           "Firecrawl (scrape/crawl/map/search via the firecrawl CLI)",
	Ref:             "firecrawl/cli",
	Skill:           "firecrawl-cli",
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
	Ref:             "anthropics/skills",
	Skill:           "webapp-testing",
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

// probabl-ai/skills (Probabl, the company behind commercial scikit-learn
// support) holds ~14 skills; the selector pins the index one, the same way
// firecrawl/cli's selector pins its own CLI skill among ten.
var DataScienceSkill = &Spec{
	ID:              types.SkillDataScience,
	Label:           "Data Science Python Stack (pandas/numpy/scikit-learn conventions)",
	Ref:             "probabl-ai/skills",
	Skill:           "data-science-python-stack",
	RequiresModules: []types.ModuleID{types.ModulePython},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Data science skill",
			Body: "Teaches the opinionated Python data-science/ML stack this image ships:\n" +
				"which library to reach for at each step (pandas/polars, numpy, scikit-learn,\n" +
				"duckdb, …) and how to keep a project organized around them.",
		}
	},
}

// remotion-dev/skills is Remotion's own repo; the selector pins its general
// best-practices skill rather than one of the ~12 narrower ones (render,
// captions, upgrade, …).
var RemotionSkill = &Spec{
	ID:              types.SkillRemotion,
	Label:           "Remotion (programmatic video with React, official best practices)",
	Ref:             "remotion-dev/skills",
	Skill:           "remotion-best-practices",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Remotion skill",
			Body: "Teaches Remotion conventions for creating and rendering video\n" +
				"programmatically with React. `chrome` and `ffmpeg` are the modules the\n" +
				"render step needs; this image already ships both.",
		}
	},
}

// n8n-io/skills is n8n's own repo; the selector pins its router meta-skill,
// which is the documented entry point that then routes to the 13 capability
// skills (workflow-lifecycle, error-handling, expressions, …).
var N8nWorkflowsSkill = &Spec{
	ID:              types.SkillN8nWorkflows,
	Label:           "n8n Workflows (official router into n8n's capability skills)",
	Ref:             "n8n-io/skills",
	Skill:           "using-n8n-skills-official",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "n8n workflows skill",
			Body: "Teaches building n8n automation workflows: this is the router skill n8n\n" +
				"ships, which then points at its own capability skills (expressions, error\n" +
				"handling, sub-workflows, …) as the task needs them.",
		}
	},
}

// All is the ordered catalogue of installable skills.
var All = []*Spec{
	FirecrawlSkill,
	AgentBrowserSkill,
	WebappTestingSkill,
	DataScienceSkill,
	RemotionSkill,
	N8nWorkflowsSkill,
}

// InstallRef is the entry the installer receives: the source, with the skill
// selector appended when the source holds more than one. It stays a single
// token because the entries travel space-separated in the environment.
func (s *Spec) InstallRef() string {
	if s.Skill == "" {
		return s.Ref
	}
	return s.Ref + types.SkillRefSeparator + s.Skill
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
