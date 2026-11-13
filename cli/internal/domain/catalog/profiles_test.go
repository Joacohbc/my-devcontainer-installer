package catalog_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestResolveBuiltin(t *testing.T) {
	p, ok := catalog.Resolve("nodejs", "")
	if !ok {
		t.Fatal("expected to resolve nodejs")
	}
	if p.ID != "nodejs" {
		t.Errorf("expected ID nodejs, got %s", p.ID)
	}
	if p.Source != "builtin" {
		t.Errorf("expected source builtin, got %s", p.Source)
	}
	if !slices.Contains(p.Modules, "nodejs") {
		t.Errorf("expected modules to contain nodejs, got %v", p.Modules)
	}
}

func TestBaseProfile(t *testing.T) {
	p, ok := catalog.Resolve("base", "")
	if !ok {
		t.Fatal("expected to resolve base profile")
	}
	if p.Source != "builtin" {
		t.Errorf("expected source builtin, got %s", p.Source)
	}
	if !slices.Contains(p.Modules, "github-cli") {
		t.Errorf("expected base profile to contain github-cli, got %v", p.Modules)
	}
	// Zellij ships in the base image by default (always-on), so profiles must NOT
	// list it as a selectable module.
	if slices.Contains(p.Modules, "zellij") {
		t.Errorf("zellij is always-on and must not be listed in the base profile, got %v", p.Modules)
	}
	if len(p.Skills) != 0 {
		t.Errorf("base profile must have no default skills, got %v", p.Skills)
	}
}

func TestPlainBuiltinProfilesHaveNoDefaultSkills(t *testing.T) {
	plainIDs := []string{
		"base", "nodejs", "bun", "java-temurin", "python", "go",
		"node-go", "node-python", "node-java-temurin",
		"bun-go", "bun-python", "bun-java-temurin",
	}
	for _, id := range plainIDs {
		p, ok := catalog.Resolve(id, "")
		if !ok {
			t.Fatalf("expected to resolve profile %s", id)
		}
		if len(p.Skills) != 0 {
			t.Errorf("profile %q must have no default skills, got %v", id, p.Skills)
		}
	}
}

// Profile.Remote is what tells --profile's mode=profiles (pull target) role
// apart from its mode=custom (module bundle) role: it must be set on exactly
// the ids CI actually publishes (types.RemoteVariants), so a profile never
// claims a pullable image that does not exist (or hides one that does).
// 'base' is included: CI publishes devcontainer-base from the 'base' profile
// (build-base-cache job, not the build-single/build-combo matrices).
func TestBuiltinProfileRemoteFlagMatchesPublishedVariants(t *testing.T) {
	remoteVariants := map[string]bool{}
	for _, v := range types.RemoteVariants {
		remoteVariants[v] = true
	}
	for _, p := range catalog.BuiltinProfiles {
		want := remoteVariants[p.ID]
		if p.Remote != want {
			t.Errorf("profile %q: Remote = %v, want %v (types.RemoteVariants membership)", p.ID, p.Remote, want)
		}
	}
}

func TestBuiltinProfilesExcludeAITools(t *testing.T) {
	aiModules := []string{
		"claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli", "herdr",
	}
	for _, p := range catalog.BuiltinProfiles {
		for _, m := range p.Modules {
			if slices.Contains(aiModules, m) {
				t.Errorf("profile %q must not include AI module %q", p.ID, m)
			}
		}
	}
}

// pnpm requires nodejs (which pulls in nvm + Node), so it must never appear in a
// profile that doesn't also include nodejs — otherwise a Bun-only image would
// drag in the whole Node toolchain.
func TestBuiltinProfilesNoPnpmWithoutNodejs(t *testing.T) {
	for _, p := range catalog.BuiltinProfiles {
		if slices.Contains(p.Modules, "pnpm") && !slices.Contains(p.Modules, "nodejs") {
			t.Errorf("profile %q includes pnpm without nodejs: %v", p.ID, p.Modules)
		}
	}
}

func TestBuiltinProfilesHaveNoScripts(t *testing.T) {
	for _, p := range catalog.BuiltinProfiles {
		if len(p.Scripts) != 0 {
			t.Errorf("built-in profile %q must not declare scripts", p.ID)
		}
	}
}

func TestRetiredProfilesAreAbsent(t *testing.T) {
	retired := []string{"scraper", "media-editor", "n8n", "data-science", "android", "browser-tools"}
	for _, id := range retired {
		if _, ok := catalog.Resolve(id, ""); ok {
			t.Errorf("expected retired profile %q to be absent from catalog", id)
		}
	}
}

func TestResolveUnknown(t *testing.T) {
	_, ok := catalog.Resolve("non-existent-profile-id", "")
	if ok {
		t.Fatal("expected not to resolve unknown profile")
	}
}

func writeProfileDir(t *testing.T, root, id, manifest string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profile.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadUserProfilesValid(t *testing.T) {
	dir := t.TempDir()
	content := `id: myteam
label: My Team
modules: [nodejs, python]`
	profileDir := writeProfileDir(t, dir, "myteam", content)

	profiles := catalog.LoadUserProfiles(dir)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 user profile, got %d", len(profiles))
	}
	if profiles[0].ID != "myteam" {
		t.Errorf("expected ID myteam, got %s", profiles[0].ID)
	}
	if profiles[0].Dir != profileDir {
		t.Errorf("expected profile to resolve scripts against %s, got %s", profileDir, profiles[0].Dir)
	}
}

// The directory shape is what carries scripts: the manifest names them and they
// sit next to it, so Dir must point at the profile's own directory.
func TestLoadUserProfilesDirectoryShape(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "withscripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: withscripts\nmodules: [go]\nscripts:\n  - file: a.sh\n  - file: b.sh\n    when: start\n"
	if err := os.WriteFile(filepath.Join(dir, "profile.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := catalog.LoadUserProfiles(root)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 user profile, got %d", len(profiles))
	}
	p := profiles[0]
	if p.Dir != dir {
		t.Errorf("expected scripts to resolve against %s, got %s", dir, p.Dir)
	}
	if len(p.Scripts) != 2 {
		t.Fatalf("expected 2 scripts, got %d", len(p.Scripts))
	}
	// An omitted `when` is build; an explicit one is kept verbatim.
	if got := p.Scripts[0].ResolvedWhen(); got != types.ScriptWhenBuild {
		t.Errorf("expected default when=build, got %s", got)
	}
	if got := p.Scripts[1].ResolvedWhen(); got != types.ScriptWhenStart {
		t.Errorf("expected when=start, got %s", got)
	}
}

// A directory without a manifest is not a profile — it must be skipped rather
// than produce an empty entry.
func TestLoadUserProfilesIgnoresDirWithoutManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "junk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if profiles := catalog.LoadUserProfiles(root); len(profiles) != 0 {
		t.Fatalf("expected no profiles, got %d", len(profiles))
	}
}

func TestLoadUserProfilesIgnoresBroken(t *testing.T) {
	dir := t.TempDir()
	writeProfileDir(t, dir, "broken", "not: [yaml")
	writeProfileDir(t, dir, "good", "id: good")

	profiles := catalog.LoadUserProfiles(dir)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 user profile, got %d", len(profiles))
	}
	if profiles[0].ID != "good" {
		t.Errorf("expected ID good, got %s", profiles[0].ID)
	}
}

// Several directories stay supported by the variadic API: an id in both
// resolves to the one from the earlier directory.
func TestLoadUserProfilesEarlierDirWins(t *testing.T) {
	current, other := t.TempDir(), t.TempDir()
	writeProfileDir(t, current, "shared", "id: shared\nmodules: [go]")
	writeProfileDir(t, other, "shared", "id: shared\nmodules: [bun]")
	writeProfileDir(t, other, "onlyother", "id: onlyother\nmodules: [python]")

	profiles := catalog.LoadUserProfiles(current, other)
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles (deduplicated), got %d", len(profiles))
	}
	got, ok := catalog.Resolve("shared", current, other)
	if !ok {
		t.Fatal("expected to resolve shared")
	}
	if !slices.Equal(got.Modules, []string{"go"}) {
		t.Errorf("expected the earlier directory to win, got %v", got.Modules)
	}
	if _, ok := catalog.Resolve("onlyother", current, other); !ok {
		t.Error("expected an id only in the later directory to still resolve")
	}
}

func TestLoadUserProfilesIgnoresFlatFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "flat.yml"), []byte("id: flat"), 0o644); err != nil {
		t.Fatal(err)
	}
	if profiles := catalog.LoadUserProfiles(dir); len(profiles) != 0 {
		t.Fatalf("expected flat files to be ignored, got %d profiles", len(profiles))
	}
}

func TestLegacyFlatProfilesFindsFlatFiles(t *testing.T) {
	dir := t.TempDir()
	writeProfileDir(t, dir, "ok", "id: ok")
	for _, name := range []string{"a.yml", "b.yaml", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("id: x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := catalog.LegacyFlatProfiles(dir, filepath.Join(dir, "missing"))
	want := []string{filepath.Join(dir, "a.yml"), filepath.Join(dir, "b.yaml")}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestResolveComposedHintsAtLegacyFlatFile(t *testing.T) {
	dir := t.TempDir()
	flat := filepath.Join(dir, "ai-browser.yml")
	if err := os.WriteFile(flat, []byte("id: ai-browser"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := catalog.ResolveComposed([]string{"ai-browser"}, dir)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"unknown profile: ai-browser", flat, filepath.Join(dir, "ai-browser", "profile.yml")} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to contain %q, got %v", want, err)
		}
	}
	if _, err := catalog.ResolveComposed([]string{"nope"}, dir); err == nil || strings.Contains(err.Error(), "legacy") {
		t.Errorf("expected a plain unknown-profile error, got %v", err)
	}
}

func TestUserOverridesBuiltin(t *testing.T) {
	dir := t.TempDir()
	content := `id: nodejs
modules: [bun]`
	writeProfileDir(t, dir, "nodejs", content)

	got, ok := catalog.Resolve("nodejs", dir)
	if !ok {
		t.Fatal("failed to resolve nodejs")
	}
	if got.Source != "user" {
		t.Errorf("expected source to be user, got %s", got.Source)
	}
	if !slices.Equal(got.Modules, []string{"bun"}) {
		t.Errorf("expected user profile to override builtin, got %v", got.Modules)
	}
}

func TestMergeProfiles_Table(t *testing.T) {
	tests := []struct {
		name      string
		profiles  []catalog.Profile
		wantErr   bool
		errSubstr string
		assert    func(t *testing.T, merged catalog.Profile)
	}{
		{
			name:     "empty returns empty profile",
			profiles: nil,
			assert: func(t *testing.T, m catalog.Profile) {
				if m.ID != "" || len(m.Modules) != 0 {
					t.Errorf("expected empty profile, got %+v", m)
				}
			},
		},
		{
			name: "single profile returned as-is",
			profiles: []catalog.Profile{
				{
					ID:         "single",
					Modules:    []string{"nodejs", "pnpm"},
					Skills:     []types.SkillID{"agent-browser"},
					SkillsMode: types.SkillModeManual,
					Ports:      []string{"3000"},
				},
			},
			assert: func(t *testing.T, m catalog.Profile) {
				if m.ID != "single" || len(m.Modules) != 2 || m.SkillsMode != types.SkillModeManual {
					t.Errorf("unexpected merged single profile: %+v", m)
				}
			},
		},
		{
			name: "modules union preserves first-seen order and dedupes",
			profiles: []catalog.Profile{
				{ID: "p1", Modules: []string{"github-cli", "nodejs", "pnpm"}},
				{ID: "p2", Modules: []string{"github-cli", "go", "sqlite"}},
				{ID: "p3", Modules: []string{"pnpm", "python", "sqlite"}},
			},
			assert: func(t *testing.T, m catalog.Profile) {
				want := []string{"github-cli", "nodejs", "pnpm", "go", "sqlite", "python"}
				if !slices.Equal(m.Modules, want) {
					t.Errorf("modules = %v, want %v", m.Modules, want)
				}
			},
		},
		{
			name: "skills ports and forward ports union in first-seen order",
			profiles: []catalog.Profile{
				{
					ID:           "p1",
					Skills:       []types.SkillID{"firecrawl", "agent-browser"},
					Ports:        []string{"3000", "8080:80"},
					ForwardPorts: []string{"3000"},
				},
				{
					ID:           "p2",
					Skills:       []types.SkillID{"agent-browser", "webapp-testing"},
					Ports:        []string{"5432", "3000"},
					ForwardPorts: []string{"reverse:5432", "3000"},
				},
			},
			assert: func(t *testing.T, m catalog.Profile) {
				wantSkills := []types.SkillID{"firecrawl", "agent-browser", "webapp-testing"}
				if !slices.Equal(m.Skills, wantSkills) {
					t.Errorf("skills = %v, want %v", m.Skills, wantSkills)
				}
				wantPorts := []string{"3000", "8080:80", "5432"}
				if !slices.Equal(m.Ports, wantPorts) {
					t.Errorf("ports = %v, want %v", m.Ports, wantPorts)
				}
				wantFwd := []string{"3000", "reverse:5432"}
				if !slices.Equal(m.ForwardPorts, wantFwd) {
					t.Errorf("forward ports = %v, want %v", m.ForwardPorts, wantFwd)
				}
			},
		},
		{
			name: "scripts concatenated in order without collisions",
			profiles: []catalog.Profile{
				{
					ID: "p1",
					Scripts: []types.CustomScript{
						{File: "setup-a.sh", When: types.ScriptWhenBuild},
					},
				},
				{
					ID: "p2",
					Scripts: []types.CustomScript{
						{File: "start-b.sh", When: types.ScriptWhenStart},
					},
				},
			},
			assert: func(t *testing.T, m catalog.Profile) {
				if len(m.Scripts) != 2 {
					t.Fatalf("expected 2 scripts, got %d", len(m.Scripts))
				}
				if m.Scripts[0].File != "setup-a.sh" || m.Scripts[1].File != "start-b.sh" {
					t.Errorf("unexpected script order: %+v", m.Scripts)
				}
			},
		},
		{
			name: "script collision across profiles fails naming both profiles",
			profiles: []catalog.Profile{
				{
					ID: "alpha",
					Scripts: []types.CustomScript{
						{File: "install.sh"},
					},
				},
				{
					ID: "beta",
					Scripts: []types.CustomScript{
						{File: "install.sh"},
					},
				},
			},
			wantErr:   true,
			errSubstr: "alpha",
			assert: func(t *testing.T, m catalog.Profile) {
			},
		},
		{
			name: "script collision with custom prefix fails naming both profiles",
			profiles: []catalog.Profile{
				{
					ID: "prof-first",
					Scripts: []types.CustomScript{
						{File: "custom-init.sh"},
					},
				},
				{
					ID: "prof-second",
					Scripts: []types.CustomScript{
						{File: "init.sh"},
					},
				},
			},
			wantErr:   true,
			errSubstr: "prof-first",
		},
		{
			name: "conflicting explicit skills mode fails naming both profiles",
			profiles: []catalog.Profile{
				{ID: "p-manual", SkillsMode: types.SkillModeManual},
				{ID: "p-auto", SkillsMode: types.SkillModeAuto},
			},
			wantErr:   true,
			errSubstr: "p-manual",
		},
		{
			name: "inherits explicit skills mode when other profile has unset mode",
			profiles: []catalog.Profile{
				{ID: "p1", SkillsMode: ""},
				{ID: "p2", SkillsMode: types.SkillModeAuto},
				{ID: "p3", SkillsMode: ""},
			},
			assert: func(t *testing.T, m catalog.Profile) {
				if m.SkillsMode != types.SkillModeAuto {
					t.Errorf("skills mode = %q, want %q", m.SkillsMode, types.SkillModeAuto)
				}
			},
		},
		{
			name: "agreeing explicit skills mode succeeds",
			profiles: []catalog.Profile{
				{ID: "p1", SkillsMode: types.SkillModeManual},
				{ID: "p2", SkillsMode: types.SkillModeManual},
			},
			assert: func(t *testing.T, m catalog.Profile) {
				if m.SkillsMode != types.SkillModeManual {
					t.Errorf("skills mode = %q, want %q", m.SkillsMode, types.SkillModeManual)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			merged, err := catalog.MergeProfiles(tc.profiles...)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error %q does not contain expected substring %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.assert != nil {
				tc.assert(t, merged)
			}
		})
	}
}

func TestMergeProfiles_WithUserProfilesAndBuiltins(t *testing.T) {
	tempDir := t.TempDir()
	userProfileDir := filepath.Join(tempDir, "my-ai-tools")
	if err := os.MkdirAll(userProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptContent := "#!/bin/bash\necho 'hello'"
	if err := os.WriteFile(filepath.Join(userProfileDir, "setup-ai.sh"), []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}
	userManifest := `id: my-ai-tools
label: My AI Tools
modules:
  - sqlite
scripts:
  - file: setup-ai.sh
    when: build
skills:
  - agent-browser
ports:
  - "9000:9000"
`
	if err := os.WriteFile(filepath.Join(userProfileDir, "profile.yml"), []byte(userManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	composed, err := catalog.ResolveComposed([]string{"nodejs", "my-ai-tools"}, tempDir)
	if err != nil {
		t.Fatalf("ResolveComposed: %v", err)
	}

	// nodejs built-in has github-cli, nodejs, pnpm
	// my-ai-tools has sqlite
	wantModules := []string{"github-cli", "nodejs", "pnpm", "sqlite"}
	if !slices.Equal(composed.Modules, wantModules) {
		t.Errorf("modules = %v, want %v", composed.Modules, wantModules)
	}
	if len(composed.Scripts) != 1 || composed.Scripts[0].File != "setup-ai.sh" {
		t.Errorf("unexpected scripts: %+v", composed.Scripts)
	}
	if composed.Scripts[0].Source == "" {
		t.Error("expected script source to be resolved from user profile dir")
	}
	if !slices.Contains(composed.Skills, types.SkillID("agent-browser")) {
		t.Errorf("expected agent-browser skill, got %v", composed.Skills)
	}
	if !slices.Contains(composed.Ports, "9000:9000") {
		t.Errorf("expected 9000:9000 port, got %v", composed.Ports)
	}
}

func TestMergeProfilesTemplateFields(t *testing.T) {
	a := catalog.Profile{ID: "a", Services: []types.SelectedService{{ID: "postgres", Options: map[string]any{"v": 1}}}, Volumes: []string{"x:/x"}, Env: map[string]string{"A": "1"}}
	b := catalog.Profile{ID: "b", Services: []types.SelectedService{{ID: "postgres"}, {ID: "redis"}}, Volumes: []string{"x:/x", "y:/y"}, Env: map[string]string{"B": "2"}, Image: "android"}
	m, err := catalog.MergeProfiles(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Services) != 2 || m.Services[0].Options["v"] != 1 {
		t.Errorf("services = %+v (first profile's options must win)", m.Services)
	}
	if len(m.Volumes) != 2 || m.Env["A"] != "1" || m.Env["B"] != "2" || m.Image != "android" {
		t.Errorf("merged = %+v", m)
	}
}

func TestMergeProfilesTemplateConflicts(t *testing.T) {
	if _, err := catalog.MergeProfiles(catalog.Profile{ID: "a", Env: map[string]string{"K": "1"}}, catalog.Profile{ID: "b", Env: map[string]string{"K": "2"}}); err == nil {
		t.Error("conflicting env values must fail")
	}
	if _, err := catalog.MergeProfiles(catalog.Profile{ID: "a", Image: "x"}, catalog.Profile{ID: "b", Image: "y"}); err == nil {
		t.Error("two different images must fail")
	}
}

// Every published variant must be built by CI: either a build-single/build-combo matrix
// entry, or 'base', which build-base-cache publishes before the matrix runs.
func TestRemoteVariantsAreBuiltByCI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".github", "workflows", "docker-image.yml"))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	workflow := string(data)
	for _, v := range types.RemoteVariants {
		if v == "base" {
			job := workflow[strings.Index(workflow, "build-base-cache:"):]
			if !strings.Contains(job, "--profile base") || !strings.Contains(job, "devcontainer-base:latest") {
				t.Error("variant base: build-base-cache must build --profile base and publish devcontainer-base:latest")
			}
			continue
		}
		if !strings.Contains(workflow, "- name: "+v+"\n") {
			t.Errorf("variant %q missing from the build-single/build-combo matrices", v)
		}
	}
}
