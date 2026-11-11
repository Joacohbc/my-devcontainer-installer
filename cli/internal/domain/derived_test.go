package domain_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func derivedConfig() *types.DevcontainerConfig {
	return &types.DevcontainerConfig{
		Mode:      types.BuildModeCustom,
		Workspace: "image",
		Dockerfile: types.DockerfileConfig{
			From:        "base",
			BaseRef:     "devcontainer-cli-img/base:latest",
			BaseModules: []types.SelectedModule{{ID: types.ModuleGithubCli}, {ID: types.ModuleNodejs}},
			Modules:     []types.SelectedModule{{ID: types.ModuleAndroidSdk}},
		},
	}
}

// A derived build starts FROM its parent and installs only what the parent
// lacks: the parent's modules and the always-on base layers are not rebuilt.
func TestGenerateDockerfile_DerivedBuildRendersOnlyNewModules(t *testing.T) {
	df, err := domain.GenerateDockerfile(derivedConfig())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(df, "FROM ubuntu") {
		t.Error("a derived build must not start from ubuntu")
	}
	if !strings.Contains(df, "FROM devcontainer-cli-img/base:latest") {
		t.Error("expected FROM the parent image")
	}
	for _, inherited := range []string{"+ NODE (devuser)", "useradd", "## GITHUB"} {
		if strings.Contains(df, inherited) {
			t.Errorf("inherited content %q was rebuilt", inherited)
		}
	}
	for _, added := range []string{"## ANDROID SDK", "## JAVA (Temurin)", "## CLEANUP & ENTRYPOINT"} {
		if !strings.Contains(df, added) {
			t.Errorf("expected %q (new module, its requirement, and the re-rendered cleanup)", added)
		}
	}
	if strings.Index(df, "## JAVA (Temurin)") > strings.Index(df, "## ANDROID SDK") {
		t.Error("the JDK must be installed before the Android SDK")
	}
}

// The environment and the modules label cover the whole lineage, so the ENV
// block keeps the parent's PATH and a grandchild knows what is installed.
func TestGenerateDockerfile_DerivedBuildKeepsTheFullEnvironmentAndLabel(t *testing.T) {
	df, err := domain.GenerateDockerfile(derivedConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(df, "PNPM_HOME") && !strings.Contains(df, "ANDROID_HOME") {
		t.Fatal("expected environment blocks")
	}
	if !strings.Contains(df, "ANDROID_HOME") {
		t.Error("missing the new module's environment")
	}
	if !strings.Contains(df, types.LabelModules+`="github-cli,nodejs,android-sdk"`) {
		t.Errorf("modules label must list the whole lineage:\n%s", df[strings.LastIndex(df, "LABEL"):])
	}
}

func TestResolveImageParentFollowsDefinitionChain(t *testing.T) {
	dir := t.TempDir()
	for _, def := range []domain.ImageDef{
		{Name: "base", Source: domain.ImageSourceBuild, Modules: []types.SelectedModule{{ID: types.ModuleGithubCli}}},
		{Name: "jvm", Source: domain.ImageSourceBuild, From: "base", Modules: []types.SelectedModule{{ID: types.ModuleJavaTemurin}}},
	} {
		if err := domain.SaveImageDef(def, dir); err != nil {
			t.Fatal(err)
		}
	}
	noDocker := func([]string) (int, string, string) { return 1, "", "" }
	parent, err := domain.ResolveImageParent("jvm", noDocker, dir)
	if err != nil {
		t.Fatal(err)
	}
	if parent.Ref != "devcontainer-cli-img/jvm:latest" {
		t.Errorf("ref = %s", parent.Ref)
	}
	if len(parent.Modules) != 2 || parent.Modules[0].ID != types.ModuleGithubCli || parent.Modules[1].ID != types.ModuleJavaTemurin {
		t.Errorf("modules = %+v", parent.Modules)
	}
}

func TestResolveImageParentRejectsCycles(t *testing.T) {
	dir := t.TempDir()
	for _, def := range []domain.ImageDef{
		{Name: "a", Source: domain.ImageSourceBuild, From: "b"},
		{Name: "b", Source: domain.ImageSourceBuild, From: "a"},
	} {
		if err := domain.SaveImageDef(def, dir); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := domain.ResolveImageParent("a", nil, dir); err == nil || !strings.Contains(err.Error(), "itself") {
		t.Errorf("expected a cycle error, got %v", err)
	}
}

func TestResolveImageParentReadsRegistryLabels(t *testing.T) {
	capture := func(args []string) (int, string, string) {
		return 0, "github-cli,python\n", ""
	}
	parent, err := domain.ResolveImageParent("ghcr.io/acme/devcontainer-python:1", capture, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(parent.Modules) != 2 || parent.Modules[1].ID != "python" {
		t.Errorf("modules = %+v", parent.Modules)
	}
	unlabelled := func([]string) (int, string, string) { return 0, "<no value>", "" }
	if _, err := domain.ResolveImageParent("ubuntu:24.04", unlabelled, t.TempDir()); err == nil {
		t.Error("an image without the modules label cannot be derived from")
	}
}

func TestGenerateDockerfile_DerivedBuildNeedsAResolvedBase(t *testing.T) {
	config := derivedConfig()
	config.Dockerfile.BaseRef = ""
	if _, err := domain.GenerateDockerfile(config); err == nil {
		t.Fatal("an unresolved base must fail, not build FROM an empty reference")
	}
}

func TestGenerateDockerfile_DerivedBuildRemapsDevuser(t *testing.T) {
	df, err := domain.GenerateDockerfile(derivedConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(df, "ARG USER_UID=1000") || !strings.Contains(df, `usermod -o -u "${USER_UID}"`) {
		t.Error("a derived build must remap devuser to the host ids")
	}
	// Paths outside the home (Python sysconfig dirs, /opt/android-sdk) were
	// chowned to the old uid by the parent and must follow the remap.
	if !strings.Contains(df, `-user "${old_uid}" -exec chown`) || !strings.Contains(df, `-group "${old_gid}" -exec chgrp`) {
		t.Error("the remap must re-own every path the parent gave devuser, not only the home")
	}
}

// The project's env (devcontainer.config.json `env`, filled by a profile's
// `env`) must reach the container, not only compose interpolation.
func TestGenerateCompose_ForwardsProjectEnvToTheDevcontainer(t *testing.T) {
	config := &types.DevcontainerConfig{
		Mode:      types.BuildModeCustom,
		Workspace: "ws",
		Image:     "img:1",
		Env:       map[string]string{"APP_ENV": "dev", "DOCKER_SUBNET": "10.0.0.0/28"},
	}
	yml, err := domain.GenerateCompose(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yml, "APP_ENV=${APP_ENV:-}") {
		t.Errorf("APP_ENV not forwarded:\n%s", yml)
	}
	if strings.Contains(yml, "DOCKER_SUBNET=${") {
		t.Error("compose-only variables must not be forwarded")
	}
}

func TestGenerateDockerfile_DerivedRejectsConflictingOptions(t *testing.T) {
	config := derivedConfig()
	config.Dockerfile.BaseModules = []types.SelectedModule{{ID: types.ModuleJavaTemurin, Options: map[string]any{"versions": []string{"17"}}}}
	config.Dockerfile.Modules = []types.SelectedModule{{ID: types.ModuleJavaTemurin, Options: map[string]any{"versions": []string{"21"}}}}
	if _, err := domain.GenerateDockerfile(config); err == nil || !strings.Contains(err.Error(), "other options") {
		t.Errorf("expected a conflicting-options error, got %v", err)
	}
}
