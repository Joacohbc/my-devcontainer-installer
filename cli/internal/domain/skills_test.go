package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
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

func TestSkillsConfigResolvedMode(t *testing.T) {
	if got := (types.SkillsConfig{}).ResolvedMode(); got != types.DefaultSkillMode {
		t.Errorf("expected the default mode %q, got %q", types.DefaultSkillMode, got)
	}
	if got := (types.SkillsConfig{Mode: types.SkillModeManual}).ResolvedMode(); got != types.SkillModeManual {
		t.Errorf("expected an explicit mode to win, got %q", got)
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
	if !slices.Equal(refs, []string{"firecrawl/cli"}) {
		t.Errorf("expected the deduplicated CLI reference, got %v", refs)
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

	env := domain.SkillsEnv(types.SkillsConfig{Skills: []types.SkillID{types.SkillFirecrawl}})
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, types.SkillsEnvVar+"=firecrawl/cli") {
		t.Errorf("expected the skill refs, got %v", env)
	}
	if !strings.Contains(joined, types.SkillsModeEnvVar+"="+string(types.DefaultSkillMode)) {
		t.Errorf("expected the resolved mode, got %v", env)
	}

	manual := domain.SkillsEnv(types.SkillsConfig{Mode: types.SkillModeManual, Skills: []types.SkillID{types.SkillFirecrawl}})
	if !strings.Contains(strings.Join(manual, "\n"), types.SkillsModeEnvVar+"=manual") {
		t.Errorf("expected the explicit mode, got %v", manual)
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
