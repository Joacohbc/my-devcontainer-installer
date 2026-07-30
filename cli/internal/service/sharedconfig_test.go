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

// TestSyncEntryScriptUsesDerefSymlinksForDirs guards against a regression to
// plain `cp -a` for directory entries: global skills/agents installed via the
// skills.sh CLI are symlinked into each tool's config dir (e.g.
// ~/.claude/skills/<name> -> ~/.agents/skills/<name>), so the copy into the
// volume must dereference those links and persist real content, not a
// dangling host-path symlink. The actual dereferencing behavior (including
// tolerance for already-dangling symlinks) is exercised directly against the
// shell in TestDerefSymlinksFunc.
func TestSyncEntryScriptUsesDerefSymlinksForDirs(t *testing.T) {
	if !strings.Contains(syncEntryScript, `deref_symlinks "$dst"`) {
		t.Errorf("syncEntryScript must call deref_symlinks on directory entries, got: %s", syncEntryScript)
	}
}

// TestDerefSymlinksFunc runs the actual `deref_symlinks` shell function (used
// by syncEntryScript) against a real directory to verify: a symlink whose
// target exists is replaced with a real copy of that content, and a dangling
// symlink (e.g. the ~/.claude/debug/latest pointer Claude Code itself leaves
// behind) is left alone instead of aborting the script. Before this test,
// syncEntryScript used a bare `cp -aL`, which errors "cannot stat" on a
// dangling symlink and failed the whole sync for any entry containing one.
func TestDerefSymlinksFunc(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "resolvable-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "dangling-link")); err != nil {
		t.Fatal(err)
	}

	script := derefSymlinksFunc + `deref_symlinks "$1"`
	cmd := exec.Command("sh", "-c", script, "sh", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("deref_symlinks failed: %v, output: %s", err, out)
	}

	resolvable := filepath.Join(dir, "resolvable-link")
	info, err := os.Lstat(resolvable)
	if err != nil {
		t.Fatalf("resolvable-link: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("resolvable-link should have been replaced with a real copy, still a symlink")
	}
	content, err := os.ReadFile(resolvable)
	if err != nil || string(content) != "hello" {
		t.Errorf("resolvable-link content = %q, %v; want %q", content, err, "hello")
	}

	dangling := filepath.Join(dir, "dangling-link")
	danglingInfo, err := os.Lstat(dangling)
	if err != nil {
		t.Fatalf("dangling-link should still exist (as a symlink): %v", err)
	}
	if danglingInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("dangling-link should remain a symlink, not be touched")
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
