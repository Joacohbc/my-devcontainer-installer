package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestConfigureReducesWizardAnswers(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeCustom),
		stepKeySubnet:    "172.45.0.0/16",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if cfg.Workspace != "testws" {
		t.Errorf("Workspace = %q, want testws", cfg.Workspace)
	}
	if cfg.Mode != types.BuildModeCustom {
		t.Errorf("Mode = %q, want local-cached", cfg.Mode)
	}
	if cfg.Compose.Subnet != "172.45.0.0/16" {
		t.Errorf("Subnet = %q, want 172.45.0.0/16", cfg.Compose.Subnet)
	}
}

func TestConfigureReducesPorts(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeCustom),
		stepKeySubnet:    "172.45.0.0/16",
		stepKeyPorts:     "8080:80, 5432:5432",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	want := []string{"8080:80", "5432:5432"}
	if len(cfg.Compose.Ports) != len(want) {
		t.Fatalf("Ports = %v, want %v", cfg.Compose.Ports, want)
	}
	for i, p := range want {
		if cfg.Compose.Ports[i] != p {
			t.Errorf("Ports[%d] = %q, want %q", i, cfg.Compose.Ports[i], p)
		}
	}
}

func TestConfigureReducesVolumes(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeCustom),
		stepKeySubnet:    "172.45.0.0/16",
		stepKeyVolumes:   "myvol:/data, ./cache:/cache",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	want := []string{"myvol:/data", "./cache:/cache"}
	if len(cfg.Compose.Volumes) != len(want) {
		t.Fatalf("Volumes = %v, want %v", cfg.Compose.Volumes, want)
	}
	for i, v := range want {
		if cfg.Compose.Volumes[i] != v {
			t.Errorf("Volumes[%d] = %q, want %q", i, cfg.Compose.Volumes[i], v)
		}
	}
}

// The wizard's yes/no steps all start at no, so a project that never chose gets
// no shared-config volume; a project that did chose keeps its answer in either
// direction when the step is left at its seeded default.
func TestConfigureKeepsBaseSharedConfig(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	answers := map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeCustom),
		stepKeySubnet:    "172.45.0.0/16",
	}
	svc := GenerateService{Report: nopReporter{}}

	nilBase := &types.DevcontainerConfig{Env: map[string]string{}}
	cfg, err := svc.Configure(nilBase, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if types.SharedConfigEnabled(cfg) {
		t.Errorf("an unset base must default to no, got %v", cfg.Compose.SharedConfig)
	}

	on := true
	optedIn := &types.DevcontainerConfig{Env: map[string]string{}, Compose: types.ComposeConfig{SharedConfig: &on}}
	cfg, err = svc.Configure(optedIn, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !types.SharedConfigEnabled(cfg) {
		t.Errorf("explicit opt-in must survive the wizard, got %v", cfg.Compose.SharedConfig)
	}

	off := false
	optedOut := &types.DevcontainerConfig{Env: map[string]string{}, Compose: types.ComposeConfig{SharedConfig: &off}}
	cfg, err = svc.Configure(optedOut, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if cfg.Compose.SharedConfig == nil || *cfg.Compose.SharedConfig {
		t.Errorf("explicit opt-out must survive the wizard, got %v", cfg.Compose.SharedConfig)
	}
}

// An explicit answer must win in either direction: opting out from an enabled
// base, and opting in from an opted-out base.
func TestConfigureHonorsSharedConfigAnswer(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	base := map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeCustom),
		stepKeySubnet:    "172.45.0.0/16",
	}
	svc := GenerateService{Report: nopReporter{}}

	// Answering false on an unset base opts out explicitly.
	answers := map[string]any{stepKeySharedConfig: false}
	for k, v := range base {
		answers[k] = v
	}
	cfg, err := svc.Configure(&types.DevcontainerConfig{Env: map[string]string{}}, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if types.SharedConfigEnabled(cfg) {
		t.Errorf("answering false must opt out, got %v", cfg.Compose.SharedConfig)
	}

	// Answering true on an opted-out base re-enables it.
	off := false
	answers = map[string]any{stepKeySharedConfig: true}
	for k, v := range base {
		answers[k] = v
	}
	optedOut := &types.DevcontainerConfig{Env: map[string]string{}, Compose: types.ComposeConfig{SharedConfig: &off}}
	cfg, err = svc.Configure(optedOut, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !types.SharedConfigEnabled(cfg) {
		t.Errorf("answering true must opt in, got %v", cfg.Compose.SharedConfig)
	}
}

// ProfileChoices backs --profile under mode=profiles, so it must only offer
// profiles with a published image, never e.g. 'scraper' (local build only).
func TestProfileChoicesOnlyListsRemoteProfiles(t *testing.T) {
	choices := ProfileChoices()

	var want []catalog.Profile
	for _, p := range catalog.All() {
		if p.Remote {
			want = append(want, p)
		}
	}
	if len(want) == 0 {
		t.Fatal("expected at least one remote-pullable built-in profile to test against")
	}
	if len(choices) != len(want) {
		t.Fatalf("got %d choices, want %d", len(choices), len(want))
	}
	for i, p := range want {
		if choices[i].Value != p.ID {
			t.Errorf("choice[%d].Value = %q, want %q", i, choices[i].Value, p.ID)
		}
	}
	for _, c := range choices {
		if c.Value == "scraper" {
			t.Error("ProfileChoices must not offer 'scraper': it has no published remote image")
		}
	}
}

// A profile saved under the pre-rename presets/ directory must still be offered
// by the wizard and still pre-select its modules.
func TestConfigureAppliesUserProfileFromLegacyPresetsDir(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	presetsDir := filepath.Join(tmp, "devcontainer-cli", "presets")
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(presetsDir, "mystack.yml"),
		[]byte("id: mystack\nmodules: [claude-code, antigravity-cli]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      modeChoiceCustomFromProfile,
		stepKeyProfile:   "mystack",
		stepKeySubnet:    "172.20.0.0/24",
	}}

	cfg, err := svc.Configure(base, tmp, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := map[string]bool{}
	for _, m := range cfg.Dockerfile.Modules {
		got[string(m.ID)] = true
	}
	for _, want := range []string{"claude-code", "antigravity-cli"} {
		if !got[want] {
			t.Errorf("expected profile module %q to be pre-selected, got %v", want, got)
		}
	}
}

// Picking a profile in the wizard must bring its custom scripts along, exactly
// like --profile does — otherwise the same profile would mean two things
// depending on how it was applied.
func TestConfigureAppliesUserProfileScripts(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	profileDir := filepath.Join(tmp, "devcontainer-cli", "profiles", "withscripts")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: withscripts\nmodules: [claude-code]\nscripts:\n  - file: setup.sh\n    when: start\n"
	if err := os.WriteFile(filepath.Join(profileDir, "profile.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "setup.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      modeChoiceCustomFromProfile,
		stepKeyProfile:   "withscripts",
		stepKeySubnet:    "172.20.0.0/24",
	}}

	cfg, err := svc.Configure(base, tmp, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Dockerfile.Scripts) != 1 {
		t.Fatalf("expected the profile's script to be carried over, got %+v", cfg.Dockerfile.Scripts)
	}
	got := cfg.Dockerfile.Scripts[0]
	if got.When != types.ScriptWhenStart {
		t.Errorf("expected when=start, got %s", got.When)
	}
	if want := filepath.Join(profileDir, "setup.sh"); got.Source != want {
		t.Errorf("expected the script to resolve against the profile dir (%q), got %q", want, got.Source)
	}
}

// The "custom, from a profile" mode choice must offer built-in profiles too,
// not just the user's own — previously the wizard's profile step only ever
// listed catalog.LoadUserProfiles, so a built-in like 'nodejs' could only be
// reached by picking modules by hand.
func TestConfigureCustomFromProfileOffersBuiltinProfile(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      modeChoiceCustomFromProfile,
		stepKeyProfile:   "nodejs",
		stepKeySubnet:    "172.20.0.0/24",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != types.BuildModeCustom {
		t.Errorf("Mode = %q, want %q (modeChoiceCustomFromProfile normalizes to custom)", cfg.Mode, types.BuildModeCustom)
	}
	got := map[string]bool{}
	for _, m := range cfg.Dockerfile.Modules {
		got[string(m.ID)] = true
	}
	for _, want := range []string{"github-cli", "nodejs", "pnpm"} {
		if !got[want] {
			t.Errorf("expected built-in 'nodejs' profile module %q to be pre-selected, got %v", want, got)
		}
	}
}

// Plain "custom" must never surface the profile-picker step, even when
// profiles (built-in or the user's own) exist — it is now opt-in via the mode
// choice, not auto-appended.
func TestConfigurePlainCustomSkipsProfileStep(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      string(types.BuildModeCustom),
		// stepKeyProfile is deliberately answered even though it must never be
		// asked under plain custom: scriptedPrompter only consumes answers for
		// steps steps() actually builds, so this proves the step was skipped.
		stepKeyProfile: "nodejs",
		stepKeySubnet:  "172.20.0.0/24",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range cfg.Dockerfile.Modules {
		if string(m.ID) == "nodejs" {
			t.Errorf("plain custom must not pre-select the profile's modules, got %v", cfg.Dockerfile.Modules)
		}
	}
}

// Not picking a profile must leave the project's existing scripts alone.
func TestConfigureKeepsExistingScriptsWithoutProfile(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{
		Env:        map[string]string{},
		Dockerfile: types.DockerfileConfig{Scripts: []types.CustomScript{{File: "kept.sh"}}},
	}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      string(types.BuildModeCustom),
		stepKeySubnet:    "172.20.0.0/24",
	}}

	cfg, err := svc.Configure(base, tmp, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Dockerfile.Scripts) != 1 || cfg.Dockerfile.Scripts[0].File != "kept.sh" {
		t.Errorf("expected the existing scripts to survive, got %+v", cfg.Dockerfile.Scripts)
	}
}

func TestSelectModules(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{}
	prompter := scriptedPrompter{answers: map[string]any{
		categoryStepKey(types.UICategoryAITools):  []string{"claude-code", "antigravity-cli"},
		categoryStepKey(types.UICategoryDevTools): []string{"chrome"},
	}}

	ids, err := svc.SelectModules(base, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	for _, want := range []string{"claude-code", "antigravity-cli", "chrome"} {
		if !got[want] {
			t.Errorf("expected selected modules to contain %q, got %v", want, ids)
		}
	}
	if len(ids) != 3 {
		t.Errorf("expected exactly 3 selected modules, got %d: %v", len(ids), ids)
	}
	// No services must leak into the result (a preset is a pure module bundle).
	for _, id := range ids {
		if catalog.GetDockerfileModule(types.ModuleID(id)) == nil {
			t.Errorf("selected id %q is not a Dockerfile module", id)
		}
	}
}

// The base module is Always-on and never appears in the category multiselects,
// but its p10kStyle option must still be offered and its answer must survive
// into the saved config (unlike an ordinary optional module's options, which
// are only asked/saved when the module itself was selected).
func TestConfigureSavesAlwaysOnModuleOptions(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace:                   "testws",
		stepKeyMode:                        string(types.BuildModeCustom),
		stepKeySubnet:                      "172.45.0.0/16",
		optionStepKey("base", "p10kStyle"): "lean",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	var got map[string]any
	for _, m := range cfg.Dockerfile.Modules {
		if string(m.ID) == "base" {
			got = m.Options
		}
	}
	if got == nil {
		t.Fatalf("expected the base module to be present with its options, got modules %v", cfg.Dockerfile.Modules)
	}
	if got["p10kStyle"] != "lean" {
		t.Errorf("p10kStyle = %v, want lean", got["p10kStyle"])
	}
}

// Env vars are declared by Dockerfile modules too (cloudflared's TUNNEL_TOKEN),
// not only by compose services, so selecting the module must raise the prompt
// and store the answer in the project's .env.
func TestConfigurePromptsModuleEnvVars(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      string(types.BuildModeCustom),
		stepKeySubnet:    "172.46.0.0/16",
		categoryStepKey(types.UICategoryDevTools): []string{string(types.ModuleCloudflared)},
		envStepKey("TUNNEL_TOKEN"):                "tok-123",
	}}

	cfg, err := svc.Configure(&types.DevcontainerConfig{Env: map[string]string{}}, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if cfg.Env["TUNNEL_TOKEN"] != "tok-123" {
		t.Errorf("Env[TUNNEL_TOKEN] = %q, want tok-123 (env = %v)", cfg.Env["TUNNEL_TOKEN"], cfg.Env)
	}

	// Leaving the token empty is the "log in from inside the container" path: the
	// module stays selected but no value is written to the .env.
	prompter.answers[envStepKey("TUNNEL_TOKEN")] = ""
	cfg, err = svc.Configure(&types.DevcontainerConfig{Env: map[string]string{}}, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if _, ok := cfg.Env["TUNNEL_TOKEN"]; ok {
		t.Errorf("an empty token must stay out of the .env, got %v", cfg.Env)
	}
	selected := false
	for _, m := range cfg.Dockerfile.Modules {
		if m.ID == types.ModuleCloudflared {
			selected = true
		}
	}
	if !selected {
		t.Errorf("cloudflared must stay selected without a token, got %v", cfg.Dockerfile.Modules)
	}
}

// The skills picker must offer a user-defined skill alongside the built-ins —
// it used to read the static catalog.AgentSkills only.
func TestSkillsStepOffersUserDefinedSkill(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := domain.SkillDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "my-skill.yml"), []byte("id: my-skill\nlabel: My Skill\nref: me/my-skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := wizardContext{base: &types.DevcontainerConfig{}}
	offered := offeredSkillIDs(ctx, skillsWanted())

	if !slices.Contains(offered, "my-skill") {
		t.Errorf("expected 'my-skill' among the choices, got %+v", offered)
	}
}

// skillPageKeyFor is the wizard step key of the page that offers a skill.
func skillPageKeyFor(t *testing.T, id types.SkillID) string {
	t.Helper()
	for _, g := range catalog.AgentSkillsByCategory() {
		for i, spec := range g.Skills {
			if spec.ID == id {
				return skillStepKey(g.ID, i/skillsPerPage)
			}
		}
	}
	t.Fatalf("skill %q is not in the catalogue", id)
	return ""
}

// skillsWanted is a State that has already answered the gate step with yes, so
// the category pages are reached.
func skillsWanted() *State {
	s := NewState()
	s.Set(stepKeySkillsEnabled, true)
	return s
}

// offeredSkillIDs is every choice the paginated category steps present.
func offeredSkillIDs(ctx wizardContext, s *State) []string {
	var out []string
	for _, step := range ctx.skillSteps(s) {
		for _, c := range step.Build(s).Choices {
			out = append(out, c.Value)
		}
	}
	return out
}

// The catalogue runs to dozens of skills, so each step asks about one category
// and at most skillsPerPage of its skills, always leading with the skip choice.
func TestSkillStepsAskOneCategoryPageAtATime(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	ctx := wizardContext{base: &types.DevcontainerConfig{}}
	state := skillsWanted()
	steps := ctx.skillSteps(state)
	if len(steps) < 2 {
		t.Fatal("expected the gate step plus at least one category page")
	}

	for _, step := range steps[1:] {
		field := step.Build(state)
		if len(field.Choices) > skillsPerPage+1 {
			t.Errorf("step %q offers %d choices, want at most %d", step.Key, len(field.Choices), skillsPerPage+1)
		}
		if field.Choices[0].Value != skipSkillCategory {
			t.Errorf("step %q leads with %q, want the skip choice", step.Key, field.Choices[0].Value)
		}
	}
}

// A category with more skills than fit on one page is asked over several, and
// skipping it drops the pages after the answer along with everything picked.
func TestSkillCategorySkipDropsTheWholeCategory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var paged catalog.SkillGroup
	for _, g := range catalog.AgentSkillsByCategory() {
		if len(g.Skills) > skillsPerPage {
			paged = g
			break
		}
	}
	if paged.ID == "" {
		t.Skip("no category is large enough to paginate")
	}

	ctx := wizardContext{base: &types.DevcontainerConfig{}}
	first, second := skillStepKey(paged.ID, 0), skillStepKey(paged.ID, 1)

	state := skillsWanted()
	state.Set(first, []string{string(paged.Skills[0].ID)})
	if !slices.ContainsFunc(ctx.skillSteps(state), func(s Step) bool { return s.Key == second }) {
		t.Errorf("expected the category's second page among %d steps", len(ctx.skillSteps(state)))
	}

	state.Set(first, []string{skipSkillCategory, string(paged.Skills[0].ID)})
	if slices.ContainsFunc(ctx.skillSteps(state), func(s Step) bool { return s.Key == second }) {
		t.Error("a skipped category must not keep asking about its remaining pages")
	}
	if ids := categorySkillIDs(state, paged); ids != nil {
		t.Errorf("a skipped category must select nothing, got %v", ids)
	}
}

// Declining the gate must stop the listing entirely — no category is asked —
// and clear what the project already carried, since "no skills" is an answer.
func TestSkillsGateDeclinedAsksNothingElse(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	base := &types.DevcontainerConfig{Skills: types.SkillsConfig{
		Mode:   types.SkillModeAuto,
		Skills: []types.SkillID{types.SkillFirecrawl},
	}}
	ctx := wizardContext{base: base}

	state := NewState()
	if enabled, _ := ctx.skillsEnabledStep().Build(state).Initial.(bool); !enabled {
		t.Error("a project that already has skills must default the gate to yes")
	}

	state.Set(stepKeySkillsEnabled, false)
	steps := ctx.skillSteps(state)
	if len(steps) != 1 || steps[0].Key != stepKeySkillsEnabled {
		t.Fatalf("expected the gate step alone, got %d steps", len(steps))
	}
	if cfg := ctx.selectedSkills(state); len(cfg.Skills) != 0 {
		t.Errorf("declining the gate must select no skills, got %+v", cfg.Skills)
	}
}

// A profile that declares skills means them the same way it means its
// modules, so picking it in the wizard must pre-check them (and pre-select
// its skills mode) rather than leave them to be re-picked by hand.
func TestConfigureProfileSkillsArePreSelected(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	profilesDir := filepath.Join(tmp, "devcontainer-cli", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: withskills\nmodules: [nodejs]\nskills: [firecrawl]\nskills_mode: auto\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "withskills.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Answer only the profile choice: every other step falls back to the
	// default the wizard seeds it with, which is exactly what this asserts.
	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      modeChoiceCustomFromProfile,
		stepKeyProfile:   "withskills",
		stepKeySubnet:    "172.20.0.0/24",
	}}

	cfg, err := svc.Configure(base, tmp, prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if len(cfg.Skills.Skills) != 1 || cfg.Skills.Skills[0] != types.SkillFirecrawl {
		t.Errorf("expected the profile's skill to be pre-selected, got %+v", cfg.Skills.Skills)
	}
	if cfg.Skills.Mode != types.SkillModeAuto {
		t.Errorf("expected the profile's skills mode to be pre-selected, got %q", cfg.Skills.Mode)
	}
}

// Seeding from a profile must not stop the user overriding it: an explicit
// answer for the skills step still wins.
func TestConfigureExplicitSkillsBeatProfile(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	profilesDir := filepath.Join(tmp, "devcontainer-cli", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: withskills\nmodules: [nodejs]\nskills: [firecrawl]\nskills_mode: auto\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "withskills.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	answers := map[string]any{
		stepKeyWorkspace:  "ws",
		stepKeyMode:       modeChoiceCustomFromProfile,
		stepKeyProfile:    "withskills",
		stepKeySubnet:     "172.20.0.0/24",
		stepKeySkillsMode: string(types.SkillModeManual),
	}
	// Clear the page the profile's own skill sits on, then pick another one —
	// they share a page when they share a category, so order matters here.
	answers[skillPageKeyFor(t, types.SkillFirecrawl)] = []string{}
	answers[skillPageKeyFor(t, types.SkillAgentBrowser)] = []string{string(types.SkillAgentBrowser)}
	prompter := scriptedPrompter{answers: answers}

	cfg, err := svc.Configure(base, tmp, prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if len(cfg.Skills.Skills) != 1 || cfg.Skills.Skills[0] != types.SkillAgentBrowser {
		t.Errorf("expected the explicit answer to win, got %+v", cfg.Skills.Skills)
	}
	if cfg.Skills.Mode != types.SkillModeManual {
		t.Errorf("expected the explicit mode to win, got %q", cfg.Skills.Mode)
	}
}

func TestServiceOptionsOf(t *testing.T) {
	services := []types.SelectedService{
		{ID: "postgres", Options: map[string]any{"version": "16"}},
	}
	if opts := ServiceOptionsOf(services, "postgres"); opts["version"] != "16" {
		t.Errorf("ServiceOptionsOf returned %v, want version=16", opts)
	}
	if opts := ServiceOptionsOf(services, "redis"); len(opts) != 0 {
		t.Errorf("ServiceOptionsOf for absent id = %v, want empty", opts)
	}
}
