package service

import (
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

func captureReturning(stdout string) domain.CaptureFunc {
	return func([]string) (int, string, string) { return 0, stdout, "" }
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
			svc := GenerateService{Report: NopReporter{}, Capture: captureReturning(c.imagesOut)}
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
	config := &types.DevcontainerConfig{
		Mode:      types.BuildModeRemote,
		Image:     "ghcr.io/owner/devcontainer-ssh:latest",
		Workspace: "ws",
		Remote:    &types.RemoteConfig{Variant: "ssh"},
		Env:       map[string]string{},
	}
	svc := GenerateService{Report: NopReporter{}, Capture: captureReturning("")}
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
			var gotArgs []string
			svc := GenerateService{
				Report: NopReporter{},
				Compose: func(_ string, args []string) int {
					gotArgs = args
					return c.status
				},
			}
			err := svc.Build("compose.yml", c.remote)
			if (err != nil) != c.wantErr {
				t.Errorf("err = %v, wantErr %v", err, c.wantErr)
			}
			if len(gotArgs) != 1 || gotArgs[0] != c.wantArg {
				t.Errorf("compose args = %v, want [%s]", gotArgs, c.wantArg)
			}
		})
	}
}
