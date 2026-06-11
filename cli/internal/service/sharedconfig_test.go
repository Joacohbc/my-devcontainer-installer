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
