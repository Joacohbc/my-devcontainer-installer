package service

import (
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func localCachedConfig() *types.DevcontainerConfig {
	return &types.DevcontainerConfig{
		Mode:       types.BuildModeLocalCached,
		Workspace:  "ws",
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{{ID: "nodejs", Options: map[string]any{}}}},
		Compose:    types.ComposeConfig{Subnet: "172.30.0.0/16"},
		Env:        map[string]string{},
	}
}

func TestPlanLocalCachedFingerprintsAndDetectsCache(t *testing.T) {
	cases := []struct {
		name         string
		imagesOut    string
		wantCacheHit bool
	}{
		{"image present", "sha256abc", true},
		{"image absent", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			restore := useFakeDocker(&fakeRunner{status: 0, stdout: c.imagesOut})
			defer restore()

			svc := GenerateService{Report: nopReporter{}}
			plan, err := svc.Plan(localCachedConfig(), map[string]string{})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if plan.Dockerfile == "" {
				t.Error("expected a non-empty Dockerfile for local-cached")
			}
			if plan.Fingerprint == "" {
				t.Error("expected a fingerprint for local-cached")
			}
			if plan.Image != domain.FingerprintTag(plan.Fingerprint) {
				t.Errorf("Image %q != FingerprintTag(%q)", plan.Image, plan.Fingerprint)
			}
			if plan.CachedImageHit != c.wantCacheHit {
				t.Errorf("CachedImageHit = %v, want %v", plan.CachedImageHit, c.wantCacheHit)
			}
		})
	}
}

func TestPlanRemoteSkipsDockerfile(t *testing.T) {
	restore := useFakeDocker(&fakeRunner{status: 0})
	defer restore()

	config := &types.DevcontainerConfig{
		Mode:      types.BuildModeRemote,
		Image:     "ghcr.io/owner/devcontainer-nodejs:latest",
		Workspace: "ws",
		Remote:    &types.RemoteConfig{Variant: "nodejs"},
		Env:       map[string]string{},
	}
	svc := GenerateService{Report: nopReporter{}}
	plan, err := svc.Plan(config, map[string]string{})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Dockerfile != "" {
		t.Error("expected no Dockerfile in remote mode")
	}
	if plan.Fingerprint != "" {
		t.Error("expected no fingerprint in remote mode")
	}
	if plan.Image != config.Image {
		t.Errorf("Image = %q, want %q", plan.Image, config.Image)
	}
	if plan.CachedImageHit {
		t.Error("remote mode must not report a local cache hit")
	}
}

func TestBuildRunsComposeAndReportsErrors(t *testing.T) {
	cases := []struct {
		name    string
		remote  bool
		status  int
		wantArg string
		wantErr bool
	}{
		{"local build ok", false, 0, "build", false},
		{"remote pull ok", true, 0, "pull", false},
		{"build fails", false, 1, "build", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runner := &fakeRunner{status: c.status}
			restore := useFakeDocker(runner)
			defer restore()

			svc := GenerateService{Report: nopReporter{}}
			err := svc.Build("compose.yml", c.remote)
			if (err != nil) != c.wantErr {
				t.Errorf("err = %v, wantErr %v", err, c.wantErr)
			}
			call := runner.callContaining("compose")
			if call == nil || !slices.Contains(call, c.wantArg) {
				t.Errorf("compose call = %v, want it to contain %q", call, c.wantArg)
			}
		})
	}
}
