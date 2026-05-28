package domain_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func configForConflicts(t *testing.T) (*types.DevcontainerConfig, []string, string) {
	t.Helper()
	cfg := domain.DefaultConfig(t.TempDir())
	cfg.Image = "conflict-test:local"
	containers, network, _, err := domain.PlannedComposeNames(cfg)
	if err != nil {
		t.Fatalf("PlannedComposeNames: %v", err)
	}
	if len(containers) == 0 {
		t.Fatal("expected at least one planned container (always-on devcontainer service)")
	}
	return cfg, containers, network
}

func argsContain(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestFindConflicts_NilWhenDockerUnavailable(t *testing.T) {
	cfg := domain.DefaultConfig(t.TempDir())
	if c := domain.FindConflicts(cfg, false, nil); c != nil {
		t.Errorf("expected nil conflicts when docker unavailable, got %+v", c)
	}
}

func TestFindConflicts_ForeignContainerIsConflict(t *testing.T) {
	cfg, containers, _ := configForConflicts(t)
	capture := func(args []string) (int, string, string) {
		if argsContain(args, "container") {
			return 0, containers[0] + "\tsome-other-project\n", ""
		}
		return 0, "", ""
	}
	conflicts := domain.FindConflicts(cfg, true, capture)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d (%+v)", len(conflicts), conflicts)
	}
	if conflicts[0].Kind != "container" || conflicts[0].Name != containers[0] || conflicts[0].Owner != "some-other-project" {
		t.Errorf("unexpected conflict: %+v", conflicts[0])
	}
}

func TestFindConflicts_SameProjectIsNotConflict(t *testing.T) {
	cfg, containers, _ := configForConflicts(t)
	project := types.ProjectID(cfg)
	capture := func(args []string) (int, string, string) {
		if argsContain(args, "container") {
			return 0, containers[0] + "\t" + project + "\n", ""
		}
		return 0, "", ""
	}
	if c := domain.FindConflicts(cfg, true, capture); len(c) != 0 {
		t.Errorf("a container owned by the same project is not a conflict, got %+v", c)
	}
}

func TestFindConflicts_ForeignNetworkIsConflict(t *testing.T) {
	cfg, _, network := configForConflicts(t)
	capture := func(args []string) (int, string, string) {
		if argsContain(args, "network") {
			return 0, network + "\tother-project\n", ""
		}
		return 0, "", ""
	}
	conflicts := domain.FindConflicts(cfg, true, capture)
	found := false
	for _, c := range conflicts {
		if c.Kind == "network" && c.Name == network {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a network conflict for %q, got %+v", network, conflicts)
	}
}

func TestFindConflicts_UnlabeledOwnerFallback(t *testing.T) {
	cfg, containers, _ := configForConflicts(t)
	capture := func(args []string) (int, string, string) {
		if argsContain(args, "container") {
			// No label column -> empty owner.
			return 0, containers[0] + "\n", ""
		}
		return 0, "", ""
	}
	conflicts := domain.FindConflicts(cfg, true, capture)
	if len(conflicts) != 1 || !strings.Contains(conflicts[0].Owner, "no label") {
		t.Errorf("expected fallback owner for unlabeled resource, got %+v", conflicts)
	}
}
