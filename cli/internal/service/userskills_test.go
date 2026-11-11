package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestUserSkillsAreVersionedInTheConfigRepo(t *testing.T) {
	isolateGit(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}

	manifest := types.UserSkillManifest{ID: "my-skill", Ref: "owner/repo", Skill: "my-skill"}
	if err := svc.SaveUserSkill(manifest, false); err != nil {
		t.Fatal(err)
	}
	manifest.Label = "My skill"
	if err := svc.SaveUserSkill(manifest, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveUserSkills([]string{"my-skill"}); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("git", "-C", domain.GlobalConfigDir(), "log", "--format=%s").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "chore(skill): remove my-skill\nfeat(skill): update my-skill\nfeat(skill): add my-skill\n" + initConfigRepoMessage
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("unexpected git log:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveUserSkillsValidatesBeforeDeleting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(domain.SkillDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(domain.SkillDir(), "keep.yml")
	if err := os.WriteFile(keep, []byte("id: keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := ConfigService{}
	if err := svc.RemoveUserSkills([]string{"keep", "ghost"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found, got %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("valid skill must survive a failed validation: %v", err)
	}
}
