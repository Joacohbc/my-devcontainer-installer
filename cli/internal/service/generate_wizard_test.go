package service

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestConfigureReducesWizardAnswers(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeLocalCached),
		stepKeySubnet:    "172.45.0.0/16",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if cfg.Workspace != "testws" {
		t.Errorf("Workspace = %q, want testws", cfg.Workspace)
	}
	if cfg.Mode != types.BuildModeLocalCached {
		t.Errorf("Mode = %q, want local-cached", cfg.Mode)
	}
	if cfg.Compose.Subnet != "172.45.0.0/16" {
		t.Errorf("Subnet = %q, want 172.45.0.0/16", cfg.Compose.Subnet)
	}
}

func TestConfigureReducesPorts(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeLocalCached),
		stepKeySubnet:    "172.45.0.0/16",
		stepKeyPorts:     "8080:80, 5432:5432",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	want := []string{"8080:80", "5432:5432"}
	if len(cfg.Compose.Ports) != len(want) {
		t.Fatalf("Ports = %v, want %v", cfg.Compose.Ports, want)
	}
	for i, p := range want {
		if cfg.Compose.Ports[i] != p {
			t.Errorf("Ports[%d] = %q, want %q", i, cfg.Compose.Ports[i], p)
		}
	}
}

// The wizard no longer prompts for shared-config or persist volumes (those are
// flag/config-only now), so Configure must carry the base values through
// unchanged: a nil base stays enabled-by-default and an explicit opt-out survives.
func TestConfigureKeepsBaseSharedConfig(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	answers := map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeLocalCached),
		stepKeySubnet:    "172.45.0.0/16",
	}
	svc := GenerateService{Report: nopReporter{}}

	nilBase := &types.DevcontainerConfig{Env: map[string]string{}}
	cfg, err := svc.Configure(nilBase, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !types.SharedConfigEnabled(cfg) {
		t.Errorf("nil base should stay enabled by default, got %v", cfg.Compose.SharedConfig)
	}

	off := false
	optedOut := &types.DevcontainerConfig{Env: map[string]string{}, Compose: types.ComposeConfig{SharedConfig: &off}}
	cfg, err = svc.Configure(optedOut, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if cfg.Compose.SharedConfig == nil || *cfg.Compose.SharedConfig {
		t.Errorf("explicit opt-out must survive the wizard, got %v", cfg.Compose.SharedConfig)
	}
}

func TestVariantChoicesCoversRemoteVariants(t *testing.T) {
	choices := VariantChoices()
	if len(choices) != len(types.RemoteVariants) {
		t.Fatalf("got %d choices, want %d", len(choices), len(types.RemoteVariants))
	}
	for i, v := range types.RemoteVariants {
		if choices[i].Value != v {
			t.Errorf("choice[%d].Value = %q, want %q", i, choices[i].Value, v)
		}
	}
}

func TestServiceOptionsOf(t *testing.T) {
	services := []any{
		types.SelectedModule{ID: "postgres", Options: map[string]any{"version": "16"}},
	}
	if opts := ServiceOptionsOf(services, "postgres"); opts["version"] != "16" {
		t.Errorf("ServiceOptionsOf returned %v, want version=16", opts)
	}
	if opts := ServiceOptionsOf(services, "redis"); len(opts) != 0 {
		t.Errorf("ServiceOptionsOf for absent id = %v, want empty", opts)
	}
}
