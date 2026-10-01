package commands

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
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

func TestConfigSkillInstall_DocumentedAndRegistered(t *testing.T) {
	cmd := newConfigSkillInstallCommand()
	if cmd.Short == "" {
		t.Error("config skill install has no Short")
	}
	if strings.HasSuffix(cmd.Short, ".") {
		t.Errorf("config skill install Short should not end with a period: %q", cmd.Short)
	}
	if cmd.Long == "" {
		t.Error("config skill install has no Long")
	}
	if cmd.Example == "" {
		t.Error("config skill install has no Example")
	}

	for _, flagName := range []string{"all", "image", "dir", "no-interactive"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("config skill install missing --%s", flagName)
		}
	}
}

func TestAgentSkillInstall_DocumentedAndRegistered(t *testing.T) {
	cmd := newAgentSkillInstallCommand()
	if cmd.Short == "" {
		t.Error("agent skill install has no Short")
	}
	if strings.HasSuffix(cmd.Short, ".") {
		t.Errorf("agent skill install Short should not end with a period: %q", cmd.Short)
	}
	if cmd.Long == "" {
		t.Error("agent skill install has no Long")
	}
	if cmd.Example == "" {
		t.Error("agent skill install has no Example")
	}

	for _, flagName := range []string{"all", "image", "dir", "no-interactive"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("agent skill install missing --%s", flagName)
		}
	}

	if cmd.PreRunE != nil {
		if err := cmd.PreRunE(cmd, nil); err != nil {
			t.Fatalf("PreRunE: %v", err)
		}
	}
	if interactiveFlag(cmd) {
		t.Error("agent skill install must default to non-interactive")
	}
}

func TestRunConfigSkillInstall_ExplicitSkill_InvokesDockerRun(t *testing.T) {
	runner := &buildStatusRunner{buildStatus: 0}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	cmd := newConfigSkillInstallCommand()
	cmd.Flags().Set("dir", t.TempDir())
	cmd.Flags().Set("no-interactive", "true")

	if err := runConfigSkillInstall(cmd, []string{"firecrawl"}); err != nil {
		t.Fatalf("runConfigSkillInstall: %v", err)
	}

	call := findRecordedCall(runner.calls, "--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", runner.calls)
	}
	if !slices.Contains(call, service.DefaultSkillInstallerImage) {
		t.Errorf("call missing default image %s: %v", service.DefaultSkillInstallerImage, call)
	}
	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "firecrawl") {
		t.Errorf("expected firecrawl in docker run args: %s", lastArg)
	}
}

func TestRunConfigSkillInstall_UnknownSkill_Fails(t *testing.T) {
	cmd := newConfigSkillInstallCommand()
	cmd.Flags().Set("no-interactive", "true")

	err := runConfigSkillInstall(cmd, []string{"unknown-xyz"})
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "unknown skill: unknown-xyz") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunAgentSkillInstall_NonInteractiveNoSkills_FailsActionable(t *testing.T) {
	cmd := newAgentSkillInstallCommand()
	cmd.Flags().Set("dir", t.TempDir())
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	err := runConfigSkillInstall(cmd, nil)
	if err == nil {
		t.Fatal("expected error for no skills in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "no skills specified") || !strings.Contains(err.Error(), "--all") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunConfigSkillInstall_AllFlag(t *testing.T) {
	runner := &buildStatusRunner{buildStatus: 0}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	cmd := newConfigSkillInstallCommand()
	cmd.Flags().Set("all", "true")
	cmd.Flags().Set("dir", t.TempDir())
	cmd.Flags().Set("no-interactive", "true")

	if err := runConfigSkillInstall(cmd, nil); err != nil {
		t.Fatalf("runConfigSkillInstall with --all: %v", err)
	}

	call := findRecordedCall(runner.calls, "--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", runner.calls)
	}
}

func TestCompleteInstallSkillArgs(t *testing.T) {
	cmd := newConfigSkillInstallCommand()
	completions, _ := completeInstallSkillArgs(cmd, []string{"firecrawl"}, "")
	if slices.Contains(completions, "firecrawl") {
		t.Error("already passed skill firecrawl should not be in completions")
	}
	if !slices.Contains(completions, "wayfinder") {
		t.Error("wayfinder should be in completions")
	}
}

func TestRunConfigSkillInstall_ExplicitSkillsAndAll_Fails(t *testing.T) {
	cmd := newConfigSkillInstallCommand()
	cmd.Flags().Set("all", "true")
	cmd.Flags().Set("dir", t.TempDir())
	cmd.Flags().Set("no-interactive", "true")

	err := runConfigSkillInstall(cmd, []string{"firecrawl"})
	if err == nil {
		t.Fatal("expected error when both skill args and --all are passed")
	}
	if !strings.Contains(err.Error(), "cannot specify both skill IDs and --all") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunConfigSkillInstall_InvalidDir_Fails(t *testing.T) {
	cmd := newConfigSkillInstallCommand()
	cmd.Flags().Set("dir", filepath.Join(t.TempDir(), "nonexistent-dir"))
	cmd.Flags().Set("no-interactive", "true")

	err := runConfigSkillInstall(cmd, []string{"firecrawl"})
	if err == nil {
		t.Fatal("expected error for non-existent target directory")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunAgentSkillInstall_UnknownSkill_Fails(t *testing.T) {
	cmd := newAgentSkillInstallCommand()
	cmd.Flags().Set("dir", t.TempDir())
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	err := runConfigSkillInstall(cmd, []string{"unknown-xyz"})
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "unknown skill: unknown-xyz") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunAgentSkillInstall_DirIsFile_Fails(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "a-file.txt")
	if err := os.WriteFile(tempFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentSkillInstallCommand()
	cmd.Flags().Set("dir", tempFile)
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	err := runConfigSkillInstall(cmd, []string{"firecrawl"})
	if err == nil {
		t.Fatal("expected error when target dir is a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunAgentSkillInstall_ExplicitSkill_InvokesDockerRun(t *testing.T) {
	runner := &buildStatusRunner{buildStatus: 0}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	cmd := newAgentSkillInstallCommand()
	cmd.Flags().Set("dir", t.TempDir())
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	if err := runConfigSkillInstall(cmd, []string{"firecrawl"}); err != nil {
		t.Fatalf("runConfigSkillInstall: %v", err)
	}

	call := findRecordedCall(runner.calls, "--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", runner.calls)
	}
	if !slices.Contains(call, service.DefaultSkillInstallerImage) {
		t.Errorf("call missing default image %s: %v", service.DefaultSkillInstallerImage, call)
	}
	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "firecrawl") {
		t.Errorf("expected firecrawl in docker run args: %s", lastArg)
	}
}

func TestRunConfigSkillInstall_CustomImage_PassedToRunner(t *testing.T) {
	runner := &buildStatusRunner{buildStatus: 0}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	cmd := newConfigSkillInstallCommand()
	cmd.Flags().Set("dir", t.TempDir())
	cmd.Flags().Set("no-interactive", "true")
	cmd.Flags().Set("image", "custom-registry.io/custom-utils:v1")

	if err := runConfigSkillInstall(cmd, []string{"firecrawl"}); err != nil {
		t.Fatalf("runConfigSkillInstall: %v", err)
	}

	call := findRecordedCall(runner.calls, "--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", runner.calls)
	}
	if !slices.Contains(call, "custom-registry.io/custom-utils:v1") {
		t.Errorf("expected custom image in docker run args, got: %v", call)
	}
}

func TestRunConfigSkillInstall_ProjectConfig_DefaultsToConfiguredSkills(t *testing.T) {
	runner := &buildStatusRunner{buildStatus: 0}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	tempDir := t.TempDir()
	configContent := `{"workspace":"testws","skills":{"skills":["firecrawl"]}}`
	if err := os.WriteFile(filepath.Join(tempDir, "devcontainer.config.json"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newConfigSkillInstallCommand()
	cmd.Flags().Set("dir", tempDir)
	cmd.Flags().Set("no-interactive", "true")

	if err := runConfigSkillInstall(cmd, nil); err != nil {
		t.Fatalf("runConfigSkillInstall: %v", err)
	}

	call := findRecordedCall(runner.calls, "--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", runner.calls)
	}
	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "firecrawl") {
		t.Errorf("expected firecrawl in docker run args: %s", lastArg)
	}
}

func findRecordedCall(calls [][]string, token string) []string {
	for _, call := range calls {
		if slices.Contains(call, token) {
			return call
		}
	}
	return nil
}
