package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/spf13/cobra"
)

// newTestConfigSkillAddCmd builds an 'add' command with --no-interactive set,
// so a caller only has to set the flags a given test cares about.
func newTestConfigSkillAddCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := newConfigSkillAddCommand()
	if err := cmd.Flags().Set("no-interactive", "true"); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestRunConfigSkillAdd_RequiresRefNonInteractive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newTestConfigSkillAddCmd(t)
	if err := runConfigSkillAdd(cmd, []string{"my-skill"}); err == nil {
		t.Fatal("expected an error without --ref in non-interactive mode")
	}
}

func TestRunConfigSkillAdd_InvalidID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newTestConfigSkillAddCmd(t)
	if err := cmd.Flags().Set("ref", "owner/repo"); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSkillAdd(cmd, []string{"bad id!"}); err == nil {
		t.Fatal("expected an error for an invalid skill id")
	}
}

func TestRunConfigSkillAdd_CreatesFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newTestConfigSkillAddCmd(t)
	for flag, val := range map[string]string{
		"ref":              "owner/my-skill",
		"label":            "My Skill",
		"skill":            "sel",
		"requires-modules": "nodejs,python",
		"context-body":     "Teaches an agent to do X.",
	} {
		if err := cmd.Flags().Set(flag, val); err != nil {
			t.Fatal(err)
		}
	}
	if err := runConfigSkillAdd(cmd, []string{"my-skill"}); err != nil {
		t.Fatalf("runConfigSkillAdd: %v", err)
	}

	path := filepath.Join(domain.SkillDir(), "my-skill.yml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}

	specs := catalog.LoadUserSkills(domain.SkillDir())
	if len(specs) != 1 {
		t.Fatalf("expected 1 loaded skill, got %d", len(specs))
	}
	s := specs[0]
	if s.Ref != "owner/my-skill" || s.Label != "My Skill" || s.Skill != "sel" {
		t.Errorf("unexpected spec: %+v", s)
	}
	if len(s.RequiresModules) != 2 {
		t.Errorf("expected 2 required modules, got %v", s.RequiresModules)
	}
	if s.Context == nil || s.Context().Body != "Teaches an agent to do X." {
		t.Error("expected the context body to round-trip")
	}
}

func TestRunConfigSkillAdd_RefusesOverwriteWithoutForce(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newTestConfigSkillAddCmd(t)
	cmd.Flags().Set("ref", "owner/repo")
	if err := runConfigSkillAdd(cmd, []string{"my-skill"}); err != nil {
		t.Fatalf("first add: %v", err)
	}

	cmd2 := newTestConfigSkillAddCmd(t)
	cmd2.Flags().Set("ref", "owner/repo2")
	if err := runConfigSkillAdd(cmd2, []string{"my-skill"}); err == nil {
		t.Fatal("expected an error re-adding the same id without --force")
	}

	cmd3 := newTestConfigSkillAddCmd(t)
	cmd3.Flags().Set("ref", "owner/repo2")
	cmd3.Flags().Set("force", "true")
	if err := runConfigSkillAdd(cmd3, []string{"my-skill"}); err != nil {
		t.Fatalf("--force add: %v", err)
	}
	specs := catalog.LoadUserSkills(domain.SkillDir())
	if len(specs) != 1 || specs[0].Ref != "owner/repo2" {
		t.Errorf("expected --force to overwrite, got %+v", specs)
	}
}

// Adding a skill under a built-in id must succeed (it shadows, not
// collides) — only an existing *user* file under that id blocks without
// --force.
func TestRunConfigSkillAdd_ShadowsBuiltinWithoutForce(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newTestConfigSkillAddCmd(t)
	cmd.Flags().Set("ref", "me/firecrawl-fork")
	if err := runConfigSkillAdd(cmd, []string{"firecrawl"}); err != nil {
		t.Fatalf("expected shadowing a built-in to succeed: %v", err)
	}
	spec := catalog.GetAgentSkill("firecrawl", domain.SkillDir())
	if spec == nil || spec.Ref != "me/firecrawl-fork" {
		t.Errorf("expected the user-defined firecrawl to win, got %+v", spec)
	}
}

func TestRunConfigSkillRemove_DeletesUserSkill(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	addCmd := newTestConfigSkillAddCmd(t)
	addCmd.Flags().Set("ref", "owner/repo")
	if err := runConfigSkillAdd(addCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	rmCmd := newConfigSkillRemoveCommand()
	rmCmd.Flags().Set("yes", "true")
	if err := runConfigSkillRemove(rmCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("runConfigSkillRemove: %v", err)
	}
	if specs := catalog.LoadUserSkills(domain.SkillDir()); len(specs) != 0 {
		t.Errorf("expected the skill to be gone, got %+v", specs)
	}
}

func TestRunConfigSkillRemove_RefusesBuiltin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rmCmd := newConfigSkillRemoveCommand()
	rmCmd.Flags().Set("yes", "true")
	if err := runConfigSkillRemove(rmCmd, []string{"firecrawl"}); err == nil {
		t.Fatal("expected an error removing a built-in skill")
	}
}

func TestRunConfigSkillRemove_RequiresConfirmationNonInteractive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	addCmd := newTestConfigSkillAddCmd(t)
	addCmd.Flags().Set("ref", "owner/repo")
	if err := runConfigSkillAdd(addCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	rmCmd := newConfigSkillRemoveCommand()
	rmCmd.Flags().Set("no-interactive", "true")
	if err := runConfigSkillRemove(rmCmd, []string{"my-skill"}); err == nil {
		t.Fatal("expected an error without --yes in non-interactive mode")
	}
	if specs := catalog.LoadUserSkills(domain.SkillDir()); len(specs) != 1 {
		t.Error("expected the skill to survive an unconfirmed removal attempt")
	}
}

func TestRunConfigSkillRemove_UnknownUserSkillFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rmCmd := newConfigSkillRemoveCommand()
	rmCmd.Flags().Set("yes", "true")
	if err := runConfigSkillRemove(rmCmd, []string{"no-such-skill"}); err == nil {
		t.Fatal("expected an error for an id that names no user-defined skill")
	}
}
