package service

import (
	"fmt"
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
