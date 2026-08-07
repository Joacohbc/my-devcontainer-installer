package service

import (
	"archive/zip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
)

func TestEnsureSharedConfigVolume_CreatesWhenMissing(t *testing.T) {
	// status != 0 simulates `volume inspect` failing (volume absent), so a
	// `volume create` carrying both labels and the name must follow.
	r := &fakeRunner{status: 1}
	restore := useFakeDocker(r)
	defer restore()

	if err := EnsureSharedConfigVolume(nopReporter{}); err != nil {
		t.Fatalf("EnsureSharedConfigVolume failed: %v", err)
	}
	create := r.callContaining("create")
	if create == nil {
		t.Fatalf("expected a `volume create` call, got %v", r.calls)
	}
	for _, want := range []string{
		types.SharedConfigVolumeName,
		types.LabelManaged + "=true",
		types.LabelSharedConfig + "=true",
	} {
		if !sliceHas(create, want) {
			t.Errorf("create call missing %q: %v", want, create)
		}
	}
}

func TestEnsureSharedConfigVolume_SkipsWhenPresent(t *testing.T) {
	// status 0 means `volume inspect` succeeds (volume already exists); no create.
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	if err := EnsureSharedConfigVolume(nopReporter{}); err != nil {
		t.Fatalf("EnsureSharedConfigVolume failed: %v", err)
	}
	if create := r.callContaining("create"); create != nil {
		t.Errorf("did not expect a create call when the volume exists, got %v", create)
	}
}

// SyncAliases writes the rendered script into the volume's alias.sh entry via a
// throwaway helper, always overwriting and re-owning to the host uid/gid. It
// never reads a host file.
func TestSyncAliases_WritesRenderedContentIntoVolume(t *testing.T) {
	// status 0: `volume inspect` succeeds (no create), and the writer `run`
	// exits cleanly.
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	content := "#!/bin/sh\nalias ll='ls -la'\n"
	if err := (SharedConfigService{Report: nopReporter{}}).SyncAliases(content); err != nil {
		t.Fatalf("SyncAliases: %v", err)
	}

	run := r.callContaining("--rm")
	if run == nil {
		t.Fatalf("expected a helper `run` call, got %v", r.calls)
	}
	for _, want := range []string{
		types.SharedConfigVolumeName + ":/vol",
		"ENTRY_ID=" + types.SharedConfigAliasID,
		"ENTRY_CONTENT=" + content,
		"ENTRY_OWNER=" + hostOwnerString(),
	} {
		if !sliceHas(run, want) {
			t.Errorf("helper run missing %q: %v", want, run)
		}
	}
	// It must never mount a host home read-only — the content comes from config.
	for _, arg := range run {
		if strings.HasSuffix(arg, ":/host:ro") {
			t.Errorf("SyncAliases must not read a host file, saw %q", arg)
		}
	}
}

// A non-zero helper exit must surface as an error rather than silently claiming
// success.
func TestSyncAliases_ReportsHelperFailure(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()
	// Volume inspect (status 0) succeeds; make the writer run fail by flipping
	// status after the volume check is not possible with this fake, so instead
	// use a runner that fails everything except version and inspect. Simpler:
	// status 1 makes inspect fail (triggering create, which also "fails" but is
	// only warned) and then the writer run fails too.
	r.status = 1
	err := (SharedConfigService{Report: nopReporter{}}).SyncAliases("x")
	if err == nil {
		t.Error("SyncAliases must return an error when the helper exits non-zero")
	}
}

// TestSharedConfigEntriesMatchEntrypoint is the contract test keeping the Go
// registry (types.SharedConfigEntries) in sync with the shell table baked into
// entrypoint.sh. Every entry must appear as "<id> <kind> <target>".
func TestSharedConfigEntriesMatchEntrypoint(t *testing.T) {
	data, err := assets.Content("entrypoint.sh")
	if err != nil {
		t.Fatalf("could not read entrypoint.sh: %v", err)
	}
	script := string(data)
	for _, e := range types.SharedConfigEntries {
		row := fmt.Sprintf("%s %s %s", e.ID, e.Kind, e.Target)
		if !strings.Contains(script, row) {
			t.Errorf("entrypoint.sh missing shared-config row %q", row)
		}
	}
}

func sliceHas(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func TestSyncFromHostCopiesViaHelper(t *testing.T) {
	// The helper prints COPIED on stdout, so the entry is classified as copied.
	r := &fakeRunner{status: 0, stdout: "COPIED"}
	defer useFakeDocker(r)()

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := SharedConfigService{Report: nopReporter{}}
	entry, _ := types.SharedConfigEntryByID("claude")
	res, err := svc.SyncFromHost([]types.SharedConfigEntry{entry}, home, false)
	if err != nil {
		t.Fatalf("SyncFromHost: %v", err)
	}
	if len(res.Copied) != 1 || res.Copied[0] != "claude" {
		t.Fatalf("Copied = %v, want [claude]", res.Copied)
	}

	run := r.callContaining("run")
	if run == nil {
		t.Fatalf("expected a docker run call, got %v", r.calls)
	}
	for _, want := range []string{
		types.SharedConfigVolumeName + ":/vol",
		home + ":/host:ro",
		"ENTRY_ID=claude",
		syncHelperImage,
	} {
		if !sliceHas(run, want) {
			t.Errorf("run call missing %q: %v", want, run)
		}
	}
}

func TestSyncFromHostSkipsNonEmptyWithoutForce(t *testing.T) {
	// The helper prints SKIPPED when the volume entry already has data.
	r := &fakeRunner{status: 0, stdout: "SKIPPED"}
	defer useFakeDocker(r)()

	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := SharedConfigService{Report: nopReporter{}}
	entry, _ := types.SharedConfigEntryByID("claude.json")
	res, err := svc.SyncFromHost([]types.SharedConfigEntry{entry}, home, false)
	if err != nil {
		t.Fatalf("SyncFromHost: %v", err)
	}
	if len(res.Skipped) != 1 || len(res.Copied) != 0 {
		t.Fatalf("expected one skipped entry, got %+v", res)
	}
}

func TestSyncFromHostReportsMissingHostConfig(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "COPIED"}
	defer useFakeDocker(r)()

	svc := SharedConfigService{Report: nopReporter{}}
	entry, _ := types.SharedConfigEntryByID("codex")
	res, err := svc.SyncFromHost([]types.SharedConfigEntry{entry}, t.TempDir(), false)
	if err != nil {
		t.Fatalf("SyncFromHost: %v", err)
	}
	if len(res.Missing) != 1 || res.Missing[0] != "codex" {
		t.Fatalf("Missing = %v, want [codex]", res.Missing)
	}
	if run := r.callContaining("run"); run != nil {
		t.Errorf("must not run the helper for an entry absent on the host: %v", run)
	}
}

func TestSyncFromHostForcePassesFlagToHelper(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "COPIED"}
	defer useFakeDocker(r)()

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "gh"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := SharedConfigService{Report: nopReporter{}}
	entry, _ := types.SharedConfigEntryByID("gh")
	if _, err := svc.SyncFromHost([]types.SharedConfigEntry{entry}, home, true); err != nil {
		t.Fatalf("SyncFromHost: %v", err)
	}
	run := r.callContaining("run")
	if run == nil || !sliceHas(run, "ENTRY_FORCE=1") {
		t.Errorf("expected ENTRY_FORCE=1 in run call, got %v", run)
	}
}

// TestSyncEntryScriptRunsTheSymlinkPasses guards against a regression to a
// plain `cp -a` for directory entries: global skills/agents installed via the
// skills.sh CLI are symlinked into each tool's config dir (e.g.
// ~/.claude/skills/<name> -> ~/.agents/skills/<name>), so the copy into the
// volume must resolve those links against the host instead of carrying them
// over blind. The behavior itself is exercised against the real shell in
// TestSyncEntryScriptFixesSymlinks.
func TestSyncEntryScriptRunsTheSymlinkPasses(t *testing.T) {
	for _, want := range []string{"ensure_volume_aliases", `fix_symlinks "$dst" "$src"`} {
		if !strings.Contains(syncEntryScript, want) {
			t.Errorf("syncEntryScript must call %s, got: %s", want, syncEntryScript)
		}
	}
	// The old pass resolved links in the destination, where a cross-entry
	// target can never exist — the exact bug that left them dangling.
	if strings.Contains(syncEntryScript, `deref_symlinks "$dst"`) {
		t.Error("syncEntryScript must not resolve symlinks against the copy in the volume")
	}
}

// lookupSharedConfigEntry is a fatal-on-miss lookup for the script tests.
func lookupSharedConfigEntry(t *testing.T, id string) types.SharedConfigEntry {
	t.Helper()
	e, ok := types.SharedConfigEntryByID(id)
	if !ok {
		t.Fatalf("no shared-config entry %q", id)
	}
	return e
}

// runSyncEntryScript executes the real syncEntryScript against local stand-ins
// for the helper container's /vol and /host mounts, so the shell logic is
// covered without Docker.
func runSyncEntryScript(t *testing.T, volDir, hostDir string, e types.SharedConfigEntry) {
	t.Helper()
	script := strings.ReplaceAll(syncEntryScript, "/vol", volDir)
	script = strings.ReplaceAll(script, "/host", hostDir)

	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(),
		"ENTRY_ID="+e.ID,
		"ENTRY_TARGET="+e.Target,
		"ENTRY_KIND="+string(e.Kind),
		"ENTRY_FORCE=1",
		"ENTRY_OWNER=",
		"ENTRY_HOST_HOME="+hostDir,
		"ENTRY_DEV_HOME="+types.DevUserHome,
		"ENTRY_SHARED_PAIRS="+renderSharedEntryPairs(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("syncEntryScript(%s): %v\n%s", e.ID, err, out)
	}
	if !strings.Contains(string(out), "COPIED") {
		t.Fatalf("syncEntryScript(%s) did not report COPIED: %s", e.ID, out)
	}
}

// createPhysicalTempDir is t.TempDir() with symlinks resolved: the script
// compares resolved paths against the /host prefix, and a /tmp that is itself a
// symlink would make every comparison miss.
func createPhysicalTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestSyncEntryScriptFixesSymlinks runs the real sync helper script over a host
// home laid out the way the skills.sh CLI leaves one, and pins every outcome a
// symlink can have. The regression it guards: the pass used to resolve links
// against the COPY in the volume, where a cross-entry target such as
// ../../.agents/skills/<name> can never exist (the volume is flat, ~/.claude is
// itself a link into it), so those links were carried over verbatim and landed
// dangling in the volume — and therefore in every container.
func TestSyncEntryScriptFixesSymlinks(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	vol := createPhysicalTempDir(t)
	host := createPhysicalTempDir(t)

	createHostDir := func(parts ...string) string {
		p := filepath.Join(append([]string{host}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	createHostSymlink := func(target string, parts ...string) {
		if err := os.Symlink(target, filepath.Join(append([]string{host}, parts...)...)); err != nil {
			t.Fatal(err)
		}
	}

	// The canonical global skills store, plus a config dir that links into it
	// both ways round (relative, as skills.sh writes them, and absolute).
	skillDir := createHostDir(".agents", "skills", "wayfinder")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	createHostDir(".claude", "skills")
	createHostSymlink("../../.agents/skills/wayfinder", ".claude", "skills", "relative-shared")
	createHostSymlink(filepath.Join(host, ".agents", "skills", "wayfinder"), ".claude", "skills", "absolute-shared")

	// A link that stays inside this entry, one pointing at content that is NOT
	// a shared entry (only reachable by copying it), and a dangling one.
	if err := os.WriteFile(filepath.Join(host, ".claude", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	createHostSymlink("../settings.json", ".claude", "skills", "internal")
	nonSharedDir := createHostDir("projects", "tool")
	if err := os.WriteFile(filepath.Join(nonSharedDir, "data.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	createHostSymlink("../projects/tool", ".claude", "outside")
	createHostDir(".claude", "debug")
	createHostSymlink("session-does-not-exist", ".claude", "debug", "latest")

	runSyncEntryScript(t, vol, host, lookupSharedConfigEntry(t, "agents"))
	runSyncEntryScript(t, vol, host, lookupSharedConfigEntry(t, "claude"))

	// The home-shaped aliases must exist for every entry, so a relative
	// cross-entry link has a name to land on inside the flat volume.
	for _, e := range types.SharedConfigEntries {
		alias := filepath.Join(vol, filepath.FromSlash(e.Target))
		info, err := os.Lstat(alias)
		if err != nil {
			t.Errorf("volume alias %s missing: %v", e.Target, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("volume alias %s must be a symlink", e.Target)
		}
	}

	skills := filepath.Join(vol, "claude", "skills")

	// A relative link into another shared entry stays a link — both sides must
	// remain one store — and now resolves inside the volume.
	assertSymlinkTarget(t, filepath.Join(skills, "relative-shared"), "../../.agents/skills/wayfinder")
	if _, err := os.Stat(filepath.Join(skills, "relative-shared", "SKILL.md")); err != nil {
		t.Errorf("relative cross-entry link does not resolve inside the volume: %v", err)
	}

	// An absolute host path cannot survive as-is, so it is repointed at the
	// home the volume is symlinked into.
	assertSymlinkTarget(t, filepath.Join(skills, "absolute-shared"), types.DevUserHome+"/.agents/skills/wayfinder")

	// A link that never left the entry is untouched (it already resolves).
	assertSymlinkTarget(t, filepath.Join(skills, "internal"), "../settings.json")

	// Content outside the shared entries only survives as a real copy.
	copiedDir := filepath.Join(vol, "claude", "outside")
	copiedInfo, err := os.Lstat(copiedDir)
	if err != nil {
		t.Fatalf("outside: %v", err)
	}
	if copiedInfo.Mode()&os.ModeSymlink != 0 {
		t.Error("a link to non-shared host content must be replaced with a real copy")
	}
	if data, err := os.ReadFile(filepath.Join(copiedDir, "data.txt")); err != nil || string(data) != "outside" {
		t.Errorf("copied content = %q, %v; want %q", data, err, "outside")
	}

	// A link dangling on the host is left alone, not treated as an error: it is
	// no worse in the volume than it already was, and `cp -aL` used to abort the
	// whole sync over it (Claude Code leaves ~/.claude/debug/latest behind).
	danglingLink := filepath.Join(vol, "claude", "debug", "latest")
	danglingInfo, err := os.Lstat(danglingLink)
	if err != nil {
		t.Fatalf("dangling link should still exist: %v", err)
	}
	if danglingInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("a link already dangling on the host must stay a symlink")
	}
}

func assertSymlinkTarget(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Errorf("%s should still be a symlink: %v", filepath.Base(path), err)
		return
	}
	if got != want {
		t.Errorf("%s -> %q, want %q", filepath.Base(path), got, want)
	}
}

// TestRenderedSharedEntryPairsCoverEveryEntry: the helper needs the whole catalog, not
// just the entries being synced — a link inside one entry routinely points into
// another, and the volume aliases must exist for all of them.
func TestRenderedSharedEntryPairsCoverEveryEntry(t *testing.T) {
	pairs := renderSharedEntryPairs()
	for _, e := range types.SharedConfigEntries {
		if !strings.Contains(pairs, e.ID+"="+e.Target) {
			t.Errorf("renderSharedEntryPairs() missing %s=%s, got %q", e.ID, e.Target, pairs)
		}
	}
}

func TestBackupToZip_ErrorsWhenVolumeMissing(t *testing.T) {
	// status != 0 simulates `volume inspect` failing (volume absent).
	r := &fakeRunner{status: 1}
	defer useFakeDocker(r)()

	svc := SharedConfigService{Report: nopReporter{}}
	entry, _ := types.SharedConfigEntryByID("claude")
	_, err := svc.BackupToZip([]types.SharedConfigEntry{entry}, filepath.Join(t.TempDir(), "backup.zip"))
	if err == nil {
		t.Fatal("expected an error when the shared-config volume does not exist")
	}
	if run := r.callContaining("run"); run != nil {
		t.Errorf("must not run the helper when the volume is missing: %v", run)
	}
}

func TestBackupToZip_RunsHelperWithExpectedArgs(t *testing.T) {
	// status 0 means the volume exists and the helper "succeeds", but
	// fakeRunner does no real file I/O, so the staging dir stays empty and
	// BackupToZip errors with "no data found" after the run call happens —
	// exactly what we want to inspect here.
	r := &fakeRunner{status: 0}
	defer useFakeDocker(r)()

	svc := SharedConfigService{Report: nopReporter{}}
	claude, _ := types.SharedConfigEntryByID("claude")
	gh, _ := types.SharedConfigEntryByID("gh")
	_, err := svc.BackupToZip([]types.SharedConfigEntry{claude, gh}, filepath.Join(t.TempDir(), "backup.zip"))
	if err == nil {
		t.Fatal("expected 'no data found' error since the fake helper does not populate the staging dir")
	}

	run := r.callContaining("run")
	if run == nil {
		t.Fatalf("expected a docker run call, got %v", r.calls)
	}
	for _, want := range []string{
		types.SharedConfigVolumeName + ":/vol:ro",
		"ENTRY_IDS=claude gh",
		syncHelperImage,
	} {
		if !sliceHas(run, want) {
			t.Errorf("run call missing %q: %v", want, run)
		}
	}
}

func TestWriteZipFromDir_RoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "claude", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "claude", "settings.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "claude", "nested", "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "claude.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(t.TempDir(), "backup.zip")
	if err := writeZipFromDir(srcDir, zipPath, []string{"claude", "claude.json"}); err != nil {
		t.Fatalf("writeZipFromDir: %v", err)
	}

	destDir := t.TempDir()
	if err := extractZipToDir(zipPath, destDir); err != nil {
		t.Fatalf("extractZipToDir: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "claude", "settings.json"))
	if err != nil || string(got) != `{"ok":true}` {
		t.Errorf("claude/settings.json round-trip mismatch: %q, err=%v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(destDir, "claude", "nested", "deep.txt"))
	if err != nil || string(got) != "deep" {
		t.Errorf("claude/nested/deep.txt round-trip mismatch: %q, err=%v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(destDir, "claude.json"))
	if err != nil || string(got) != "{}" {
		t.Errorf("claude.json round-trip mismatch: %q, err=%v", got, err)
	}
}

// TestWriteZipFromDir_PreservesSymlinks is the regression guard for the backup
// crash: tools running inside the container write symlinks directly onto the
// volume (e.g. ~/.claude/debug/latest), frequently dangling. writeZipFromDir
// must archive them as symlinks (not os.Open them, which follows the link and
// fails on a dangling one), and extractZipToDir must recreate them faithfully.
func TestWriteZipFromDir_PreservesSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "claude", "debug"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A real file the valid symlink can point at.
	if err := os.WriteFile(filepath.Join(srcDir, "claude", "debug", "2026.log"), []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A valid symlink and a dangling one (the case that crashed the backup).
	if err := os.Symlink("2026.log", filepath.Join(srcDir, "claude", "debug", "current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("nonexistent.log", filepath.Join(srcDir, "claude", "debug", "latest")); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(t.TempDir(), "backup.zip")
	if err := writeZipFromDir(srcDir, zipPath, []string{"claude"}); err != nil {
		t.Fatalf("writeZipFromDir must not fail on symlinks (incl. dangling): %v", err)
	}

	destDir := t.TempDir()
	if err := extractZipToDir(zipPath, destDir); err != nil {
		t.Fatalf("extractZipToDir: %v", err)
	}

	for name, wantTarget := range map[string]string{
		"current": "2026.log",
		"latest":  "nonexistent.log",
	} {
		p := filepath.Join(destDir, "claude", "debug", name)
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("Lstat %s: %v", name, err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s round-tripped as a regular file, want a symlink", name)
			continue
		}
		got, err := os.Readlink(p)
		if err != nil {
			t.Fatalf("Readlink %s: %v", name, err)
		}
		if got != wantTarget {
			t.Errorf("%s target = %q, want %q", name, got, wantTarget)
		}
	}

	// The regular file alongside the symlinks still round-trips.
	got, err := os.ReadFile(filepath.Join(destDir, "claude", "debug", "2026.log"))
	if err != nil || string(got) != "log" {
		t.Errorf("claude/debug/2026.log round-trip mismatch: %q, err=%v", got, err)
	}
}

func TestExtractZipToDir_RejectsPathTraversal(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("pwned")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	destDir := t.TempDir()
	if err := extractZipToDir(zipPath, destDir); err == nil {
		t.Fatal("expected extractZipToDir to reject a path-traversal entry")
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(destDir), "evil.txt")); statErr == nil {
		t.Fatal("path-traversal entry escaped destDir")
	}
}

func TestRestoreFromZip_ParsesRestoredAndSkipped(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "claude", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "gh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "gh", "config.yml"), []byte("x: 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "backup.zip")
	if err := writeZipFromDir(srcDir, zipPath, []string{"claude", "gh"}); err != nil {
		t.Fatalf("writeZipFromDir: %v", err)
	}

	// status 0 also satisfies EnsureSharedConfigVolume's `volume inspect`
	// check (volume already exists, no create call needed).
	r := &fakeRunner{status: 0, stdout: "RESTORED claude\nSKIPPED gh"}
	defer useFakeDocker(r)()

	svc := SharedConfigService{Report: nopReporter{}}
	claude, _ := types.SharedConfigEntryByID("claude")
	gh, _ := types.SharedConfigEntryByID("gh")
	res, err := svc.RestoreFromZip(zipPath, []types.SharedConfigEntry{claude, gh}, false)
	if err != nil {
		t.Fatalf("RestoreFromZip: %v", err)
	}
	if len(res.Restored) != 1 || res.Restored[0] != "claude" {
		t.Errorf("Restored = %v, want [claude]", res.Restored)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "gh" {
		t.Errorf("Skipped = %v, want [gh]", res.Skipped)
	}
	if len(res.Missing) != 0 {
		t.Errorf("Missing = %v, want []", res.Missing)
	}

	run := r.callContaining("run")
	if run == nil || !sliceHas(run, "ENTRY_IDS=claude gh") {
		t.Errorf("expected ENTRY_IDS=claude gh in run call, got %v", run)
	}
}

func TestRestoreFromZip_ForcePassesFlagToHelper(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "claude.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "backup.zip")
	if err := writeZipFromDir(srcDir, zipPath, []string{"claude.json"}); err != nil {
		t.Fatalf("writeZipFromDir: %v", err)
	}

	r := &fakeRunner{status: 0, stdout: "RESTORED claude.json"}
	defer useFakeDocker(r)()

	svc := SharedConfigService{Report: nopReporter{}}
	entry, _ := types.SharedConfigEntryByID("claude.json")
	if _, err := svc.RestoreFromZip(zipPath, []types.SharedConfigEntry{entry}, true); err != nil {
		t.Fatalf("RestoreFromZip: %v", err)
	}
	run := r.callContaining("run")
	if run == nil || !sliceHas(run, "ENTRY_FORCE=1") {
		t.Errorf("expected ENTRY_FORCE=1 in run call, got %v", run)
	}
}

func TestRestoreFromZip_ReportsMissingEntriesNotInZip(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "claude", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "backup.zip")
	if err := writeZipFromDir(srcDir, zipPath, []string{"claude"}); err != nil {
		t.Fatalf("writeZipFromDir: %v", err)
	}

	r := &fakeRunner{status: 0, stdout: "RESTORED claude"}
	defer useFakeDocker(r)()

	svc := SharedConfigService{Report: nopReporter{}}
	claude, _ := types.SharedConfigEntryByID("claude")
	gh, _ := types.SharedConfigEntryByID("gh")
	res, err := svc.RestoreFromZip(zipPath, []types.SharedConfigEntry{claude, gh}, false)
	if err != nil {
		t.Fatalf("RestoreFromZip: %v", err)
	}
	if len(res.Missing) != 1 || res.Missing[0] != "gh" {
		t.Fatalf("Missing = %v, want [gh]", res.Missing)
	}
	run := r.callContaining("run")
	if run == nil || sliceHas(run, "ENTRY_IDS=claude gh") {
		t.Errorf("expected only the present entry (claude) passed to the helper, got %v", run)
	}
	if !sliceHas(run, "ENTRY_IDS=claude") {
		t.Errorf("expected ENTRY_IDS=claude in run call, got %v", run)
	}
}
