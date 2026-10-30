package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestResolveDockerfileModules_AlwaysOnIncluded(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]types.SelectedModule{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	assertContains(t, ids, "base")
	assertContains(t, ids, "aliases")
	assertContains(t, ids, "cleanup")
}

// The aliases module appends to the rc files, and base's zsh installer rewrites
// ~/.zshrc from scratch, so aliases must always render AFTER base.
func TestResolveDockerfileModules_AliasesOrderedAfterBase(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]types.SelectedModule{{ID: "python"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	base, aliases := -1, -1
	for i, id := range ids {
		switch id {
		case "base":
			base = i
		case "aliases":
			aliases = i
		}
	}
	if base == -1 || aliases == -1 {
		t.Fatalf("expected both base and aliases in %v", ids)
	}
	if aliases < base {
		t.Errorf("aliases (%d) must come after base (%d): %v", aliases, base, ids)
	}
}

func TestResolveDockerfileModules_RequiresAutoAdded(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]types.SelectedModule{{ID: "nodejs"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	assertContains(t, ids, "github-cli")
	assertContains(t, ids, "nodejs")
}

func TestResolveDockerfileModules_TransitiveRequires(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]types.SelectedModule{{ID: "pnpm"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	assertContains(t, ids, "pnpm")
	assertContains(t, ids, "nodejs")
	assertContains(t, ids, "github-cli")
}

func TestResolveDockerfileModules_CodexCliDependencies(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]types.SelectedModule{{ID: "codex-cli"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	assertContains(t, ids, "codex-cli")
	assertContains(t, ids, "nodejs")
	assertContains(t, ids, "github-cli")
}

func TestResolveDockerfileModules_UnknownModuleErrors(t *testing.T) {
	_, err := domain.ResolveDockerfileModules([]types.SelectedModule{{ID: "nonexistent"}})
	if err == nil {
		t.Fatal("expected error for unknown module, got nil")
	}
	var re domain.ResolverError
	if !errors.As(err, &re) {
		t.Errorf("expected ResolverError, got %T: %v", err, err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "devcontainer-cli agent cli-info") {
		t.Errorf("expected error message to suggest 'devcontainer-cli agent cli-info', got: %s", msg)
	}
	if !strings.Contains(msg, "when: start") {
		t.Errorf("expected error message to mention 'when: start', got: %s", msg)
	}
}

func TestResolveDockerfileModules_RemovedModuleError(t *testing.T) {
	removedModules := []string{"graphify", "caveman", "claude-mem", "context-mode", "ecc", "browser-harness", "ngrok"}
	for _, mod := range removedModules {
		_, err := domain.ResolveDockerfileModules([]types.SelectedModule{{ID: types.ModuleID(mod)}})
		if err == nil {
			t.Fatalf("expected error for removed module %q, got nil", mod)
		}
		var re domain.ResolverError
		if !errors.As(err, &re) {
			t.Errorf("expected ResolverError for %q, got %T: %v", mod, err, err)
		}
		msg := err.Error()
		if !strings.Contains(msg, "devcontainer-cli agent cli-info") {
			t.Errorf("expected hint to suggest 'devcontainer-cli agent cli-info' for %q, got: %s", mod, msg)
		}
		if !strings.Contains(msg, "when: start") {
			t.Errorf("expected hint to mention 'when: start' for %q, got: %s", mod, msg)
		}
	}
}

func TestResolveDockerfileModules_CategoryOrder(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]types.SelectedModule{
		{ID: "java-temurin"},
		{ID: "github-cli"},
		{ID: "nodejs"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	if ids[0] != "base" {
		t.Errorf("first module should be 'base', got %q", ids[0])
	}
	if ids[len(ids)-1] != "cleanup" {
		t.Errorf("last module should be 'cleanup', got %q", ids[len(ids)-1])
	}
}

func TestResolveDockerfileModules_OptionsPassThrough(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]types.SelectedModule{
		{ID: "java-temurin", Options: map[string]any{"versions": []any{"17"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var java *domain.ResolvedModule
	for i := range resolved {
		if resolved[i].Module.ID == "java-temurin" {
			java = &resolved[i]
			break
		}
	}
	if java == nil {
		t.Fatal("java-temurin module not found")
	}
	versions, ok := java.Options["versions"].([]any)
	if !ok || len(versions) != 1 || versions[0] != "17" {
		t.Errorf("expected versions=[17], got %v", java.Options["versions"])
	}
}

func TestResolveDockerfileModules_ConflictsError(t *testing.T) {
	_, err := domain.ResolveDockerfileModules([]types.SelectedModule{
		{ID: "java-temurin"},
		{ID: "java-openjdk"},
	})
	if err == nil {
		t.Fatal("expected error for conflicting modules, got nil")
	}
	var re domain.ResolverError
	if !errors.As(err, &re) {
		t.Errorf("expected ResolverError, got %T: %v", err, err)
	}
}

func moduleIDs(resolved []domain.ResolvedModule) []string {
	ids := make([]string, len(resolved))
	for i, r := range resolved {
		ids[i] = string(r.Module.ID)
	}
	return ids
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, v := range haystack {
		if v == needle {
			return
		}
	}
	t.Errorf("expected %q in %v", needle, haystack)
}
