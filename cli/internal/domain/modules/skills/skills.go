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

// ClaudeVideoSkill provides video inspection and analysis using yt-dlp & FFmpeg.
var ClaudeVideoSkill = &Spec{
	ID:              types.SkillClaudeVideo,
	Label:           "Claude Video (video analysis via yt-dlp & FFmpeg)",
	Ref:             "bradautomates/claude-video",
	RequiresModules: []types.ModuleID{types.ModuleNodejs, types.ModuleFfmpeg},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Claude Video skill",
			Body: "Teaches how to analyze video content from a URL or local file.\n" +
				"Extracts frames via FFmpeg and transcripts/captions via yt-dlp.",
		}
	},
}

// ImpeccableSkill provides design critique, UX polish, and frontend craftsmanship.
var ImpeccableSkill = &Spec{
	ID:              types.SkillImpeccable,
	Label:           "Impeccable (design and frontend polish)",
	Ref:             "pbakaus/impeccable",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Impeccable skill",
			Body: "Teaches frontend design critique, UX polish, typography, layout,\n" +
				"and modern web craft to create distinctive, production-grade interfaces.",
		}
	},
}

// WayfinderSkill provides codebase navigation, orientation, and structure analysis.
var WayfinderSkill = &Spec{
	ID:              types.SkillWayfinder,
	Label:           "Wayfinder (codebase navigation and deep orientation)",
	Ref:             "mattpocock/skills",
	Skill:           "wayfinder",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Wayfinder skill",
			Body: "Teaches codebase navigation, orientation, and architectural guidance\n" +
				"for coding agents working in complex codebases.",
		}
	},
}

// FrontendDesignSkill creates distinctive, production-grade frontend interfaces.
var FrontendDesignSkill = &Spec{
	ID:              types.SkillFrontendDesign,
	Label:           "Frontend Design (distinctive, production-grade frontend interfaces)",
	Ref:             "anthropics/skills",
	Skill:           "frontend-design",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Frontend design skill",
			Body: "Teaches building distinctive, high-quality, production-grade web\n" +
				"interfaces and frontend components.",
		}
	},
}

// UIUXProMaxSkill provides UI/UX design intelligence, color palettes, and component patterns.
var UIUXProMaxSkill = &Spec{
	ID:              types.SkillUIUXProMax,
	Label:           "UI/UX Pro Max (design intelligence, color palettes, typography, UX patterns)",
	Ref:             "nextlevelbuilder/ui-ux-pro-max-skill",
	Skill:           "ui-ux-pro-max",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "UI/UX Pro Max skill",
			Body: "Comprehensive UI/UX design intelligence database with palettes, styles,\n" +
				"fonts, UX guidelines, motion presets, and components.",
		}
	},
}

// FindSkillsSkill assists in discovering and installing agent skills on demand.
var FindSkillsSkill = &Spec{
	ID:              types.SkillFindSkills,
	Label:           "Find Skills (discover and install agent skills on demand)",
	Ref:             "vercel-labs/skills",
	Skill:           "find-skills",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Find Skills skill",
			Body: "Teaches how to discover, search, and install agent skills from the Skills\n" +
				"ecosystem on demand.",
		}
	},
}

// GraphifySkill builds queryable codebase knowledge graphs for AI agents.
var GraphifySkill = &Spec{
	ID:              types.SkillGraphify,
	Label:           "Graphify (codebase knowledge graphs for AI agents)",
	Ref:             "safishamsi/graphify",
	RequiresModules: []types.ModuleID{types.ModulePython},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Graphify skill",
			Body: "Transforms codebases, documentation, schemas and files into a\n" +
				"queryable knowledge graph for AI coding agents.",
		}
	},
}

// CavemanSkill compresses AI agent output and provides token-saving communication hooks.
var CavemanSkill = &Spec{
	ID:              types.SkillCaveman,
	Label:           "Caveman (AI agent output compression + hooks)",
	Ref:             "JuliusBrussee/caveman",
	RequiresModules: []types.ModuleID{types.ModuleNodejs},
	Context: func() *types.ContextSection {
		return &types.ContextSection{
			Title: "Caveman skill",
			Body: "Compresses AI agent output and installs token-saving communication\n" +
				"hooks and skills for AI agents in this workspace.",
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
	ClaudeVideoSkill,
	N8nWorkflowsSkill,
	ImpeccableSkill,
	WayfinderSkill,
	FrontendDesignSkill,
	UIUXProMaxSkill,
	FindSkillsSkill,
	GraphifySkill,
	CavemanSkill,
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

// Group describes a logical collection of agent skills. When a project includes
// a Group's ID, it is expanded into the group's constituent Specs.
type Group struct {
	ID    types.SkillID
	Label string
	Specs []*Spec
}

var N8nSkillGroup = &Group{
	ID:    types.SkillN8nAll,
	Label: "n8n full suite (all official and internal n8n skills)",
	Specs: []*Spec{
		{
			ID:    types.SkillID("n8n-agents-official"),
			Label: "n8n-agents-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-agents-official",
		},
		{
			ID:    types.SkillID("n8n-binary-and-data-official"),
			Label: "n8n-binary-and-data-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-binary-and-data-official",
		},
		{
			ID:    types.SkillID("n8n-code-nodes-official"),
			Label: "n8n-code-nodes-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-code-nodes-official",
		},
		{
			ID:    types.SkillID("n8n-credentials-and-security-official"),
			Label: "n8n-credentials-and-security-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-credentials-and-security-official",
		},
		{
			ID:    types.SkillID("n8n-data-tables-official"),
			Label: "n8n-data-tables-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-data-tables-official",
		},
		{
			ID:    types.SkillID("n8n-debugging-official"),
			Label: "n8n-debugging-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-debugging-official",
		},
		{
			ID:    types.SkillID("n8n-error-handling-official"),
			Label: "n8n-error-handling-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-error-handling-official",
		},
		{
			ID:    types.SkillID("n8n-expressions-official"),
			Label: "n8n-expressions-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-expressions-official",
		},
		{
			ID:    types.SkillID("n8n-extending-mcp-official"),
			Label: "n8n-extending-mcp-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-extending-mcp-official",
		},
		{
			ID:    types.SkillID("n8n-loops-official"),
			Label: "n8n-loops-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-loops-official",
		},
		{
			ID:    types.SkillID("n8n-node-configuration-official"),
			Label: "n8n-node-configuration-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-node-configuration-official",
		},
		{
			ID:    types.SkillID("n8n-subworkflows-official"),
			Label: "n8n-subworkflows-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-subworkflows-official",
		},
		{
			ID:    types.SkillID("n8n-workflow-lifecycle-official"),
			Label: "n8n-workflow-lifecycle-official",
			Ref:   "n8n-io/skills",
			Skill: "n8n-workflow-lifecycle-official",
		},
		{
			ID:    types.SkillID("using-n8n-skills-official"),
			Label: "using-n8n-skills-official",
			Ref:   "n8n-io/skills",
			Skill: "using-n8n-skills-official",
		},
	},
}

var MattPocockSkillGroup = &Group{
	ID:    types.SkillMattPocockSkills,
	Label: "Matt Pocock skills suite (all 51 developer & architecture skills)",
	Specs: []*Spec{
		{ID: types.SkillID("grill-me"), Label: "grill-me", Ref: "mattpocock/skills", Skill: "grill-me"},
		{ID: types.SkillID("grill-with-docs"), Label: "grill-with-docs", Ref: "mattpocock/skills", Skill: "grill-with-docs"},
		{ID: types.SkillID("improve-codebase-architecture"), Label: "improve-codebase-architecture", Ref: "mattpocock/skills", Skill: "improve-codebase-architecture"},
		{ID: types.SkillID("tdd"), Label: "tdd", Ref: "mattpocock/skills", Skill: "tdd"},
		{ID: types.SkillID("setup-matt-pocock-skills"), Label: "setup-matt-pocock-skills", Ref: "mattpocock/skills", Skill: "setup-matt-pocock-skills"},
		{ID: types.SkillID("handoff"), Label: "handoff", Ref: "mattpocock/skills", Skill: "handoff"},
		{ID: types.SkillID("triage"), Label: "triage", Ref: "mattpocock/skills", Skill: "triage"},
		{ID: types.SkillID("prototype"), Label: "prototype", Ref: "mattpocock/skills", Skill: "prototype"},
		{ID: types.SkillID("teach"), Label: "teach", Ref: "mattpocock/skills", Skill: "teach"},
		{ID: types.SkillID("grilling"), Label: "grilling", Ref: "mattpocock/skills", Skill: "grilling"},
		{ID: types.SkillID("domain-modeling"), Label: "domain-modeling", Ref: "mattpocock/skills", Skill: "domain-modeling"},
		{ID: types.SkillID("codebase-design"), Label: "codebase-design", Ref: "mattpocock/skills", Skill: "codebase-design"},
		{ID: types.SkillID("diagnosing-bugs"), Label: "diagnosing-bugs", Ref: "mattpocock/skills", Skill: "diagnosing-bugs"},
		{ID: types.SkillID("ask-matt"), Label: "ask-matt", Ref: "mattpocock/skills", Skill: "ask-matt"},
		{ID: types.SkillID("to-prd"), Label: "to-prd", Ref: "mattpocock/skills", Skill: "to-prd"},
		{ID: types.SkillID("implement"), Label: "implement", Ref: "mattpocock/skills", Skill: "implement"},
		{ID: types.SkillID("to-issues"), Label: "to-issues", Ref: "mattpocock/skills", Skill: "to-issues"},
		{ID: types.SkillID("code-review"), Label: "code-review", Ref: "mattpocock/skills", Skill: "code-review"},
		{ID: types.SkillID("writing-great-skills"), Label: "writing-great-skills", Ref: "mattpocock/skills", Skill: "writing-great-skills"},
		WayfinderSkill,
		{ID: types.SkillID("research"), Label: "research", Ref: "mattpocock/skills", Skill: "research"},
		{ID: types.SkillID("resolving-merge-conflicts"), Label: "resolving-merge-conflicts", Ref: "mattpocock/skills", Skill: "resolving-merge-conflicts"},
		{ID: types.SkillID("to-spec"), Label: "to-spec", Ref: "mattpocock/skills", Skill: "to-spec"},
		{ID: types.SkillID("to-tickets"), Label: "to-tickets", Ref: "mattpocock/skills", Skill: "to-tickets"},
		{ID: types.SkillID("diagnose"), Label: "diagnose", Ref: "mattpocock/skills", Skill: "diagnose"},
		{ID: types.SkillID("write-a-skill"), Label: "write-a-skill", Ref: "mattpocock/skills", Skill: "write-a-skill"},
		{ID: types.SkillID("git-guardrails-claude-code"), Label: "git-guardrails-claude-code", Ref: "mattpocock/skills", Skill: "git-guardrails-claude-code"},
		{ID: types.SkillID("zoom-out"), Label: "zoom-out", Ref: "mattpocock/skills", Skill: "zoom-out"},
		{ID: types.SkillID("mattpocock-caveman"), Label: "caveman (Matt Pocock)", Ref: "mattpocock/skills", Skill: "caveman"},
		{ID: types.SkillID("setup-pre-commit"), Label: "setup-pre-commit", Ref: "mattpocock/skills", Skill: "setup-pre-commit"},
		{ID: types.SkillID("scaffold-exercises"), Label: "scaffold-exercises", Ref: "mattpocock/skills", Skill: "scaffold-exercises"},
		{ID: types.SkillID("writing-shape"), Label: "writing-shape", Ref: "mattpocock/skills", Skill: "writing-shape"},
		{ID: types.SkillID("writing-fragments"), Label: "writing-fragments", Ref: "mattpocock/skills", Skill: "writing-fragments"},
		{ID: types.SkillID("writing-beats"), Label: "writing-beats", Ref: "mattpocock/skills", Skill: "writing-beats"},
		{ID: types.SkillID("migrate-to-shoehorn"), Label: "migrate-to-shoehorn", Ref: "mattpocock/skills", Skill: "migrate-to-shoehorn"},
		{ID: types.SkillID("design-an-interface"), Label: "design-an-interface", Ref: "mattpocock/skills", Skill: "design-an-interface"},
		{ID: types.SkillID("request-refactor-plan"), Label: "request-refactor-plan", Ref: "mattpocock/skills", Skill: "request-refactor-plan"},
		{ID: types.SkillID("qa"), Label: "qa", Ref: "mattpocock/skills", Skill: "qa"},
		{ID: types.SkillID("ubiquitous-language"), Label: "ubiquitous-language", Ref: "mattpocock/skills", Skill: "ubiquitous-language"},
		{ID: types.SkillID("obsidian-vault"), Label: "obsidian-vault", Ref: "mattpocock/skills", Skill: "obsidian-vault"},
		{ID: types.SkillID("edit-article"), Label: "edit-article", Ref: "mattpocock/skills", Skill: "edit-article"},
		{ID: types.SkillID("wizard"), Label: "wizard", Ref: "mattpocock/skills", Skill: "wizard"},
		{ID: types.SkillID("loop-me"), Label: "loop-me", Ref: "mattpocock/skills", Skill: "loop-me"},
		{ID: types.SkillID("claude-handoff"), Label: "claude-handoff", Ref: "mattpocock/skills", Skill: "claude-handoff"},
		{ID: types.SkillID("to-questionnaire"), Label: "to-questionnaire", Ref: "mattpocock/skills", Skill: "to-questionnaire"},
		{ID: types.SkillID("setup-ts-deep-modules"), Label: "setup-ts-deep-modules", Ref: "mattpocock/skills", Skill: "setup-ts-deep-modules"},
		{ID: types.SkillID("review"), Label: "review", Ref: "mattpocock/skills", Skill: "review"},
		{ID: types.SkillID("writing-for-agents"), Label: "writing-for-agents", Ref: "mattpocock/skills", Skill: "writing-for-agents"},
		{ID: types.SkillID("wait-what"), Label: "wait-what", Ref: "mattpocock/skills", Skill: "wait-what"},
		{ID: types.SkillID("batch-grill-me"), Label: "batch-grill-me", Ref: "mattpocock/skills", Skill: "batch-grill-me"},
		{ID: types.SkillID("decision-mapping"), Label: "decision-mapping", Ref: "mattpocock/skills", Skill: "decision-mapping"},
	},
}

var MattPocockAllSkillGroup = &Group{
	ID:    types.SkillMattPocockAll,
	Label: "Matt Pocock skills suite (all 51 developer & architecture skills)",
	Specs: MattPocockSkillGroup.Specs,
}

var AnthropicSkillGroup = &Group{
	ID:    types.SkillAnthropicSkills,
	Label: "Anthropic skills suite (all official Anthropic skills)",
	Specs: []*Spec{
		FrontendDesignSkill,
		{ID: types.SkillID("skill-creator"), Label: "skill-creator", Ref: "anthropics/skills", Skill: "skill-creator"},
		{ID: types.SkillID("pptx"), Label: "pptx", Ref: "anthropics/skills", Skill: "pptx"},
		{ID: types.SkillID("pdf"), Label: "pdf", Ref: "anthropics/skills", Skill: "pdf"},
		{ID: types.SkillID("docx"), Label: "docx", Ref: "anthropics/skills", Skill: "docx"},
		{ID: types.SkillID("xlsx"), Label: "xlsx", Ref: "anthropics/skills", Skill: "xlsx"},
		WebappTestingSkill,
		{ID: types.SkillID("mcp-builder"), Label: "mcp-builder", Ref: "anthropics/skills", Skill: "mcp-builder"},
		{ID: types.SkillID("canvas-design"), Label: "canvas-design", Ref: "anthropics/skills", Skill: "canvas-design"},
		{ID: types.SkillID("web-artifacts-builder"), Label: "web-artifacts-builder", Ref: "anthropics/skills", Skill: "web-artifacts-builder"},
		{ID: types.SkillID("theme-factory"), Label: "theme-factory", Ref: "anthropics/skills", Skill: "theme-factory"},
		{ID: types.SkillID("doc-coauthoring"), Label: "doc-coauthoring", Ref: "anthropics/skills", Skill: "doc-coauthoring"},
		{ID: types.SkillID("brand-guidelines"), Label: "brand-guidelines", Ref: "anthropics/skills", Skill: "brand-guidelines"},
		{ID: types.SkillID("algorithmic-art"), Label: "algorithmic-art", Ref: "anthropics/skills", Skill: "algorithmic-art"},
		{ID: types.SkillID("internal-comms"), Label: "internal-comms", Ref: "anthropics/skills", Skill: "internal-comms"},
		{ID: types.SkillID("slack-gif-creator"), Label: "slack-gif-creator", Ref: "anthropics/skills", Skill: "slack-gif-creator"},
		{ID: types.SkillID("template-skill"), Label: "template-skill", Ref: "anthropics/skills", Skill: "template-skill"},
	},
}

var AnthropicAllSkillGroup = &Group{
	ID:    types.SkillAnthropicAll,
	Label: "Anthropic skills suite (all official Anthropic skills)",
	Specs: AnthropicSkillGroup.Specs,
}

var AnthropicsSkillGroup = &Group{
	ID:    types.SkillAnthropicsSkills,
	Label: "Anthropic skills suite (all official Anthropic skills)",
	Specs: AnthropicSkillGroup.Specs,
}

var Groups = []*Group{
	N8nSkillGroup,
	MattPocockSkillGroup,
	MattPocockAllSkillGroup,
	AnthropicSkillGroup,
	AnthropicAllSkillGroup,
	AnthropicsSkillGroup,
}

func init() {
	for _, spec := range N8nSkillGroup.Specs {
		spec := spec // capture loop variable
		if spec.Context == nil {
			spec.Context = func() *types.ContextSection {
				return &types.ContextSection{
					Title: "n8n skill: " + spec.Skill,
					Body:  "An n8n automation skill (" + spec.Skill + ") installed via the n8n profile.",
				}
			}
		}
		if len(spec.RequiresModules) == 0 {
			spec.RequiresModules = []types.ModuleID{types.ModuleNodejs}
		}
	}
	for _, spec := range MattPocockSkillGroup.Specs {
		spec := spec // capture loop variable
		if spec.Context == nil {
			spec.Context = func() *types.ContextSection {
				return &types.ContextSection{
					Title: "Matt Pocock skill: " + spec.Skill,
					Body:  "A developer productivity and coding skill (" + spec.Skill + ") from mattpocock/skills.",
				}
			}
		}
		if len(spec.RequiresModules) == 0 {
			spec.RequiresModules = []types.ModuleID{types.ModuleNodejs}
		}
	}
	for _, spec := range AnthropicSkillGroup.Specs {
		spec := spec // capture loop variable
		if spec.Context == nil {
			spec.Context = func() *types.ContextSection {
				return &types.ContextSection{
					Title: "Anthropic skill: " + spec.Skill,
					Body:  "An official agent skill (" + spec.Skill + ") from anthropics/skills.",
				}
			}
		}
		if len(spec.RequiresModules) == 0 {
			spec.RequiresModules = []types.ModuleID{types.ModuleNodejs}
		}
	}
}
