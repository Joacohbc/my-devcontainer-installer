package catalog

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// TestEverySelectableHasUICategory guards the single-source-of-truth invariant:
// any module/service the wizard can offer must declare a UICategory, otherwise
// it would silently vanish from the grouped selection.
func TestEverySelectableHasUICategory(t *testing.T) {
	for _, m := range DockerfileModules {
		if m.Always || m.Internal {
			continue
		}
		if m.UICategory == "" {
			t.Errorf("dockerfile module %q is selectable but has no UICategory", m.ID)
		}
	}
	for _, s := range ComposeServices {
		if s.Always || s.Internal {
			continue
		}
		if s.UICategory == "" {
			t.Errorf("compose service %q is selectable but has no UICategory", s.ID)
		}
	}
}

func TestSelectableByCategoryMapping(t *testing.T) {
	grouped := SelectableByCategory()

	want := map[types.UICategory][]string{
		types.UICategoryAITools:   {"claude-code", "opencode", "codex-cli", "antigravity-cli", "copilot-cli"},
		types.UICategoryLanguages: {"java-temurin", "java-openjdk", "python", "go", "php", "rust", "c-cpp", "nodejs", "pnpm", "yarn", "bun"},
		types.UICategoryDatabases: {"sqlite", "mongo", "redis", "postgres"},
		types.UICategoryDevTools:  {"github-cli", "chrome", "ffmpeg", "dod", "ngrok", "cloudflared"},
		types.UICategoryClients:   {"postgres-client", "redis-client", "mongo-client"},
	}

	for cat, wantIDs := range want {
		gotIDs := map[string]bool{}
		for _, e := range grouped[cat] {
			gotIDs[e.ID] = true
		}
		if len(gotIDs) != len(wantIDs) {
			t.Errorf("category %q: got %d entries, want %d (%v)", cat, len(gotIDs), len(wantIDs), grouped[cat])
		}
		for _, id := range wantIDs {
			if !gotIDs[id] {
				t.Errorf("category %q: missing %q", cat, id)
			}
		}
	}
}

// TestSelectableExcludesAlwaysOn verifies base/cleanup/devcontainer never leak
// into the grouped selection regardless of any UICategory they might carry.
func TestSelectableExcludesAlwaysOn(t *testing.T) {
	grouped := SelectableByCategory()
	excluded := map[string]bool{"base": true, "cleanup": true, "devcontainer": true}
	for _, entries := range grouped {
		for _, e := range entries {
			if excluded[e.ID] {
				t.Errorf("always-on entry %q must not appear in the grouped selection", e.ID)
			}
		}
	}
}

// Zellij now ships in the base image by default: it must be always-on (like
// base/cleanup) and must never appear as a selectable entry in the wizard.
func TestZellijIsAlwaysOnAndNotSelectable(t *testing.T) {
	m := GetDockerfileModule("zellij")
	if m == nil {
		t.Fatal("expected zellij module to exist in the catalog")
	}
	if !m.Always {
		t.Error("zellij must be always-on so it ships in the base image by default")
	}
	if m.UICategory != "" {
		t.Errorf("zellij must not be selectable (no UICategory), got %q", m.UICategory)
	}
	for cat, entries := range SelectableByCategory() {
		for _, e := range entries {
			if e.ID == "zellij" {
				t.Errorf("zellij must not appear in the grouped selection (category %q)", cat)
			}
		}
	}
}

func TestCategoriesInOrder(t *testing.T) {
	got := CategoriesInOrder()
	want := types.UICategoryOrder // all five have at least one selectable entry
	if len(got) != len(want) {
		t.Fatalf("got %d categories, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
