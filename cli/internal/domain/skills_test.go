package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestValidateSkills(t *testing.T) {
	cases := []struct {
		name    string
		config  types.SkillsConfig
		wantErr bool
	}{
		{"empty", types.SkillsConfig{}, false},
		{"known skill", types.SkillsConfig{Skills: []types.SkillID{types.SkillFirecrawl}}, false},
		{"auto", types.SkillsConfig{Mode: types.SkillModeAuto, Skills: []types.SkillID{types.SkillFirecrawl}}, false},
		{"manual", types.SkillsConfig{Mode: types.SkillModeManual}, false},
		{"unknown skill", types.SkillsConfig{Skills: []types.SkillID{"nope"}}, true},
		{"unknown mode", types.SkillsConfig{Mode: "someday"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := domain.ValidateSkills(c.config)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateSkills(%+v) error = %v, wantErr %v", c.config, err, c.wantErr)
			}
		})
	}
}

// The default is manual: the installer writes into the user's own repository, so
// the first write is not a decision to take for them.
func TestSkillsConfigResolvedMode(t *testing.T) {
	if types.DefaultSkillMode != types.SkillModeManual {
		t.Errorf("expected the default mode to be manual, got %q", types.DefaultSkillMode)
	}
	if got := (types.SkillsConfig{}).ResolvedMode(); got != types.SkillModeManual {
		t.Errorf("expected an unset mode to resolve to manual, got %q", got)
	}
	if got := (types.SkillsConfig{Mode: types.SkillModeAuto}).ResolvedMode(); got != types.SkillModeAuto {
		t.Errorf("expected an explicit mode to win, got %q", got)
	}
}

// Every catalogued skill must resolve to a single-token entry the installer can
// split, and describe itself.
func TestAgentSkillCatalogue(t *testing.T) {
	for _, spec := range catalog.AgentSkills {
		if spec.ID == "" || spec.Label == "" || spec.Ref == "" {
			t.Errorf("skill %+v is missing an id, label or ref", spec)
		}
		if strings.ContainsAny(spec.InstallRef(), " \t") {
			t.Errorf("skill %q has a multi-token entry %q; the entries travel space-separated in %s",
				spec.ID, spec.InstallRef(), types.SkillsEnvVar)
		}
		// The separator splits the entry, so it cannot appear inside either half.
		if strings.Contains(spec.Ref, types.SkillRefSeparator) || strings.Contains(spec.Skill, types.SkillRefSeparator) {
			t.Errorf("skill %q has %q in a ref or selector, which is the entry separator", spec.ID, types.SkillRefSeparator)
		}
		// A branch name in a ref breaks the day the repo renames its default
		// branch; the --skill selector exists so it never has to be there.
		if strings.Contains(spec.Ref, "/tree/") {
			t.Errorf("skill %q pins a branch in %q; name the skill with Skill instead", spec.ID, spec.Ref)
		}
		if spec.Context == nil || spec.Context().Body == "" {
			t.Errorf("skill %q must describe itself in CONTEXT.md", spec.ID)
		}
		for _, id := range spec.RequiresModules {
			if catalog.GetDockerfileModule(id) == nil {
				t.Errorf("skill %q requires unknown module %q", spec.ID, id)
			}
		}
	}
}

// Selecting a skill is what puts the installer in the image, and the skills
// module requiring nodejs is what guarantees the npx it runs on.
func TestApplySelectedSkillsAddsTheModule(t *testing.T) {
	config := &types.DevcontainerConfig{
		Skills: types.SkillsConfig{Skills: []types.SkillID{types.SkillFirecrawl}},
	}

	domain.ApplySelectedSkills(config)

	if !slices.ContainsFunc(config.Dockerfile.Modules, func(m types.SelectedModule) bool {
		return m.ID == types.ModuleSkills
	}) {
		t.Fatalf("expected the skills module to be added, got %+v", config.Dockerfile.Modules)
	}

	resolved, err := domain.ResolveDockerfileModules(config.Dockerfile.Modules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.ContainsFunc(resolved, func(r domain.ResolvedModule) bool {
		return r.Module.ID == types.ModuleNodejs
	}) {
		t.Error("the skills module must pull nodejs in, since the installer runs on npx")
	}
}

func TestApplySelectedSkillsIsIdempotent(t *testing.T) {
	config := &types.DevcontainerConfig{
		Skills: types.SkillsConfig{Skills: []types.SkillID{types.SkillFirecrawl}},
	}

	domain.ApplySelectedSkills(config)
	domain.ApplySelectedSkills(config)

	count := 0
	for _, m := range config.Dockerfile.Modules {
		if m.ID == types.ModuleSkills {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected the skills module exactly once, got %d", count)
	}
}

func TestApplySelectedSkillsWithoutSkillsAddsNothing(t *testing.T) {
	config := &types.DevcontainerConfig{}
	domain.ApplySelectedSkills(config)
	if len(config.Dockerfile.Modules) != 0 {
		t.Errorf("expected no modules, got %+v", config.Dockerfile.Modules)
	}
}

func TestSkillRefs(t *testing.T) {
	config := types.SkillsConfig{Skills: []types.SkillID{types.SkillFirecrawl, types.SkillFirecrawl}}
	refs := domain.SkillRefs(config)
	// firecrawl/cli holds ten skills, so the entry carries a selector for the one
	// we want: an unattended install must not take the other nine.
	if !slices.Equal(refs, []string{"firecrawl/cli#firecrawl-cli"}) {
		t.Errorf("expected the selector entry, got %v", refs)
	}

	// A source that is one skill needs no selector.
	if refs := domain.SkillRefs(types.SkillsConfig{Skills: []types.SkillID{types.SkillAgentBrowser}}); !slices.Equal(refs, []string{"vercel-labs/agent-browser"}) {
		t.Errorf("expected a bare source, got %v", refs)
	}

	// Catalogue order, not selection order.
	ordered := domain.SkillRefs(types.SkillsConfig{Skills: []types.SkillID{types.SkillWebappTesting, types.SkillFirecrawl}})
	if len(ordered) != 2 || !strings.Contains(ordered[0], "firecrawl") {
		t.Errorf("expected catalogue order, got %v", ordered)
	}
	if refs := domain.SkillRefs(types.SkillsConfig{Skills: []types.SkillID{"nope"}}); len(refs) != 0 {
		t.Errorf("an unknown id resolves to no reference, got %v", refs)
	}
}

// The refs travel as compose environment, not as image content: changing which
// skills a project wants must not change its image.
func TestSkillsEnv(t *testing.T) {
	if env := domain.SkillsEnv(types.SkillsConfig{}); env != nil {
		t.Errorf("expected no environment without skills, got %v", env)
	}

	env := domain.SkillsEnv(types.SkillsConfig{Skills: []types.SkillID{types.SkillAgentBrowser}})
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, types.SkillsEnvVar+"=vercel-labs/agent-browser") {
		t.Errorf("expected the skill refs, got %v", env)
	}
	if !strings.Contains(joined, types.SkillsModeEnvVar+"="+string(types.DefaultSkillMode)) {
		t.Errorf("expected the resolved mode, got %v", env)
	}

	auto := domain.SkillsEnv(types.SkillsConfig{Mode: types.SkillModeAuto, Skills: []types.SkillID{types.SkillFirecrawl}})
	if !strings.Contains(strings.Join(auto, "\n"), types.SkillsModeEnvVar+"=auto") {
		t.Errorf("expected the explicit mode, got %v", auto)
	}
}

func TestMissingSkillModules(t *testing.T) {
	config := &types.DevcontainerConfig{
		Skills: types.SkillsConfig{Skills: []types.SkillID{types.SkillFirecrawl}},
	}
	if missing := domain.MissingSkillModules(config); !slices.Contains(missing, types.ModuleNodejs) {
		t.Errorf("expected nodejs to be reported missing, got %v", missing)
	}

	config.Dockerfile.Modules = []types.SelectedModule{{ID: types.ModuleNodejs}}
	if missing := domain.MissingSkillModules(config); len(missing) != 0 {
		t.Errorf("expected nothing missing once nodejs is selected, got %v", missing)
	}
}
