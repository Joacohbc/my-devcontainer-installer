package catalog_test

import (
	"os"
	"path/filepath"
	"slices"
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
}

func TestBuiltinProfilesExcludeAITools(t *testing.T) {
	aiModules := []string{
		"claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli",
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

// A built-in ships no scripts, so it never needs a directory to resolve them
// against — which is what lets Dir stay empty for them.
func TestBuiltinProfilesHaveNoScripts(t *testing.T) {
	for _, p := range catalog.BuiltinProfiles {
		if len(p.Scripts) > 0 {
			t.Errorf("built-in profile %q must not declare scripts, got %v", p.ID, p.Scripts)
		}
	}
}

func TestResolveUnknown(t *testing.T) {
	_, ok := catalog.Resolve("non-existent-profile-id", "")
	if ok {
		t.Fatal("expected not to resolve unknown profile")
	}
}

func TestLoadUserProfilesValid(t *testing.T) {
	dir := t.TempDir()
	content := `id: myteam
label: My Team
modules: [nodejs, python]`
	err := os.WriteFile(filepath.Join(dir, "myteam.yml"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test profile file: %v", err)
	}

	profiles := catalog.LoadUserProfiles(dir)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 user profile, got %d", len(profiles))
	}
	if profiles[0].ID != "myteam" {
		t.Errorf("expected ID myteam, got %s", profiles[0].ID)
	}
	if profiles[0].Dir != dir {
		t.Errorf("expected flat profile to resolve scripts against %s, got %s", dir, profiles[0].Dir)
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
	_ = os.WriteFile(filepath.Join(dir, "broken.yml"), []byte("not: [yaml"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "good.yml"), []byte("id: good"), 0644)

	profiles := catalog.LoadUserProfiles(dir)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 user profile, got %d", len(profiles))
	}
	if profiles[0].ID != "good" {
		t.Errorf("expected ID good, got %s", profiles[0].ID)
	}
}

// Several directories are how the legacy presets/ dir keeps working: an id in
// both resolves to the one from the earlier (current) directory.
func TestLoadUserProfilesEarlierDirWins(t *testing.T) {
	current, legacy := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(current, "shared.yml"), []byte("id: shared\nmodules: [go]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "shared.yml"), []byte("id: shared\nmodules: [bun]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "onlylegacy.yml"), []byte("id: onlylegacy\nmodules: [python]"), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := catalog.LoadUserProfiles(current, legacy)
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles (deduplicated), got %d", len(profiles))
	}
	got, ok := catalog.Resolve("shared", current, legacy)
	if !ok {
		t.Fatal("expected to resolve shared")
	}
	if !slices.Equal(got.Modules, []string{"go"}) {
		t.Errorf("expected the current directory to win, got %v", got.Modules)
	}
	// A profile that only exists in the legacy directory is still found.
	if _, ok := catalog.Resolve("onlylegacy", current, legacy); !ok {
		t.Error("expected a legacy-only profile to still resolve")
	}
}

func TestUserOverridesBuiltin(t *testing.T) {
	dir := t.TempDir()
	content := `id: nodejs
modules: [bun]`
	err := os.WriteFile(filepath.Join(dir, "fs.yml"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

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
