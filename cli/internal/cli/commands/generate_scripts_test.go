package commands

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// writeScript drops a script in a fake profile directory and returns its path.
func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func scriptTestConfig(scripts ...types.CustomScript) *types.DevcontainerConfig {
	return &types.DevcontainerConfig{
		Mode:       types.BuildModeCustom,
		Image:      "devcontainer-cli/test:latest",
		Workspace:  "scriptws",
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{}, Scripts: scripts},
	}
}

// A profile's script has to land in the build dir under its namespaced name —
// that copy, not the profile, is what the Dockerfile COPYs.
func TestPrepareBuildDirMaterializesCustomScripts(t *testing.T) {
	profileDir := t.TempDir()
	src := writeScript(t, profileDir, "setup.sh", "#!/bin/sh\necho hi\n")

	cwd := t.TempDir()
	config := scriptTestConfig(types.CustomScript{File: "setup.sh", Source: src})
	paths := project.ProjectPaths(cwd, config.Workspace)

	copyContents, _, err := prepareBuildDir(cwd, config, paths)
	if err != nil {
		t.Fatalf("prepareBuildDir: %v", err)
	}

	onDisk, err := os.ReadFile(filepath.Join(paths.BuildDir, "custom-setup.sh"))
	if err != nil {
		t.Fatalf("the script must be copied into the build dir: %v", err)
	}
	if string(onDisk) != "#!/bin/sh\necho hi\n" {
		t.Errorf("unexpected content copied: %q", onDisk)
	}

	// It must feed the fingerprint, or editing a script would not rebuild.
	if got, ok := copyContents["custom-setup.sh"]; !ok {
		t.Errorf("the script must be folded into copyContents, got keys %v", keysOf(copyContents))
	} else if got != string(onDisk) {
		t.Error("the fingerprinted content must be exactly what was written to disk")
	}
}

// Editing the script must change what the fingerprint sees, whatever `when` it
// runs at — a start script changes the image too, since it is COPYed into it.
func TestPrepareBuildDirCustomScriptFeedsFingerprint(t *testing.T) {
	for _, when := range []types.ScriptWhen{types.ScriptWhenBuild, types.ScriptWhenStart, types.ScriptWhenManual} {
		t.Run(string(when), func(t *testing.T) {
			profileDir := t.TempDir()
			src := writeScript(t, profileDir, "setup.sh", "#!/bin/sh\necho one\n")

			firstCwd := t.TempDir()
			first := scriptTestConfig(types.CustomScript{File: "setup.sh", When: when, Source: src})
			firstContents, _, err := prepareBuildDir(firstCwd, first, project.ProjectPaths(firstCwd, first.Workspace))
			if err != nil {
				t.Fatalf("prepareBuildDir: %v", err)
			}

			writeScript(t, profileDir, "setup.sh", "#!/bin/sh\necho two\n")
			secondCwd := t.TempDir()
			second := scriptTestConfig(types.CustomScript{File: "setup.sh", When: when, Source: src})
			secondContents, _, err := prepareBuildDir(secondCwd, second, project.ProjectPaths(secondCwd, second.Workspace))
			if err != nil {
				t.Fatalf("prepareBuildDir: %v", err)
			}

			if firstContents["custom-setup.sh"] == secondContents["custom-setup.sh"] {
				t.Error("editing the script must change the fingerprinted content")
			}
		})
	}
}

// Regenerating without the profile (no Source) must reuse the copy already in
// the build dir, so a project stays reproducible after the profile is gone.
func TestPrepareBuildDirKeepsPersistedCustomScript(t *testing.T) {
	profileDir := t.TempDir()
	src := writeScript(t, profileDir, "setup.sh", "#!/bin/sh\necho hi\n")

	cwd := t.TempDir()
	config := scriptTestConfig(types.CustomScript{File: "setup.sh", Source: src})
	paths := project.ProjectPaths(cwd, config.Workspace)
	if _, _, err := prepareBuildDir(cwd, config, paths); err != nil {
		t.Fatalf("prepareBuildDir: %v", err)
	}

	// The profile is gone and the persisted entry carries no Source, exactly as
	// it was written to devcontainer.config.json.
	if err := os.RemoveAll(profileDir); err != nil {
		t.Fatal(err)
	}
	persisted := scriptTestConfig(types.CustomScript{File: "setup.sh"})
	contents, _, err := prepareBuildDir(cwd, persisted, paths)
	if err != nil {
		t.Fatalf("prepareBuildDir after the profile is gone: %v", err)
	}
	if contents["custom-setup.sh"] != "#!/bin/sh\necho hi\n" {
		t.Errorf("expected the build-dir copy to survive, got %q", contents["custom-setup.sh"])
	}
}

// A persisted entry whose build-dir copy was deleted cannot be recovered, and
// must fail loudly instead of building an image missing the script.
func TestPrepareBuildDirFailsOnMissingCustomScript(t *testing.T) {
	cwd := t.TempDir()
	config := scriptTestConfig(types.CustomScript{File: "gone.sh"})
	paths := project.ProjectPaths(cwd, config.Workspace)

	if _, _, err := prepareBuildDir(cwd, config, paths); err == nil {
		t.Fatal("expected an error for a custom script that is nowhere to be found")
	}
}

// Remote mode builds nothing locally, so it must not try to copy scripts in.
func TestPrepareBuildDirSkipsCustomScriptsForRemote(t *testing.T) {
	cwd := t.TempDir()
	config := scriptTestConfig(types.CustomScript{File: "gone.sh"})
	config.Mode = types.BuildModeProfiles
	paths := project.ProjectPaths(cwd, config.Workspace)

	contents, _, err := prepareBuildDir(cwd, config, paths)
	if err != nil {
		t.Fatalf("remote mode must ignore custom scripts: %v", err)
	}
	if _, ok := contents["custom-gone.sh"]; ok {
		t.Error("remote mode must not fingerprint a custom script")
	}
}

// mergeCustomScripts is what keeps re-running with the same profile from
// stacking duplicates while still picking up a changed `when`.
func TestMergeCustomScripts(t *testing.T) {
	existing := []types.CustomScript{
		{File: "a.sh", When: types.ScriptWhenBuild},
		{File: "b.sh", When: types.ScriptWhenStart},
	}
	incoming := []types.CustomScript{
		{File: "a.sh", When: types.ScriptWhenManual, Source: "/tmp/a.sh"},
		{File: "c.sh"},
	}

	got := mergeCustomScripts(existing, incoming)
	if len(got) != 3 {
		t.Fatalf("expected 3 scripts, got %d: %+v", len(got), got)
	}
	if got[0].When != types.ScriptWhenManual || got[0].Source != "/tmp/a.sh" {
		t.Errorf("expected the incoming entry to replace the existing one, got %+v", got[0])
	}
	if got[1].File != "b.sh" || got[2].File != "c.sh" {
		t.Errorf("expected b.sh kept and c.sh appended, got %+v", got)
	}
}

// parseGenFlagsFor parses args through the real generate flag set.
func parseGenFlagsFor(t *testing.T, args ...string) (*genFlags, error) {
	t.Helper()
	cmd := &cobra.Command{Use: "generate"}
	addGenerateFlags(cmd)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	return parseGenFlags(cmd)
}

func TestParseGenFlags_Profile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	flags, err := parseGenFlagsFor(t, "--profile", "nodejs")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if flags.profile != "nodejs" {
		t.Errorf("expected profile nodejs, got %q", flags.profile)
	}
}

// --preset is the old spelling. It is deprecated, not removed: an existing
// script must keep working.
func TestParseGenFlags_PresetIsADeprecatedAliasOfProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	flags, err := parseGenFlagsFor(t, "--preset", "nodejs")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if flags.profile != "nodejs" {
		t.Errorf("expected --preset to resolve to the profile, got %q", flags.profile)
	}

	cmd := &cobra.Command{Use: "generate"}
	addGenerateFlags(cmd)
	if f := cmd.Flags().Lookup(flagPreset); f == nil || f.Deprecated == "" {
		t.Error("expected --preset to be marked deprecated")
	}
}

// --variant was removed (folded into --profile, no alias kept).
func TestParseGenFlags_VariantFlagDoesNotExist(t *testing.T) {
	cmd := &cobra.Command{Use: "generate"}
	addGenerateFlags(cmd)
	if cmd.Flags().Lookup("variant") != nil {
		t.Error("expected --variant to be gone, not just deprecated")
	}
}

// "ssh" is not a catalog profile — it is the hand-built full image — so it
// must not trip the generic "unknown profile" check that every other
// --profile value goes through.
func TestParseGenFlags_SSHProfileIsNotUnknown(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	flags, err := parseGenFlagsFor(t, "--profile", "ssh")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if flags.profile != "ssh" {
		t.Errorf("expected profile ssh, got %q", flags.profile)
	}
}

func TestParseGenFlags_UnknownProfileFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := parseGenFlagsFor(t, "--profile", "no-such-profile"); err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
}

// A profile's scripts are resolved at parse time so they can be persisted with
// the rest of the config.
func TestParseGenFlags_ProfileCarriesItsScripts(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	dir := filepath.Join(tmpHome, "devcontainer-cli", "profiles", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: demo\nmodules: [github-cli]\nscripts:\n  - file: setup.sh\n    when: start\n"
	if err := os.WriteFile(filepath.Join(dir, "profile.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	writeScript(t, dir, "setup.sh", "#!/bin/sh\n")

	flags, err := parseGenFlagsFor(t, "--profile", "demo")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if len(flags.scripts) != 1 {
		t.Fatalf("expected 1 script from the profile, got %d", len(flags.scripts))
	}
	if flags.scripts[0].When != types.ScriptWhenStart {
		t.Errorf("expected when=start, got %s", flags.scripts[0].When)
	}
	if want := filepath.Join(dir, "setup.sh"); flags.scripts[0].Source != want {
		t.Errorf("expected source %q, got %q", want, flags.scripts[0].Source)
	}
}

func TestParseGenFlags_Script(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	a := writeScript(t, dir, "a.sh", "#!/bin/sh\n")
	b := writeScript(t, dir, "b.sh", "#!/bin/sh\n")

	flags, err := parseGenFlagsFor(t, "--script", a, "--script", b+":manual")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if len(flags.scripts) != 2 {
		t.Fatalf("expected 2 scripts, got %d", len(flags.scripts))
	}
	if flags.scripts[0].ResolvedWhen() != types.ScriptWhenBuild {
		t.Errorf("expected the default when to be build, got %s", flags.scripts[0].ResolvedWhen())
	}
	if flags.scripts[1].When != types.ScriptWhenManual {
		t.Errorf("expected when=manual, got %s", flags.scripts[1].When)
	}
}

func TestParseGenFlags_ScriptRejectsBadSpec(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := parseGenFlagsFor(t, "--script", "/tmp/not-a-shell-script.py"); err == nil {
		t.Fatal("expected an error for a non-.sh script")
	}
}

// applyGenFlags is what puts the resolved scripts into the config that gets
// persisted, so a later regenerate no longer needs the profile.
func TestApplyGenFlags_PersistsScripts(t *testing.T) {
	config := &types.DevcontainerConfig{}
	flags := &genFlags{scripts: []types.CustomScript{{File: "setup.sh", When: types.ScriptWhenStart, Source: "/tmp/setup.sh"}}}

	applyGenFlags(config, flags)

	if len(config.Dockerfile.Scripts) != 1 {
		t.Fatalf("expected the script to be persisted, got %+v", config.Dockerfile.Scripts)
	}
	if config.Dockerfile.Scripts[0].When != types.ScriptWhenStart {
		t.Errorf("expected when=start, got %s", config.Dockerfile.Scripts[0].When)
	}
}

// A path typed with a `when` suffix must open the picker on that value, not on
// the default — otherwise the suffix looks accepted and is then silently lost.
func TestWhenOptionMatchesTheParsedWhen(t *testing.T) {
	choices := []service.Option{
		{Value: string(types.ScriptWhenBuild)},
		{Value: string(types.ScriptWhenStart)},
		{Value: string(types.ScriptWhenManual)},
	}
	if got := whenOption(choices, types.ScriptWhenStart); got.Value != string(types.ScriptWhenStart) {
		t.Errorf("expected the picker to open on start, got %q", got.Value)
	}
	// An unknown value falls back to the first choice rather than panicking.
	if got := whenOption(choices, "someday"); got.Value != string(types.ScriptWhenBuild) {
		t.Errorf("expected a fallback to build, got %q", got.Value)
	}
}

// Two paths sharing a base name would land on one file inside the profile.
func TestIndexOfScriptFindsANameCollision(t *testing.T) {
	scripts := []types.CustomScript{
		{File: "setup.sh", Source: "/a/setup.sh"},
		{File: "other.sh", Source: "/a/other.sh"},
	}
	if got := indexOfScript(scripts, types.CustomScript{File: "setup.sh", Source: "/b/setup.sh"}); got != 0 {
		t.Errorf("expected the collision at index 0, got %d", got)
	}
	if got := indexOfScript(scripts, types.CustomScript{File: "new.sh"}); got != -1 {
		t.Errorf("expected no collision, got %d", got)
	}
}

func TestParseGenFlags_Skills(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	flags, err := parseGenFlagsFor(t, "--skill", "firecrawl", "--skills-mode", "manual")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if len(flags.skills) != 1 || flags.skills[0] != types.SkillFirecrawl {
		t.Errorf("expected the firecrawl skill, got %v", flags.skills)
	}
	if flags.skillsMode != string(types.SkillModeManual) {
		t.Errorf("expected mode manual, got %q", flags.skillsMode)
	}
}

func TestParseGenFlags_RejectsUnknownSkillAndMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := parseGenFlagsFor(t, "--skill", "no-such-skill"); err == nil {
		t.Error("expected an error for an unknown skill")
	}
	if _, err := parseGenFlagsFor(t, "--skills-mode", "someday"); err == nil {
		t.Error("expected an error for an unknown skills mode")
	}
}

// The profile supplies skills, and an explicit flag overrides them — the same
// precedence --with has over the profile's modules.
func TestApplyGenFlags_SkillPrecedence(t *testing.T) {
	fromProfile := &types.DevcontainerConfig{}
	applyGenFlags(fromProfile, &genFlags{
		profileSkills:     []types.SkillID{types.SkillFirecrawl},
		profileSkillsMode: types.SkillModeManual,
	})
	if len(fromProfile.Skills.Skills) != 1 || fromProfile.Skills.Mode != types.SkillModeManual {
		t.Errorf("expected the profile's skills to apply, got %+v", fromProfile.Skills)
	}
	if !slices.ContainsFunc(fromProfile.Dockerfile.Modules, func(m types.SelectedModule) bool {
		return m.ID == types.ModuleSkills
	}) {
		t.Error("selecting skills must add the skills module")
	}

	explicit := &types.DevcontainerConfig{}
	applyGenFlags(explicit, &genFlags{
		skills:            []types.SkillID{types.SkillFirecrawl},
		skillsMode:        string(types.SkillModeAuto),
		profileSkills:     []types.SkillID{},
		profileSkillsMode: types.SkillModeManual,
	})
	if explicit.Skills.Mode != types.SkillModeAuto {
		t.Errorf("an explicit --skills-mode must win over the profile's, got %q", explicit.Skills.Mode)
	}
}

// A profile's ports apply to the project, and an explicit flag overrides them —
// the same precedence --with has over the profile's modules.
func TestApplyGenFlags_PortPrecedence(t *testing.T) {
	fromProfile := &types.DevcontainerConfig{}
	applyGenFlags(fromProfile, &genFlags{
		profilePorts:        []string{"3000"},
		profileForwardPorts: []string{"5432:postgres:5432"},
	})
	if !slices.Equal(fromProfile.Compose.Ports, []string{"3000"}) {
		t.Errorf("expected the profile's published ports, got %v", fromProfile.Compose.Ports)
	}
	if !slices.Equal(fromProfile.ForwardPorts, []string{"5432:postgres:5432"}) {
		t.Errorf("expected the profile's forward ports, got %v", fromProfile.ForwardPorts)
	}

	explicit := &types.DevcontainerConfig{}
	ports, forward := []string{"8080:80"}, []string{"9229"}
	applyGenFlags(explicit, &genFlags{
		ports:               &ports,
		forwardPorts:        &forward,
		profilePorts:        []string{"3000"},
		profileForwardPorts: []string{"5432"},
	})
	if !slices.Equal(explicit.Compose.Ports, ports) {
		t.Errorf("an explicit --ports must win, got %v", explicit.Compose.Ports)
	}
	if !slices.Equal(explicit.ForwardPorts, forward) {
		t.Errorf("an explicit --forward-ports must win, got %v", explicit.ForwardPorts)
	}

	// "none" clears them rather than falling back to the profile's.
	cleared := &types.DevcontainerConfig{}
	empty := []string{}
	applyGenFlags(cleared, &genFlags{ports: &empty, profilePorts: []string{"3000"}})
	if len(cleared.Compose.Ports) != 0 {
		t.Errorf("expected --ports none to clear them, got %v", cleared.Compose.Ports)
	}
}

func TestParseGenFlags_ForwardPorts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	flags, err := parseGenFlagsFor(t, "--forward-ports", "3000,5432:postgres:5432")
	if err != nil {
		t.Fatalf("parseGenFlags: %v", err)
	}
	if flags.forwardPorts == nil || !slices.Equal(*flags.forwardPorts, []string{"3000", "5432:postgres:5432"}) {
		t.Errorf("unexpected forward ports: %v", flags.forwardPorts)
	}

	if _, err := parseGenFlagsFor(t, "--forward-ports", "not-a-port"); err == nil {
		t.Error("expected an invalid port spec to be rejected")
	}
}

// A profile with bad ports must fail at generate time, not at compose time.
func TestParseGenFlags_ProfileWithInvalidPortsFails(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	dir := filepath.Join(tmpHome, "devcontainer-cli", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.yml"),
		[]byte("id: broken\nmodules: [github-cli]\nports: [\"nope\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := parseGenFlagsFor(t, "--profile", "broken"); err == nil {
		t.Error("expected a profile with an invalid port to be rejected")
	}
}

// A copy that silently dropped the source's ports would produce a profile that
// looks the same and behaves differently.
func TestProfileCopy_CarriesPortsAndSkills(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	dir := filepath.Join(tmpHome, "devcontainer-cli", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: src\nmodules: [github-cli]\nports: [\"3000\"]\nforward_ports: [\"5432:postgres:5432\"]\nskills: [firecrawl]\nskills_mode: auto\n"
	if err := os.WriteFile(filepath.Join(dir, "src.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "copy", "src", "dst", "--no-interactive"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error copying the profile: %v", err)
	}

	copied, ok := catalog.Resolve("dst", domain.ProfileDirs()...)
	if !ok {
		t.Fatal("expected the copy to resolve")
	}
	if !slices.Equal(copied.Ports, []string{"3000"}) {
		t.Errorf("expected the published ports to be copied, got %v", copied.Ports)
	}
	if !slices.Equal(copied.ForwardPorts, []string{"5432:postgres:5432"}) {
		t.Errorf("expected the forward ports to be copied, got %v", copied.ForwardPorts)
	}
	if !slices.Contains(copied.Skills, types.SkillFirecrawl) || copied.SkillsMode != types.SkillModeAuto {
		t.Errorf("expected the skills and mode to be copied, got %v / %q", copied.Skills, copied.SkillsMode)
	}
}
