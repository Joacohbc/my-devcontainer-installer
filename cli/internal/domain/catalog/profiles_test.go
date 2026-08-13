package catalog_test

import (
	"io/fs"
	"os"
	"path"
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
}

// Profile.Remote is what tells --profile's mode=profiles (pull target) role
// apart from its mode=custom (module bundle) role: it must be set on exactly
// the ids CI actually publishes (types.RemoteVariants), so a profile never
// claims a pullable image that does not exist (or hides one that does).
// 'base' is deliberately excluded — its published devcontainer-base is the
// minimal base-cache image (no github-cli), not this profile's content.
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

// A built-in that declares scripts must be one of the repo-shipped (embedded)
// ones: a Go literal has no directory to resolve a script against, so it would
// fail at generate time rather than here.
func TestBuiltinProfilesWithScriptsAreEmbedded(t *testing.T) {
	for _, p := range catalog.BuiltinProfiles {
		if len(p.Scripts) == 0 {
			continue
		}
		if !p.Embedded || p.Dir == "" {
			t.Errorf("built-in profile %q declares scripts but is not embedded (dir %q)", p.ID, p.Dir)
		}
	}
}

// Every repo-shipped profile must parse and every script it names must exist in
// the embedded tree, or the profile is dead weight nobody notices until a build.
func TestEmbeddedProfilesAreWellFormed(t *testing.T) {
	entries, err := fs.ReadDir(catalog.BuiltinProfileFS, catalog.BuiltinProfileRoot)
	if err != nil {
		t.Fatalf("the embedded profile tree must exist: %v", err)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		t.Fatal("expected at least one repo-shipped profile")
	}

	for _, id := range dirs {
		p, ok := catalog.Resolve(id, "")
		if !ok {
			t.Errorf("embedded profile %q does not resolve — did its profile.yml fail to parse?", id)
			continue
		}
		if !p.Embedded {
			t.Errorf("profile %q must be marked embedded", id)
		}
		if p.ID != id {
			t.Errorf("profile in directory %q declares id %q; they must match", id, p.ID)
		}
		if len(p.Modules) == 0 {
			t.Errorf("profile %q lists no modules", id)
		}
		for _, m := range p.Modules {
			if catalog.GetDockerfileModule(types.ModuleID(m)) == nil {
				t.Errorf("profile %q lists unknown module %q", id, m)
			}
		}
		for _, s := range p.Scripts {
			if _, err := fs.Stat(catalog.BuiltinProfileFS, path.Join(p.Dir, s.File)); err != nil {
				t.Errorf("profile %q references %q, which is not in the embedded tree: %v", id, s.File, err)
			}
		}
	}
}

// The scraper profile is the reason repo-shipped profiles exist; its shape is
// what the feature promises.
func TestScraperProfile(t *testing.T) {
	p, ok := catalog.Resolve("scraper", "")
	if !ok {
		t.Fatal("expected to resolve the scraper profile")
	}
	if p.Source != "builtin" {
		t.Errorf("expected source builtin, got %s", p.Source)
	}
	for _, want := range []string{"chrome", "python", "nodejs", "pnpm", "sqlite", "ffmpeg", "graphify"} {
		if !slices.Contains(p.Modules, want) {
			t.Errorf("expected the scraper profile to include %q, got %v", want, p.Modules)
		}
	}
	// Same cache convention as every other built-in.
	if len(p.Modules) == 0 || p.Modules[0] != "github-cli" {
		t.Errorf("github-cli must be listed first for the base layer cache, got %v", p.Modules)
	}
	byFile := map[string]types.ScriptWhen{}
	for _, s := range p.Scripts {
		byFile[s.File] = s.ResolvedWhen()
	}
	if got := byFile["install-scraper-tools.sh"]; got != types.ScriptWhenBuild {
		t.Errorf("the toolchain must be baked into the image, got when=%q", got)
	}

	// The agent skill is declared, not scripted: a script cannot install one
	// correctly, since the agent config dirs are symlinks into the shared-config
	// volume that only exists at runtime.
	for _, want := range []types.SkillID{types.SkillFirecrawl, types.SkillAgentBrowser, types.SkillWebappTesting} {
		if !slices.Contains(p.Skills, want) {
			t.Errorf("expected the %q skill, got %v", want, p.Skills)
		}
	}
	// Left unset so it follows the default, which is manual: the installer writes
	// into the user's repository.
	if p.SkillsMode != "" {
		t.Errorf("expected the scraper profile to leave the mode at its default, got %q", p.SkillsMode)
	}
}

// Installing an agent skill from a script writes into ~/.claude or ~/.agents,
// which are symlinks into the shared-config volume: at build time the volume
// shadows the write, and at any time it leaks the skill into every other
// container. The `skills` field exists so no profile has to try.
func TestEmbeddedProfilesDoNotScriptSkillInstalls(t *testing.T) {
	for _, p := range catalog.BuiltinProfiles {
		for _, s := range p.Scripts {
			if strings.Contains(s.File, "skill") {
				t.Errorf("profile %q installs skills through the script %q; declare them under `skills:` instead",
					p.ID, s.File)
			}
		}
	}
}

// Every skill a repo-shipped profile names must exist in the catalogue, and its
// mode must be one the installer understands.
func TestEmbeddedProfileSkillsAreCatalogued(t *testing.T) {
	for _, p := range catalog.BuiltinProfiles {
		if p.SkillsMode != "" && !slices.Contains(types.SkillModes, p.SkillsMode) {
			t.Errorf("profile %q has an unknown skills mode %q", p.ID, p.SkillsMode)
		}
		for _, id := range p.Skills {
			if catalog.GetAgentSkill(id) == nil {
				t.Errorf("profile %q names unknown skill %q", p.ID, id)
			}
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
