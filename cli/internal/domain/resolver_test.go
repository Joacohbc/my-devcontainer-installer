package domain_test

import (
	"errors"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestResolveDockerfileModules_AlwaysOnIncluded(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]core.SelectedModule{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	assertContains(t, ids, "base")
	assertContains(t, ids, "cleanup")
}

func TestResolveDockerfileModules_RequiresAutoAdded(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]core.SelectedModule{{ID: "nodejs"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	assertContains(t, ids, "github-cli")
	assertContains(t, ids, "nodejs")
}

func TestResolveDockerfileModules_TransitiveRequires(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]core.SelectedModule{{ID: "pnpm"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := moduleIDs(resolved)
	assertContains(t, ids, "pnpm")
	assertContains(t, ids, "nodejs")
	assertContains(t, ids, "github-cli")
}

func TestResolveDockerfileModules_UnknownModuleErrors(t *testing.T) {
	_, err := domain.ResolveDockerfileModules([]core.SelectedModule{{ID: "nonexistent"}})
	if err == nil {
		t.Fatal("expected error for unknown module, got nil")
	}
	var re domain.ResolverError
	if !errors.As(err, &re) {
		t.Errorf("expected ResolverError, got %T: %v", err, err)
	}
}

func TestResolveDockerfileModules_CategoryOrder(t *testing.T) {
	resolved, err := domain.ResolveDockerfileModules([]core.SelectedModule{
		{ID: "java-temurin"},
		{ID: "dod"},
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
	resolved, err := domain.ResolveDockerfileModules([]core.SelectedModule{
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
	_, err := domain.ResolveDockerfileModules([]core.SelectedModule{
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
		ids[i] = r.Module.ID
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
