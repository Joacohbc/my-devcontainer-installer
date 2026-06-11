package service

import (
	"fmt"
	"os"
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

func TestSyncFromHostCopiesViaRunningCarrier(t *testing.T) {
	// fakeRunner stdout "dc-x" makes `docker ps` report a running carrier; the
	// emptiness checks return the same string (≠ "nonempty") so entries copy.
	r := &fakeRunner{status: 0, stdout: "dc-x"}
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

	cp := r.callContaining("cp")
	if cp == nil {
		t.Fatalf("expected a docker cp call, got %v", r.calls)
	}
	wantDst := "dc-x:" + types.SharedConfigMountPath + "/claude"
	if !sliceHas(cp, wantDst) {
		t.Errorf("cp call missing destination %q: %v", wantDst, cp)
	}
	if chown := r.callContaining("devuser:devuser"); chown == nil {
		t.Errorf("expected chown to devuser on managed carrier, calls=%v", r.calls)
	}
	if helper := r.callContaining(syncHelperName); helper != nil {
		t.Errorf("must not start a helper when a carrier is running: %v", helper)
	}
}

func TestSyncFromHostSkipsNonEmptyWithoutForce(t *testing.T) {
	// stdout "nonempty" doubles as the carrier name from `docker ps` and as the
	// emptiness probe result, so every entry is skipped.
	r := &fakeRunner{status: 0, stdout: "nonempty"}
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
	if cp := r.callContaining("cp"); cp != nil {
		t.Errorf("must not docker cp a non-empty entry without force: %v", cp)
	}
}

func TestSyncFromHostReportsMissingHostConfig(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "dc-x"}
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
	if cp := r.callContaining("cp"); cp != nil {
		t.Errorf("must not docker cp an entry absent on the host: %v", cp)
	}
}

func TestSyncFromHostStartsHelperWhenNoCarrier(t *testing.T) {
	// Empty stdout: `docker ps` finds no carrier, so a temporary helper runs and
	// is removed afterwards.
	r := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(r)()

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "gh"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := SharedConfigService{Report: nopReporter{}}
	entry, _ := types.SharedConfigEntryByID("gh")
	if _, err := svc.SyncFromHost([]types.SharedConfigEntry{entry}, home, false); err != nil {
		t.Fatalf("SyncFromHost: %v", err)
	}
	if run := r.callContaining(syncHelperImage); run == nil {
		t.Errorf("expected a helper `docker run` with %s, calls=%v", syncHelperImage, r.calls)
	}
	if rm := r.callContaining("rm"); rm == nil || !sliceHas(rm, syncHelperName) {
		t.Errorf("expected the helper to be removed, calls=%v", r.calls)
	}
}
