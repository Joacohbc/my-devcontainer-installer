package service

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// daemonRunner fakes a daemon holding the given images, all honouring the
// image contract. It records the Dockerfile handed to `docker build`.
type daemonRunner struct {
	mu         sync.Mutex
	local      map[string]bool
	calls      [][]string
	dockerfile string
	// modules answers the modules label of every image; empty means
	// "github-cli", what a definition created with no modules has.
	modules string
}

func (r *daemonRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, args)
	if len(args) < 2 {
		return 0, "", ""
	}
	switch args[1] {
	case "version":
		return 0, "27.0.0", ""
	case "images":
		if r.local[args[len(args)-1]] {
			return 0, "sha256:x", ""
		}
		return 0, "", ""
	case "inspect":
		format := args[3]
		switch {
		case strings.Contains(format, "Entrypoint"):
			return 0, `["/entrypoint.sh"]`, ""
		case strings.Contains(format, types.LabelManaged):
			return 0, "true", ""
		}
	case "image":
		if len(args) > 4 && strings.Contains(args[4], types.LabelModules) {
			if r.modules == "" {
				return 0, "github-cli", ""
			}
			return 0, r.modules, ""
		}
	case "build":
		if data, err := os.ReadFile(filepath.Join(args[len(args)-1], "Dockerfile")); err == nil {
			r.dockerfile = string(data)
		}
	}
	return 0, "", ""
}

func newImageSvc(t *testing.T) ImageService {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return ImageService{Report: nopReporter{}, ImagesDir: t.TempDir()}
}

func TestImageBuildDerivedStartsFromTheBuiltParent(t *testing.T) {
	svc := newImageSvc(t)
	if err := svc.Create("base", CreateImageOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Create("android", CreateImageOpts{From: "base", Modules: []string{"android-sdk"}}); err != nil {
		t.Fatal(err)
	}
	runner := &daemonRunner{local: map[string]bool{"devcontainer-cli-img/base:latest": true}}
	defer useFakeDocker(runner)()

	if err := svc.Build("android"); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(runner.dockerfile, "FROM devcontainer-cli-img/base:latest") {
		t.Errorf("derived Dockerfile does not start from the parent:\n%s", runner.dockerfile)
	}
	if strings.Contains(runner.dockerfile, "## GITHUB CLI") {
		t.Error("the parent's modules must not be rebuilt")
	}
	if !strings.Contains(runner.dockerfile, types.LabelKind+`="named"`) {
		t.Error("a named image must be labelled kind=named")
	}
}

// The parent must be built first; the error names the non-prompting command.
func TestImageBuildDerivedNeedsTheParentImage(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("base", CreateImageOpts{})
	_ = svc.Create("android", CreateImageOpts{From: "base", Modules: []string{"android-sdk"}})
	defer useFakeDocker(&daemonRunner{local: map[string]bool{}})()

	err := svc.Build("android")
	if err == nil || !strings.Contains(err.Error(), "devcontainer-cli image build base") {
		t.Fatalf("expected a hint to build the parent, got %v", err)
	}
}

func TestImageCreateRejectsDerivingFromItself(t *testing.T) {
	if err := newImageSvc(t).Create("loop", CreateImageOpts{From: "loop"}); err == nil {
		t.Fatal("expected an error")
	}
}

// --from-profile reads user profiles too, and copies their scripts next to
// image.yml so the definition is self-contained and shareable.
func TestImageCreateFromUserProfileCopiesScripts(t *testing.T) {
	svc := newImageSvc(t)
	profDir := filepath.Join(domain.GlobalConfigDir(), "profiles", "tools")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(profDir, "setup.sh"), []byte("#!/bin/sh\necho hi\n"), 0o755)
	_ = os.WriteFile(filepath.Join(profDir, "profile.yml"), []byte("id: tools\nmodules: [sqlite]\nscripts:\n  - file: setup.sh\n    when: build\n"), 0o644)

	if err := svc.Create("tools-img", CreateImageOpts{FromProfile: "tools"}); err != nil {
		t.Fatal(err)
	}
	def, ok, _ := domain.ResolveImageDef("tools-img", svc.ImagesDir)
	if !ok || !slices.ContainsFunc(def.Modules, func(m types.SelectedModule) bool { return m.ID == "sqlite" }) {
		t.Fatalf("def = %+v", def)
	}
	if data, err := os.ReadFile(filepath.Join(svc.ImagesDir, "tools-img", "setup.sh")); err != nil || !strings.Contains(string(data), "echo hi") {
		t.Errorf("script not copied into the image definition: %v", err)
	}

	runner := &daemonRunner{local: map[string]bool{}}
	defer useFakeDocker(runner)()
	if err := svc.Build("tools-img"); err != nil {
		t.Fatalf("Build with a copied script: %v", err)
	}
}

// A project deriving from a named image resolves the lineage at plan time, and
// the parent's content ID feeds the fingerprint: rebuilding the parent under
// the same tag gives the project a new image instead of a stale one.
func TestPlanDerivedProjectTracksTheParentImage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ImageService{Report: nopReporter{}}
	if err := svc.Create("android", CreateImageOpts{Modules: []string{"github-cli"}}); err != nil {
		t.Fatal(err)
	}
	plan := func(parentID string) GeneratePlan {
		runner := &parentIDRunner{daemonRunner: daemonRunner{local: map[string]bool{"devcontainer-cli-img/android:latest": true}}, id: parentID}
		defer useFakeDocker(runner)()
		config := &types.DevcontainerConfig{
			Mode: types.BuildModeCustom, Workspace: "app", Origin: "android-app",
			Dockerfile: types.DockerfileConfig{From: "android", Modules: []types.SelectedModule{{ID: types.ModuleNodejs}}},
		}
		p, err := GenerateService{Report: nopReporter{}}.Plan(config, nil)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		return p
	}
	first, second := plan("sha256:aaa"), plan("sha256:bbb")
	if !strings.Contains(first.Dockerfile, "FROM devcontainer-cli-img/android:latest") {
		t.Errorf("not derived:\n%s", first.Dockerfile)
	}
	if first.Image == second.Image {
		t.Error("a rebuilt parent must change the derived project's image")
	}
}

type parentIDRunner struct {
	daemonRunner
	id string
}

func (r *parentIDRunner) Run(ctx context.Context, args []string, a, b string, env map[string]string) (int, string, string) {
	if len(args) >= 5 && args[1] == "image" && args[2] == "inspect" && args[4] == "{{.Id}}" {
		return 0, r.id, ""
	}
	return r.daemonRunner.Run(ctx, args, a, b, env)
}

// CONTEXT.md is mounted, so it must describe what the image inherited too.
func TestContextConfigIncludesInheritedModules(t *testing.T) {
	derived := &types.DevcontainerConfig{
		Mode: types.BuildModeCustom,
		Dockerfile: types.DockerfileConfig{
			From: "x", BaseRef: "devcontainer-cli-img/x:latest",
			BaseModules: []types.SelectedModule{{ID: types.ModuleAndroidSdk}},
			Modules:     []types.SelectedModule{{ID: types.ModuleSqlite}},
		},
	}
	doc, err := domain.GenerateContext(contextConfig(derived))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, "Android SDK") {
		t.Error("a derived project's CONTEXT.md must describe the parent's modules")
	}

	profiles := &types.DevcontainerConfig{Mode: types.BuildModeProfiles, Workspace: "ws", Remote: &types.RemoteConfig{Variant: "python"}}
	doc, err = domain.GenerateContext(contextConfig(profiles))
	if err != nil || doc == "" || !strings.Contains(strings.ToLower(doc), "python") {
		t.Errorf("a prebuilt profile container needs a real CONTEXT.md (err=%v):\n%.300s", err, doc)
	}
}

// An image.yml edited without rebuilding no longer matches the local parent: a
// derived build would skip modules the parent lacks, so it must stop.
func TestImageBuildDerivedRejectsAnOutdatedParent(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("base", CreateImageOpts{Modules: []string{"github-cli", "python"}})
	_ = svc.Create("child", CreateImageOpts{From: "base", Modules: []string{"sqlite"}})
	defer useFakeDocker(&daemonRunner{local: map[string]bool{"devcontainer-cli-img/base:latest": true}, modules: "github-cli"})()

	err := svc.Build("child")
	if err == nil || !strings.Contains(err.Error(), "image build base") {
		t.Fatalf("expected an out-of-date parent error, got %v", err)
	}
}

// Options travel with the image: a registry parent built with non-default
// options hands them to the child's environment and context.
func TestResolveImageParentKeepsModuleOptions(t *testing.T) {
	capture := func(args []string) (int, string, string) {
		if strings.Contains(args[3], types.LabelModuleOptions) {
			return 0, `{"nodejs":{"manager":"nvm"}}`, ""
		}
		return 0, "github-cli,nodejs", ""
	}
	parent, err := domain.ResolveImageParent("ghcr.io/acme/devcontainer-node:1", capture, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if parent.Modules[1].Options["manager"] != "nvm" {
		t.Errorf("options lost: %+v", parent.Modules)
	}
}
