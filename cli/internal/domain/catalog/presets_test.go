package catalog_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
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

func TestBasePreset(t *testing.T) {
	p, ok := catalog.Resolve("base", "")
	if !ok {
		t.Fatal("expected to resolve base preset")
	}
	if p.Source != "builtin" {
		t.Errorf("expected source builtin, got %s", p.Source)
	}
	if !slices.Contains(p.Modules, "github-cli") {
		t.Errorf("expected base preset to contain github-cli, got %v", p.Modules)
	}
	if !slices.Contains(p.Modules, "zellij") {
		t.Errorf("expected base preset to contain zellij, got %v", p.Modules)
	}
}

func TestBuiltinPresetsExcludeAITools(t *testing.T) {
	aiModules := []string{
		"claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli",
	}
	for _, p := range catalog.BuiltinPresets {
		for _, m := range p.Modules {
			if slices.Contains(aiModules, m) {
				t.Errorf("preset %q must not include AI module %q", p.ID, m)
			}
		}
	}
}

// pnpm requires nodejs (which pulls in nvm + Node), so it must never appear in a
// preset that doesn't also include nodejs — otherwise a Bun-only image would
// drag in the whole Node toolchain.
func TestBuiltinPresetsNoPnpmWithoutNodejs(t *testing.T) {
	for _, p := range catalog.BuiltinPresets {
		if slices.Contains(p.Modules, "pnpm") && !slices.Contains(p.Modules, "nodejs") {
			t.Errorf("preset %q includes pnpm without nodejs: %v", p.ID, p.Modules)
		}
	}
}

func TestResolveUnknown(t *testing.T) {
	_, ok := catalog.Resolve("non-existent-preset-id", "")
	if ok {
		t.Fatal("expected not to resolve unknown preset")
	}
}

func TestLoadUserPresetsValid(t *testing.T) {
	dir := t.TempDir()
	content := `id: myteam
label: My Team
modules: [nodejs, python]`
	err := os.WriteFile(filepath.Join(dir, "myteam.yml"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test preset file: %v", err)
	}

	presets := catalog.LoadUserPresets(dir)
	if len(presets) != 1 {
		t.Fatalf("expected 1 user preset, got %d", len(presets))
	}
	if presets[0].ID != "myteam" {
		t.Errorf("expected ID myteam, got %s", presets[0].ID)
	}
}

func TestLoadUserPresetsIgnoresBroken(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "broken.yml"), []byte("not: [yaml"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "good.yml"), []byte("id: good"), 0644)

	presets := catalog.LoadUserPresets(dir)
	if len(presets) != 1 {
		t.Fatalf("expected 1 user preset, got %d", len(presets))
	}
	if presets[0].ID != "good" {
		t.Errorf("expected ID good, got %s", presets[0].ID)
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
		t.Errorf("expected user preset to override builtin, got %v", got.Modules)
	}
}
