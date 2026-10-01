package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

type mockSkillPrompter struct {
	selected []Option
	err      error
}

func (p mockSkillPrompter) Ask(string) (string, error)   { return "", nil }
func (p mockSkillPrompter) Confirm(string) (bool, error) { return false, nil }
func (p mockSkillPrompter) Select(string, []Option, Option) (Option, error) {
	return Option{}, nil
}
func (p mockSkillPrompter) Multiselect(string, []Option, []Option) ([]Option, error) {
	return p.selected, p.err
}
func (p mockSkillPrompter) Wizard(func(*State) []Step) (*State, error) {
	return nil, nil
}

func TestSkillInstallService_ExplicitSkills_Success(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	tempDir := t.TempDir()

	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         tempDir,
		Interactive: false,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", r.calls)
	}

	for _, token := range []string{
		"run", "--rm",
		"-v", tempDir + ":/workspace",
		"-w", "/workspace",
		DefaultSkillInstallerImage,
		"sh", "-c",
	} {
		if !slices.Contains(call, token) {
			t.Errorf("call missing expected token %q: %v", token, call)
		}
	}

	if owner := hostOwnerString(); owner != "" {
		if !slices.Contains(call, owner) {
			t.Errorf("call missing host owner %q: %v", owner, call)
		}
	}

	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "npx --yes skills add firecrawl/cli --skill firecrawl") {
		t.Errorf("unexpected script command: %s", lastArg)
	}
}

func TestSkillInstallService_ExplicitSkills_MultipleAndGrouped(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	tempDir := t.TempDir()

	err := svc.Install([]string{"wayfinder", "firecrawl"}, SkillInstallOptions{
		Dir:         tempDir,
		Interactive: false,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", r.calls)
	}

	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "npx --yes skills add mattpocock/skills --skill wayfinder") {
		t.Errorf("expected wayfinder install command in: %s", lastArg)
	}
	if !strings.Contains(lastArg, "npx --yes skills add firecrawl/cli --skill firecrawl") {
		t.Errorf("expected firecrawl install command in: %s", lastArg)
	}
	if !strings.Contains(lastArg, " && ") {
		t.Errorf("expected commands chained with &&: %s", lastArg)
	}
}

func TestSkillInstallService_ExplicitSkills_UnknownSkill_Fails(t *testing.T) {
	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install([]string{"non-existent-skill-xyz"}, SkillInstallOptions{
		Dir:         t.TempDir(),
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected error for unknown skill ID")
	}
	if !strings.Contains(err.Error(), "unknown skill: non-existent-skill-xyz") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_AllFlag_InstallsAllSkills(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	tempDir := t.TempDir()

	err := svc.Install(nil, SkillInstallOptions{
		Dir:         tempDir,
		All:         true,
		Interactive: false,
	})
	if err != nil {
		t.Fatalf("Install with --all: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", r.calls)
	}

	lastArg := call[len(call)-1]
	allSkills := catalog.AllAgentSkills()
	for _, s := range allSkills {
		if s.Skill != "" && !strings.Contains(lastArg, s.Skill) {
			t.Errorf("expected skill %q in install commands", s.Skill)
			break
		}
	}
}

func TestSkillInstallService_ProjectConfig_DefaultsToConfiguredSkills(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	tempDir := t.TempDir()
	configData := types.DevcontainerConfig{
		Workspace: "testws",
		Skills: types.SkillsConfig{
			Skills: []types.SkillID{"firecrawl"},
		},
	}
	rawConfig, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, types.ConfigFile), rawConfig, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc := SkillInstallService{Report: nopReporter{}}
	if err := svc.Install(nil, SkillInstallOptions{Dir: tempDir, Interactive: false}); err != nil {
		t.Fatalf("Install from project config: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", r.calls)
	}

	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "npx --yes skills add firecrawl/cli --skill firecrawl") {
		t.Errorf("unexpected script command: %s", lastArg)
	}
}

func TestSkillInstallService_NoSkillsNonInteractive_ReturnsActionableError(t *testing.T) {
	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install(nil, SkillInstallOptions{
		Dir:         t.TempDir(),
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected error when no skills specified in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "no skills specified") || !strings.Contains(err.Error(), "--all") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_InteractivePrompt_Success(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	prompter := mockSkillPrompter{
		selected: []Option{
			{Value: "firecrawl", Label: "firecrawl"},
		},
	}
	svc := SkillInstallService{Report: nopReporter{}, Prompt: prompter}
	tempDir := t.TempDir()

	err := svc.Install(nil, SkillInstallOptions{
		Dir:         tempDir,
		Interactive: true,
	})
	if err != nil {
		t.Fatalf("Install with interactive prompt: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", r.calls)
	}
	if !slices.Contains(call, "-i") {
		t.Errorf("expected -i in interactive container run, got %v", call)
	}
}

func TestSkillInstallService_InteractivePrompt_EmptySelection(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	prompter := mockSkillPrompter{
		selected: nil,
	}
	svc := SkillInstallService{Report: nopReporter{}, Prompt: prompter}

	err := svc.Install(nil, SkillInstallOptions{
		Dir:         t.TempDir(),
		Interactive: true,
	})
	if err != nil {
		t.Fatalf("Install with empty selection: %v", err)
	}

	call := r.callContaining("--rm")
	if call != nil {
		t.Errorf("expected no container run when no skills selected, got %v", call)
	}
}

func TestSkillInstallService_ImageOverride(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	customImage := "node:22-slim"

	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         t.TempDir(),
		Image:       customImage,
		Interactive: false,
	})
	if err != nil {
		t.Fatalf("Install with custom image: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run call, got %v", r.calls)
	}
	if !slices.Contains(call, customImage) {
		t.Errorf("call missing custom image %q: %v", customImage, call)
	}
}

func TestSkillInstallService_ContainerFailure_ReportsError(t *testing.T) {
	r := &fakeRunner{status: 1, stdout: "installation failed"}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         t.TempDir(),
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected error on container failure")
	}
	if !strings.Contains(err.Error(), "skill installer failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_ExplicitSkillsAndAll_Fails(t *testing.T) {
	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         t.TempDir(),
		All:         true,
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected error when both explicit skill IDs and --all are specified")
	}
	if !strings.Contains(err.Error(), "cannot specify both skill IDs and --all") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_EmptySkillIDString_FallsBackToProjectConfig(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	tempDir := t.TempDir()
	configData := types.DevcontainerConfig{
		Workspace: "testws",
		Skills: types.SkillsConfig{
			Skills: []types.SkillID{"firecrawl"},
		},
	}
	rawConfig, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, types.ConfigFile), rawConfig, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc := SkillInstallService{Report: nopReporter{}}
	if err := svc.Install([]string{""}, SkillInstallOptions{Dir: tempDir, Interactive: false}); err != nil {
		t.Fatalf("Install with empty skill ID string: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", r.calls)
	}
	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "firecrawl") {
		t.Errorf("expected project config skill firecrawl to be installed, got %s", lastArg)
	}
}

func TestSkillInstallService_InvalidTargetDir_Fails(t *testing.T) {
	svc := SkillInstallService{Report: nopReporter{}}
	nonExistentDir := filepath.Join(t.TempDir(), "does-not-exist")
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         nonExistentDir,
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected error for non-existent target directory")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_CommaSeparatedSkills_Parsed(t *testing.T) {
	r := &fakeRunner{status: 0}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	tempDir := t.TempDir()

	err := svc.Install([]string{"firecrawl,wayfinder"}, SkillInstallOptions{
		Dir:         tempDir,
		Interactive: false,
	})
	if err != nil {
		t.Fatalf("Install with comma-separated skills: %v", err)
	}

	call := r.callContaining("--rm")
	if call == nil {
		t.Fatalf("expected docker run --rm call, got %v", r.calls)
	}
	lastArg := call[len(call)-1]
	if !strings.Contains(lastArg, "firecrawl") || !strings.Contains(lastArg, "wayfinder") {
		t.Errorf("expected both skills in command, got %s", lastArg)
	}
}

func TestSkillInstallService_ContainerFailure_EmptyOutput_ReportsStatus(t *testing.T) {
	r := &fakeRunner{status: 1, stdout: ""}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         t.TempDir(),
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected error on container failure")
	}
	if !strings.Contains(err.Error(), "skill installer failed with exit status 1") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_ProjectConfig_UnknownSkill_Fails(t *testing.T) {
	tempDir := t.TempDir()
	configData := types.DevcontainerConfig{
		Workspace: "testws",
		Skills: types.SkillsConfig{
			Skills: []types.SkillID{"unknown-skill-xyz"},
		},
	}
	rawConfig, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, types.ConfigFile), rawConfig, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc := SkillInstallService{Report: nopReporter{}}
	err = svc.Install(nil, SkillInstallOptions{Dir: tempDir, Interactive: false})
	if err == nil {
		t.Fatal("expected error for unknown skill in project config")
	}
	if !strings.Contains(err.Error(), "unknown skill in devcontainer.config.json: unknown-skill-xyz") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_ProjectConfig_WhitespaceOnlySkills_FallsBackToActionableError(t *testing.T) {
	tempDir := t.TempDir()
	configData := types.DevcontainerConfig{
		Workspace: "testws",
		Skills: types.SkillsConfig{
			Skills: []types.SkillID{"  ", ""},
		},
	}
	rawConfig, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, types.ConfigFile), rawConfig, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc := SkillInstallService{Report: nopReporter{}}
	err = svc.Install(nil, SkillInstallOptions{Dir: tempDir, Interactive: false})
	if err == nil {
		t.Fatal("expected actionable error when project config has only whitespace skills")
	}
	if !strings.Contains(err.Error(), "no skills specified") || !strings.Contains(err.Error(), "--all") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_TargetDirIsFile_Fails(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "regular-file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{Dir: filePath, Interactive: false})
	if err == nil {
		t.Fatal("expected error when target dir is a regular file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_InteractivePrompt_PromptError(t *testing.T) {
	prompter := mockSkillPrompter{
		err: fmt.Errorf("user aborted prompt"),
	}
	svc := SkillInstallService{Report: nopReporter{}, Prompt: prompter}
	err := svc.Install(nil, SkillInstallOptions{Dir: t.TempDir(), Interactive: true})
	if err == nil {
		t.Fatal("expected error when prompt fails")
	}
	if !strings.Contains(err.Error(), "user aborted prompt") {
		t.Errorf("unexpected error message: %v", err)
	}
}

type recordingReporter struct {
	infos []string
}

func (r *recordingReporter) Info(format string, args ...any) {
	r.infos = append(r.infos, fmt.Sprintf(format, args...))
}
func (recordingReporter) Warn(string, ...any)    {}
func (recordingReporter) Success(string, ...any) {}
func (recordingReporter) Error(string, ...any)   {}
func (recordingReporter) Fatal(string, ...any)   {}
func (recordingReporter) Debug(string, ...any)   {}

func TestSkillInstallService_NonInteractiveOutput_FormattedSafely(t *testing.T) {
	r := &fakeRunner{status: 0, stdout: "Progress: 100% complete with %s %d verbs"}
	restore := useFakeDocker(r)
	defer restore()

	rec := &recordingReporter{}
	svc := SkillInstallService{Report: rec}
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{Dir: t.TempDir(), Interactive: false})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	found := false
	for _, info := range rec.infos {
		if strings.Contains(info, "100% complete with %s %d verbs") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected safely formatted output, got: %v", rec.infos)
	}
}

type unavailableDockerRunner struct{}

func (unavailableDockerRunner) Run(_ context.Context, _ []string, _ string, _ string, _ map[string]string) (int, string, string) {
	return 1, "", "docker daemon not running"
}

func TestSkillInstallService_DockerUnavailable_Fails(t *testing.T) {
	docker.SetRunner(unavailableDockerRunner{})
	docker.ResetDockerCache()
	defer func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	}()

	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         t.TempDir(),
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected error when docker is unavailable")
	}
	if !strings.Contains(err.Error(), "docker is not installed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_InteractiveContainerFailure_Fails(t *testing.T) {
	r := &fakeRunner{status: 2}
	restore := useFakeDocker(r)
	defer restore()

	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install([]string{"firecrawl"}, SkillInstallOptions{
		Dir:         t.TempDir(),
		Interactive: true,
	})
	if err == nil {
		t.Fatal("expected error when interactive container fails")
	}
	if !strings.Contains(err.Error(), "skill installer failed with exit status 2") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_ProjectConfig_MalformedJSON_Fails(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, types.ConfigFile), []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}

	svc := SkillInstallService{Report: nopReporter{}}
	err := svc.Install(nil, SkillInstallOptions{Dir: tempDir, Interactive: false})
	if err == nil {
		t.Fatal("expected error when devcontainer.config.json has malformed json")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillInstallService_ProjectConfig_EmptySkillsField_FallsBackToActionableError(t *testing.T) {
	tempDir := t.TempDir()
	configData := types.DevcontainerConfig{
		Workspace: "testws",
	}
	rawConfig, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, types.ConfigFile), rawConfig, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc := SkillInstallService{Report: nopReporter{}}
	err = svc.Install(nil, SkillInstallOptions{Dir: tempDir, Interactive: false})
	if err == nil {
		t.Fatal("expected actionable error when project config has empty skills field")
	}
	if !strings.Contains(err.Error(), "no skills specified") || !strings.Contains(err.Error(), "--all") {
		t.Errorf("unexpected error message: %v", err)
	}
}
