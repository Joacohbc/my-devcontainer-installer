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

func TestValidateProfileID(t *testing.T) {
	cases := []struct {
		id      string
		wantErr bool
	}{
		{"my-profile", false},
		{"my_profile", false},
		{"Profile42", false},
		{"", true},
		{"with space", true},
		{"with/slash", true},
		{"../escape", true},
	}
	for _, c := range cases {
		err := domain.ValidateProfileID(c.id)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateProfileID(%q) error = %v, wantErr %v", c.id, err, c.wantErr)
		}
	}
}

func TestValidateCustomScript(t *testing.T) {
	cases := []struct {
		name    string
		script  types.CustomScript
		wantErr bool
	}{
		{"plain", types.CustomScript{File: "setup.sh"}, false},
		{"explicit build", types.CustomScript{File: "setup.sh", When: types.ScriptWhenBuild}, false},
		{"start", types.CustomScript{File: "setup.sh", When: types.ScriptWhenStart}, false},
		{"manual", types.CustomScript{File: "setup.sh", When: types.ScriptWhenManual}, false},
		{"empty", types.CustomScript{}, true},
		{"unknown when", types.CustomScript{File: "setup.sh", When: "someday"}, true},
		{"not a shell script", types.CustomScript{File: "setup.py"}, true},
		{"path", types.CustomScript{File: "sub/setup.sh"}, true},
		{"traversal", types.CustomScript{File: "../setup.sh"}, true},
		{"windows path", types.CustomScript{File: `sub\setup.sh`}, true},
		// The name is interpolated into a single-quoted shell word in the RUN
		// that executes it, so a quote in it must never reach the Dockerfile.
		{"quote", types.CustomScript{File: "it's.sh"}, true},
		{"dollar", types.CustomScript{File: "a$(id).sh"}, true},
		{"leading dot", types.CustomScript{File: ".hidden.sh"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := domain.ValidateCustomScript(c.script)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateCustomScript(%+v) error = %v, wantErr %v", c.script, err, c.wantErr)
			}
		})
	}
}

func TestCustomScriptBuildFile(t *testing.T) {
	s := types.CustomScript{File: "setup.sh"}
	if got := s.BuildFile(); got != "custom-setup.sh" {
		t.Errorf("expected the build name to be namespaced, got %q", got)
	}
	// Applying a profile twice must not stack prefixes.
	already := types.CustomScript{File: "custom-setup.sh"}
	if got := already.BuildFile(); got != "custom-setup.sh" {
		t.Errorf("expected an already-prefixed name to be left alone, got %q", got)
	}
}

func TestParseScriptSpec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.sh")

	cases := []struct {
		name     string
		spec     string
		wantWhen types.ScriptWhen
		wantErr  bool
	}{
		{"bare path defaults to build", path, types.ScriptWhenBuild, false},
		{"explicit start", path + ":start", types.ScriptWhenStart, false},
		{"explicit manual", path + ":manual", types.ScriptWhenManual, false},
		{"explicit build", path + ":build", types.ScriptWhenBuild, false},
		// A colon that is not a known `when` belongs to the path, not to the spec.
		{"colon in name", filepath.Join(dir, "we_ird.sh") + ":nope", "", true},
		{"empty", "", "", true},
		{"not a shell script", filepath.Join(dir, "setup.py"), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := domain.ParseScriptSpec(c.spec)
			if (err != nil) != c.wantErr {
				t.Fatalf("ParseScriptSpec(%q) error = %v, wantErr %v", c.spec, err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if got.ResolvedWhen() != c.wantWhen {
				t.Errorf("expected when %s, got %s", c.wantWhen, got.ResolvedWhen())
			}
			if got.File != "setup.sh" {
				t.Errorf("expected file setup.sh, got %q", got.File)
			}
			if !filepath.IsAbs(got.Source) {
				t.Errorf("expected an absolute source path, got %q", got.Source)
			}
		})
	}
}

func TestProfileScriptsResolvesAgainstProfileDir(t *testing.T) {
	dir := t.TempDir()
	p := catalog.Profile{
		ID:      "demo",
		Dir:     dir,
		Scripts: []types.CustomScript{{File: "a.sh"}, {File: "b.sh", When: types.ScriptWhenStart}},
	}

	got, err := domain.ProfileScripts(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 scripts, got %d", len(got))
	}
	if want := filepath.Join(dir, "a.sh"); got[0].Source != want {
		t.Errorf("expected source %q, got %q", want, got[0].Source)
	}
	if got[1].When != types.ScriptWhenStart {
		t.Errorf("expected when to be preserved, got %s", got[1].When)
	}
}

// A profile that declares scripts but was loaded from nowhere cannot resolve
// them; that is a broken profile, not an empty one.
func TestProfileScriptsWithoutDirFails(t *testing.T) {
	p := catalog.Profile{ID: "demo", Scripts: []types.CustomScript{{File: "a.sh"}}}
	if _, err := domain.ProfileScripts(p); err == nil {
		t.Fatal("expected an error for a profile with scripts and no directory")
	}
}

func TestProfileScriptsRejectsInvalidEntry(t *testing.T) {
	p := catalog.Profile{ID: "demo", Dir: t.TempDir(), Scripts: []types.CustomScript{{File: "../evil.sh"}}}
	if _, err := domain.ProfileScripts(p); err == nil {
		t.Fatal("expected an error for a script that escapes the profile directory")
	}
}

func TestPartitionCustomScripts(t *testing.T) {
	config := &types.DevcontainerConfig{
		Dockerfile: types.DockerfileConfig{
			Scripts: []types.CustomScript{
				{File: "b.sh"},
				{File: "s.sh", When: types.ScriptWhenStart},
				{File: "m.sh", When: types.ScriptWhenManual},
				{File: "explicit.sh", When: types.ScriptWhenBuild},
				// A duplicate must not produce a duplicate COPY.
				{File: "b.sh"},
			},
		},
	}

	build, start, manual, err := domain.PartitionCustomScripts(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"custom-b.sh", "custom-explicit.sh"}; !slices.Equal(build, want) {
		t.Errorf("expected build %v, got %v", want, build)
	}
	if want := []string{"custom-s.sh"}; !slices.Equal(start, want) {
		t.Errorf("expected start %v, got %v", want, start)
	}
	if want := []string{"custom-m.sh"}; !slices.Equal(manual, want) {
		t.Errorf("expected manual %v, got %v", want, manual)
	}

	all, err := domain.CollectCustomScriptFiles(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("expected 4 collected scripts, got %d: %v", len(all), all)
	}
}

func TestPartitionCustomScriptsRejectsInvalid(t *testing.T) {
	config := &types.DevcontainerConfig{
		Dockerfile: types.DockerfileConfig{
			Scripts: []types.CustomScript{{File: "ok.sh", When: "eventually"}},
		},
	}
	if _, _, _, err := domain.PartitionCustomScripts(config); err == nil {
		t.Fatal("expected an error for an unknown 'when'")
	}
}

func TestProfileDirsPrefersCurrentOverLegacy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dirs := domain.ProfileDirs()
	if len(dirs) != 2 {
		t.Fatalf("expected 2 profile directories, got %v", dirs)
	}
	if !strings.HasSuffix(dirs[0], string(os.PathSeparator)+domain.ProfileDirName) {
		t.Errorf("expected the current profiles dir first, got %q", dirs[0])
	}
	if !strings.HasSuffix(dirs[1], string(os.PathSeparator)+domain.LegacyProfileDirName) {
		t.Errorf("expected the legacy presets dir second, got %q", dirs[1])
	}
	if dirs[0] != domain.ProfileDir() {
		t.Errorf("expected ProfileDir() to be the write target %q, got %q", dirs[0], domain.ProfileDir())
	}
}

// `x.sh` and `custom-x.sh` both stage as `custom-x.sh`. Silently keeping one
// would build an image missing the other, so it is an error, not a dedup.
func TestPartitionCustomScriptsRejectsCollidingBuildNames(t *testing.T) {
	config := &types.DevcontainerConfig{
		Dockerfile: types.DockerfileConfig{
			Scripts: []types.CustomScript{
				{File: "setup.sh"},
				{File: "custom-setup.sh"},
			},
		},
	}
	_, _, _, err := domain.PartitionCustomScripts(config)
	if err == nil {
		t.Fatal("expected an error for two scripts staged under the same name")
	}
	if !strings.Contains(err.Error(), "collide") {
		t.Errorf("expected the error to name the collision, got %v", err)
	}
}

// Listing the very same file twice is harmless — it is one script.
func TestPartitionCustomScriptsAllowsExactDuplicates(t *testing.T) {
	config := &types.DevcontainerConfig{
		Dockerfile: types.DockerfileConfig{
			Scripts: []types.CustomScript{{File: "setup.sh"}, {File: "setup.sh"}},
		},
	}
	build, _, _, err := domain.PartitionCustomScripts(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(build) != 1 {
		t.Errorf("expected the duplicate to collapse to one entry, got %v", build)
	}
}

// A repo-shipped profile resolves its scripts against the embedded tree, not the
// host filesystem — nothing of it exists on disk.
func TestProfileScriptsForEmbeddedProfile(t *testing.T) {
	p, ok := catalog.Resolve("scraper", "")
	if !ok {
		t.Fatal("expected to resolve the scraper profile")
	}

	scripts, err := domain.ProfileScripts(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scripts) == 0 {
		t.Fatal("expected the profile to carry scripts")
	}
	for _, s := range scripts {
		if !s.Embedded {
			t.Errorf("a built-in profile's script must be marked embedded: %+v", s)
		}
		if want := "profiles/scraper/" + s.File; s.Source != want {
			t.Errorf("expected a slash-separated embedded path %q, got %q", want, s.Source)
		}
		data, err := domain.ReadCustomScript(s)
		if err != nil {
			t.Fatalf("the embedded script %q must be readable: %v", s.File, err)
		}
		if !strings.HasPrefix(string(data), "#!") {
			t.Errorf("expected %q to start with a shebang", s.File)
		}
	}
}

func TestReadCustomScript(t *testing.T) {
	dir := t.TempDir()
	host := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(host, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := domain.ReadCustomScript(types.CustomScript{File: "setup.sh", Source: host}); err != nil {
		t.Errorf("a host script must be readable: %v", err)
	}
	if _, err := domain.ReadCustomScript(types.CustomScript{File: "setup.sh"}); err == nil {
		t.Error("expected an error for a script with no source")
	}
	if _, err := domain.ReadCustomScript(types.CustomScript{File: "nope.sh", Source: "profiles/nope/nope.sh", Embedded: true}); err == nil {
		t.Error("expected an error for a missing embedded script")
	}
}
