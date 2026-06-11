package types

import (
	"strings"
	"testing"
)

func TestSharedConfigEntries(t *testing.T) {
	if len(SharedConfigEntries) == 0 {
		t.Fatal("expected at least one shared-config entry")
	}

	ids := map[string]bool{}
	targets := map[string]bool{}
	for _, e := range SharedConfigEntries {
		if e.ID == "" || e.Target == "" {
			t.Errorf("entry has empty id/target: %+v", e)
		}
		if e.Kind != SharedConfigDir && e.Kind != SharedConfigFile {
			t.Errorf("entry %s has invalid kind %q", e.ID, e.Kind)
		}
		if ids[e.ID] {
			t.Errorf("duplicate entry id %q", e.ID)
		}
		if targets[e.Target] {
			t.Errorf("duplicate entry target %q", e.Target)
		}
		ids[e.ID] = true
		targets[e.Target] = true
	}

	// The Claude session config is a single file, not a directory.
	var found bool
	for _, e := range SharedConfigEntries {
		if e.ID == "claude.json" {
			found = true
			if e.Kind != SharedConfigFile {
				t.Errorf("claude.json must be a file entry, got %q", e.Kind)
			}
			if e.Target != ".claude.json" {
				t.Errorf("claude.json target = %q, want .claude.json", e.Target)
			}
		}
	}
	if !found {
		t.Error("expected a claude.json entry")
	}

	// gh persists under ~/.config/gh, so its target must include a parent dir.
	for _, e := range SharedConfigEntries {
		if e.ID == "gh" && !strings.Contains(e.Target, "/") {
			t.Errorf("gh target %q should be nested (have a parent dir)", e.Target)
		}
	}
}

func TestSharedConfigEntryByID(t *testing.T) {
	e, ok := SharedConfigEntryByID("claude")
	if !ok || e.Target != ".claude" {
		t.Errorf("unexpected claude entry: %+v ok=%v", e, ok)
	}
	if _, ok := SharedConfigEntryByID("nope"); ok {
		t.Error("expected unknown id to be reported as missing")
	}
	if got, want := len(SharedConfigIDs()), len(SharedConfigEntries); got != want {
		t.Errorf("SharedConfigIDs() has %d ids, want %d", got, want)
	}
}

func TestSharedConfigMount(t *testing.T) {
	if got, want := SharedConfigMount(), "devcontainer-shared-config:/mnt/shared-config"; got != want {
		t.Errorf("SharedConfigMount() = %q, want %q", got, want)
	}
	// The volume name must not contain '_', so it can never collide with the
	// workspace_<name> prefixing scheme used for the persistence volumes.
	if strings.Contains(SharedConfigVolumeName, "_") {
		t.Errorf("shared volume name %q must not contain '_'", SharedConfigVolumeName)
	}
}

func TestSharedConfigEnabled(t *testing.T) {
	yes := true
	no := false
	cases := []struct {
		name string
		val  *bool
		want bool
	}{
		{"nil/unset is enabled", nil, true},
		{"explicit true", &yes, true},
		{"explicit false", &no, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &DevcontainerConfig{Compose: ComposeConfig{SharedConfig: c.val}}
			if got := SharedConfigEnabled(cfg); got != c.want {
				t.Errorf("SharedConfigEnabled = %v, want %v", got, c.want)
			}
		})
	}
}
