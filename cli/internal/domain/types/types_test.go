package types

import (
	"reflect"
	"testing"
)

func TestPersistVolumeSpecByID(t *testing.T) {
	spec, ok := PersistVolumeSpecByID("etc")
	if !ok {
		t.Fatal("expected etc to be a known persistence volume")
	}
	if spec.Volume != "devcontainer_etc" || spec.Mount != "/etc" {
		t.Errorf("unexpected spec for etc: %+v", spec)
	}
	if _, ok := PersistVolumeSpecByID("nope"); ok {
		t.Error("expected unknown id to be reported as missing")
	}
}

func TestDefaultPersistVolumeIDs(t *testing.T) {
	got := DefaultPersistVolumeIDs()
	want := []string{"etc", "root", "home"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultPersistVolumeIDs() = %v, want %v", got, want)
	}
	// Must stay in sync with the spec catalog.
	if len(got) != len(PersistVolumeSpecs) {
		t.Errorf("default ids (%d) out of sync with specs (%d)", len(got), len(PersistVolumeSpecs))
	}
}
