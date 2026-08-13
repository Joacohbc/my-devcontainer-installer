package domain_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// writeUserSkill drops a minimal user-defined skill manifest under
// domain.SkillDir(), creating the directory if needed.
func writeUserSkill(t *testing.T, id, ref string) {
	t.Helper()
	dir := domain.SkillDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "id: " + id + "\nref: " + ref + "\n"
	if err := os.WriteFile(filepath.Join(dir, id+".yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

// caveman and graphify exist twice: a Dockerfile module installing the CLI,
// and a skill of the same id teaching an agent to drive it. The skill must
// require its module, or the project ships a document about a binary that is
// not in the image.
func TestCLIPairedSkillsRequireTheirModule(t *testing.T) {
	pairs := map[types.SkillID]types.ModuleID{
		types.SkillCaveman:  types.ModuleCaveman,
		types.SkillGraphify: types.ModuleGraphify,
	}
	for skillID, moduleID := range pairs {
		spec := catalog.GetAgentSkill(skillID)
		if spec == nil {
			t.Errorf("skill %q is not catalogued", skillID)
			continue
		}
		if !slices.Contains(spec.RequiresModules, moduleID) {
			t.Errorf("skill %q must require module %q, got %v", skillID, moduleID, spec.RequiresModules)
		}
	}
}

// The install entry the script splits on "#" must reproduce the command these
// skills were added from.
func TestCLIPairedSkillsInstallRefs(t *testing.T) {
	want := map[types.SkillID]string{
		types.SkillCaveman:  "https://github.com/juliusbrussee/caveman#caveman",
		types.SkillGraphify: "https://github.com/graphify-labs/graphify#graphify",
	}
	for id, ref := range want {
		spec := catalog.GetAgentSkill(id)
		if spec == nil {
			t.Errorf("skill %q is not catalogued", id)
			continue
		}
		if got := spec.InstallRef(); got != ref {
			t.Errorf("skill %q InstallRef = %q, want %q", id, got, ref)
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

// A user-defined skill under SkillDirs must flow through the same functions a
// built-in one does: valid, resolvable, and its ref installed.
func TestUserDefinedSkill_ValidatesAndResolves(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeUserSkill(t, "my-skill", "me/my-skill")

	config := types.SkillsConfig{Skills: []types.SkillID{"my-skill"}}
	if err := domain.ValidateSkills(config); err != nil {
		t.Fatalf("ValidateSkills: %v", err)
	}

	refs := domain.SkillRefs(config)
	if !slices.Equal(refs, []string{"me/my-skill"}) {
		t.Errorf("expected the user-defined ref, got %v", refs)
	}

	env := domain.SkillsEnv(config)
	if !strings.Contains(strings.Join(env, "\n"), types.SkillsEnvVar+"=me/my-skill") {
		t.Errorf("expected the user-defined ref in the env, got %v", env)
	}
}

// An id that names neither a built-in nor a user-defined skill is still
// rejected.
func TestValidateSkills_UnknownStaysUnknownWithUserDirPresent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeUserSkill(t, "my-skill", "me/my-skill")

	if err := domain.ValidateSkills(types.SkillsConfig{Skills: []types.SkillID{"nope"}}); err == nil {
		t.Fatal("expected an error for an id that is neither built-in nor user-defined")
	}
}

// A user-defined skill's own requires_modules is enforced exactly like a
// built-in's.
func TestMissingSkillModules_UserDefinedSkill(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := domain.SkillDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: my-skill\nref: me/my-skill\nrequires_modules: [python]\n"
	if err := os.WriteFile(filepath.Join(dir, "my-skill.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	config := &types.DevcontainerConfig{Skills: types.SkillsConfig{Skills: []types.SkillID{"my-skill"}}}
	if missing := domain.MissingSkillModules(config); !slices.Contains(missing, types.ModulePython) {
		t.Errorf("expected python to be reported missing, got %v", missing)
	}

	config.Dockerfile.Modules = []types.SelectedModule{{ID: types.ModulePython}}
	if missing := domain.MissingSkillModules(config); len(missing) != 0 {
		t.Errorf("expected nothing missing once python is selected, got %v", missing)
	}
}

func TestValidateSkillID(t *testing.T) {
	cases := []struct {
		id      string
		wantErr bool
	}{
		{"my-skill", false},
		{"my_skill_2", false},
		{"", true},
		{"bad id", true},
		{"bad/id", true},
		{"bad!id", true},
	}
	for _, c := range cases {
		err := domain.ValidateSkillID(c.id)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateSkillID(%q) error = %v, wantErr %v", c.id, err, c.wantErr)
		}
	}
}

func TestSkillDirs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dirs := domain.SkillDirs()
	if len(dirs) != 1 || dirs[0] != domain.SkillDir() {
		t.Errorf("SkillDirs() = %v, want [%s]", dirs, domain.SkillDir())
	}
	if filepath.Base(domain.SkillDir()) != domain.SkillDirName {
		t.Errorf("SkillDir() = %q, want basename %q", domain.SkillDir(), domain.SkillDirName)
	}
}

// catalog.All (profiles) also lives on GlobalConfigDir()'s "profiles" sibling,
// so SkillDir and ProfileDir must not collide.
func TestSkillDirDoesNotCollideWithProfileDir(t *testing.T) {
	if domain.SkillDir() == domain.ProfileDir() {
		t.Error("SkillDir must not equal ProfileDir")
	}
}
