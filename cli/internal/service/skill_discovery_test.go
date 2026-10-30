package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/skillsdiscovery"
)

type fakeDiscoveryRunner struct {
	payload *skillsdiscovery.DiscoveredRaw
	err     error
	source  string
}

func (f *fakeDiscoveryRunner) DiscoverSkills(source string) (*skillsdiscovery.DiscoveredRaw, error) {
	f.source = source
	if f.err != nil {
		return nil, f.err
	}
	return f.payload, nil
}

type testReporter struct {
	infos    []string
	warns    []string
	successs []string
	errors   []string
}

func (r *testReporter) Info(format string, args ...any) {
	r.infos = append(r.infos, fmt.Sprintf(format, args...))
}

func (r *testReporter) Warn(format string, args ...any) {
	r.warns = append(r.warns, fmt.Sprintf(format, args...))
}

func (r *testReporter) Success(format string, args ...any) {
	r.successs = append(r.successs, fmt.Sprintf(format, args...))
}

func (r *testReporter) Error(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *testReporter) Fatal(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *testReporter) Debug(string, ...any) {}

func TestConfigService_DiscoverSkills(t *testing.T) {
	sampleJSON := `[
		{
			"name": "pdf",
			"status": "installed",
			"source": "anthropics/skills",
			"path": "/tmp/.claude/skills/pdf",
			"security": { "snyk": "low" }
		},
		{
			"name": "broken",
			"status": "skipped",
			"source": "anthropics/skills"
		}
	]`
	sampleMD := []byte(`---
name: pdf
description: Read and parse PDF files.
---
# PDF docs`)

	runner := &fakeDiscoveryRunner{
		payload: &skillsdiscovery.DiscoveredRaw{
			RawJSON: []byte(sampleJSON),
			SkillMDs: map[string][]byte{
				"/tmp/.claude/skills/pdf": sampleMD,
			},
		},
	}
	rep := &testReporter{}
	svc := ConfigService{
		Report:          rep,
		DiscoveryRunner: runner,
	}

	discovered, err := svc.DiscoverSkills("anthropics/skills")
	if err != nil {
		t.Fatalf("DiscoverSkills: %v", err)
	}

	if runner.source != "anthropics/skills" {
		t.Errorf("runner source = %q, want %q", runner.source, "anthropics/skills")
	}

	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered skill, got %d", len(discovered))
	}
	if discovered[0].Name != "pdf" || string(discovered[0].ID) != "pdf" {
		t.Errorf("unexpected discovered skill: %+v", discovered[0])
	}
	if discovered[0].Description != "Read and parse PDF files." {
		t.Errorf("unexpected description: %q", discovered[0].Description)
	}

	// Skipped item should be warned
	if len(rep.warns) != 1 || !strings.Contains(rep.warns[0], "broken") {
		t.Errorf("expected warning for broken skill, got %v", rep.warns)
	}
}

func TestConfigService_DiscoverSkills_NoSkillsFound(t *testing.T) {
	runner := &fakeDiscoveryRunner{
		payload: &skillsdiscovery.DiscoveredRaw{
			RawJSON: []byte(`[]`),
		},
	}
	svc := ConfigService{
		DiscoveryRunner: runner,
	}
	_, err := svc.DiscoverSkills("empty/repo")
	if err == nil {
		t.Fatal("expected error when no skills found")
	}
	if !strings.Contains(err.Error(), "no skills found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigService_DiscoverSkills_RunnerError(t *testing.T) {
	runner := &fakeDiscoveryRunner{
		err: fmt.Errorf("requires Node >= 22.20 on the host"),
	}
	svc := ConfigService{
		DiscoveryRunner: runner,
	}
	_, err := svc.DiscoverSkills("some/repo")
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
	if !strings.Contains(err.Error(), "requires Node >= 22.20") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigService_DiscoverSkills_SourceStartingWithDash(t *testing.T) {
	svc := ConfigService{}
	_, err := svc.DiscoverSkills("-flag-injection")
	if err == nil {
		t.Fatal("expected error for source starting with '-'")
	}
	if !strings.Contains(err.Error(), "cannot start with '-'") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConfigService_SaveUserSkills(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rep := &testReporter{}
	svc := ConfigService{Report: rep}

	skillsToSave := []*domain.DiscoveredSkill{
		{
			ID:          "my-discovered-skill",
			Name:        "My Discovered Skill",
			Description: "A great skill",
			Source:      "owner/repo",
			Manifest: types.UserSkillManifest{
				ID:    "my-discovered-skill",
				Label: "My Discovered Skill",
				Ref:   "owner/repo",
				Skill: "My Discovered Skill",
				Context: &types.UserSkillContext{
					Title: "My Discovered Skill",
					Body:  "A great skill",
				},
			},
		},
	}

	if err := svc.SaveUserSkills(skillsToSave, false); err != nil {
		t.Fatalf("first SaveUserSkills failed: %v", err)
	}

	filePath := filepath.Join(domain.SkillDir(), "my-discovered-skill.yml")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected skill file to exist at %s: %v", filePath, err)
	}

	loaded := catalog.LoadUserSkills(domain.SkillDir())
	if len(loaded) != 1 || string(loaded[0].ID) != "my-discovered-skill" {
		t.Fatalf("unexpected loaded user skills: %+v", loaded)
	}

	// Saving again without force should skip with warning
	rep.warns = nil
	if err := svc.SaveUserSkills(skillsToSave, false); err != nil {
		t.Fatalf("second SaveUserSkills (skip) failed: %v", err)
	}
	if len(rep.warns) != 1 || !strings.Contains(rep.warns[0], "already exists; skipping") {
		t.Errorf("expected skip warning, got %v", rep.warns)
	}

	// Saving with force should overwrite
	if err := svc.SaveUserSkills(skillsToSave, true); err != nil {
		t.Fatalf("third SaveUserSkills (force) failed: %v", err)
	}
}

func TestConfigService_SaveUserSkill_ShadowsBuiltin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rep := &testReporter{}
	svc := ConfigService{Report: rep}

	manifest := types.UserSkillManifest{
		ID:    "pdf",
		Ref:   "custom/pdf",
		Label: "Custom PDF",
	}

	if err := svc.SaveUserSkill(manifest, false); err != nil {
		t.Fatalf("SaveUserSkill failed: %v", err)
	}

	hasBuiltinNote := false
	for _, info := range rep.infos {
		if strings.Contains(info, "is also a built-in skill id; yours takes precedence") {
			hasBuiltinNote = true
			break
		}
	}
	if !hasBuiltinNote {
		t.Errorf("expected builtin precedence note, got %v", rep.infos)
	}
}
