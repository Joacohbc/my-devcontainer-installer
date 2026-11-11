package domain_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// skillExamplesDir holds the profiles and images the devcontainer-cli skill
// teaches from. Agents copy them verbatim, so each one is loaded through the
// same code the CLI uses and must come out valid and lint-clean.
var skillExamplesDir = filepath.Join("..", "..", "..", "skills", "devcontainer-cli", "examples")

func TestSkillExampleProfilesAreValid(t *testing.T) {
	profiles := catalog.LoadUserProfiles(filepath.Join(skillExamplesDir, "profiles"))
	if len(profiles) == 0 {
		t.Fatalf("no example profiles loaded from %s", skillExamplesDir)
	}
	covered := map[types.ScriptWhen]bool{}
	for _, p := range profiles {
		t.Run(p.ID, func(t *testing.T) {
			if p.ID != filepath.Base(p.Dir) {
				t.Errorf("profile id %q must match its directory %q, or a copy keeps the wrong id", p.ID, filepath.Base(p.Dir))
			}
			if _, err := domain.ResolveDockerfileModules(selected(p.Modules)); err != nil {
				t.Errorf("modules: %v", err)
			}
			if p.Image != "" {
				if _, ok, err := domain.ResolveImageDef(p.Image, filepath.Join(skillExamplesDir, "images")); err != nil || !ok {
					t.Errorf("image %q is not one of the example images (err %v)", p.Image, err)
				}
			}
			scripts, err := domain.ProfileScripts(p)
			if err != nil {
				t.Fatal(err)
			}
			for _, sc := range scripts {
				covered[sc.ResolvedWhen()] = true
				assertScriptClean(t, sc)
			}
		})
	}
	for _, when := range types.ScriptWhens {
		if !covered[when] {
			t.Errorf("no example profile shows a 'when: %s' script", when)
		}
	}
}

func TestSkillExampleImagesAreValid(t *testing.T) {
	dir := filepath.Join(skillExamplesDir, "images")
	defs, err := domain.LoadImageDefs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) == 0 {
		t.Fatalf("no example images loaded from %s", dir)
	}
	derived := false
	for _, def := range defs {
		t.Run(def.Name, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(dir, def.Name, domain.ImageManifestFileName)); err != nil {
				t.Errorf("image name %q must match its directory: %v", def.Name, err)
			}
			if _, err := domain.ResolveDockerfileModules(def.Modules); err != nil {
				t.Errorf("modules: %v", err)
			}
			if def.From != "" {
				derived = true
				if _, ok, err := domain.ResolveImageDef(def.From, dir); err != nil || !ok {
					t.Errorf("from %q is not one of the example images (err %v)", def.From, err)
				}
			}
			for _, sc := range def.Scripts {
				sc.Source = filepath.Join(dir, def.Name, sc.File)
				assertScriptClean(t, sc)
			}
		})
	}
	if !derived {
		t.Error("no example image shows a derived image (from:)")
	}
}

func assertScriptClean(t *testing.T, sc types.CustomScript) {
	t.Helper()
	if err := domain.ValidateCustomScript(sc); err != nil {
		t.Error(err)
		return
	}
	data, err := domain.ReadCustomScript(sc)
	if err != nil {
		t.Error(err)
		return
	}
	if warnings := domain.LintCustomScript(sc, data); len(warnings) > 0 {
		t.Errorf("lint: %s", strings.Join(warnings, "; "))
	}
	if !strings.HasPrefix(string(data), "#!/usr/bin/env bash\n") || !strings.Contains(string(data), "set -euo pipefail") {
		t.Errorf("%s must start with a bash shebang and set -euo pipefail, the pattern agents copy", sc.File)
	}
	if _, err := exec.LookPath("bash"); err == nil {
		if out, err := exec.Command("bash", "-n", sc.Source).CombinedOutput(); err != nil {
			t.Errorf("bash -n %s: %v\n%s", sc.File, err, out)
		}
	}
}

func selected(ids []string) []types.SelectedModule {
	out := make([]types.SelectedModule, len(ids))
	for i, id := range ids {
		out[i] = types.SelectedModule{ID: types.ModuleID(id)}
	}
	return out
}
