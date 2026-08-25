package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestInitServiceLocalProjectDetection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	svc := InitService{
		Report: nopReporter{},
		Prompt: scriptedPrompter{},
	}

	opts := InitOptions{
		TargetDir: dir,
		NoBuild:   true,
		NoUp:      true,
	}

	cfg, err := svc.Init(opts)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if cfg == nil {
		t.Fatalf("expected non-nil config")
	}

	hasGolang := false
	for _, m := range cfg.Dockerfile.Modules {
		if m.ID == types.ModuleGolang {
			hasGolang = true
			break
		}
	}
	if !hasGolang {
		t.Errorf("expected golang module in generated config, got %v", cfg.Dockerfile.Modules)
	}

	cfgPath := filepath.Join(dir, "devcontainer.config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Errorf("expected devcontainer.config.json to exist at %s", cfgPath)
	}
}

func TestInitServiceSkillsDetection(t *testing.T) {
	fake := &fakeRunner{}
	restore := useFakeDocker(fake)
	defer restore()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills-lock.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	svc := InitService{
		Report: nopReporter{},
		Prompt: scriptedPrompter{},
	}

	opts := InitOptions{
		TargetDir: dir,
		NoBuild:   false,
		NoUp:      false,
	}

	cfg, err := svc.Init(opts)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	var moduleIDs []types.ModuleID
	for _, m := range cfg.Dockerfile.Modules {
		moduleIDs = append(moduleIDs, m.ID)
	}

	if !slices.Contains(moduleIDs, types.ModuleID("nodejs")) {
		t.Errorf("expected nodejs in %v", moduleIDs)
	}
	if !slices.Contains(moduleIDs, types.ModuleID("pnpm")) {
		t.Errorf("expected pnpm in %v", moduleIDs)
	}

	// Verify docker compose build and up were invoked
	if fake.callContaining("build") == nil {
		t.Errorf("expected docker compose build to be called")
	}
	if fake.callContaining("up") == nil {
		t.Errorf("expected docker compose up to be called")
	}
}
