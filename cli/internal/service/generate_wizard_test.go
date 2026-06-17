package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
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

func TestConfigureReducesVolumes(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeLocalCached),
		stepKeySubnet:    "172.45.0.0/16",
		stepKeyVolumes:   "myvol:/data, ./cache:/cache",
	}}

	cfg, err := svc.Configure(base, "/home/user/proj", prompter)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	want := []string{"myvol:/data", "./cache:/cache"}
	if len(cfg.Compose.Volumes) != len(want) {
		t.Fatalf("Volumes = %v, want %v", cfg.Compose.Volumes, want)
	}
	for i, v := range want {
		if cfg.Compose.Volumes[i] != v {
			t.Errorf("Volumes[%d] = %q, want %q", i, cfg.Compose.Volumes[i], v)
		}
	}
}

// The wizard prompts for shared-config (defaulting to the base's current value),
// but when the step is left at its seeded default Configure must carry the base
// value through: a nil base stays enabled-by-default and an explicit opt-out survives.
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

// The shared-config prompt defaults to on, but an explicit answer must win in
// either direction: opting out from an enabled base, and opting in from an
// opted-out base.
func TestConfigureHonorsSharedConfigAnswer(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	base := map[string]any{
		stepKeyWorkspace: "testws",
		stepKeyMode:      string(types.BuildModeLocalCached),
		stepKeySubnet:    "172.45.0.0/16",
	}
	svc := GenerateService{Report: nopReporter{}}

	// Answering false on an enabled (nil) base opts out.
	answers := map[string]any{stepKeySharedConfig: false}
	for k, v := range base {
		answers[k] = v
	}
	cfg, err := svc.Configure(&types.DevcontainerConfig{Env: map[string]string{}}, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if types.SharedConfigEnabled(cfg) {
		t.Errorf("answering false must opt out, got %v", cfg.Compose.SharedConfig)
	}

	// Answering true on an opted-out base re-enables it.
	off := false
	answers = map[string]any{stepKeySharedConfig: true}
	for k, v := range base {
		answers[k] = v
	}
	optedOut := &types.DevcontainerConfig{Env: map[string]string{}, Compose: types.ComposeConfig{SharedConfig: &off}}
	cfg, err = svc.Configure(optedOut, "/home/user/proj", scriptedPrompter{answers: answers})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !types.SharedConfigEnabled(cfg) {
		t.Errorf("answering true must opt in, got %v", cfg.Compose.SharedConfig)
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

func TestConfigureAppliesUserPreset(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	presetsDir := filepath.Join(tmp, "devcontainer-cli", "presets")
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(presetsDir, "mystack.yml"),
		[]byte("id: mystack\nmodules: [claude-code, antigravity-cli]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{Env: map[string]string{}}
	prompter := scriptedPrompter{answers: map[string]any{
		stepKeyWorkspace: "ws",
		stepKeyMode:      string(types.BuildModeLocalCached),
		stepKeyPreset:    "mystack",
		stepKeySubnet:    "172.20.0.0/24",
	}}

	cfg, err := svc.Configure(base, tmp, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := map[string]bool{}
	for _, m := range cfg.Dockerfile.Modules {
		got[string(m.ID)] = true
	}
	for _, want := range []string{"claude-code", "antigravity-cli"} {
		if !got[want] {
			t.Errorf("expected preset module %q to be pre-selected, got %v", want, got)
		}
	}
}

func TestSelectModules(t *testing.T) {
	defer useFakeDocker(&fakeRunner{status: 0})()

	svc := GenerateService{Report: nopReporter{}}
	base := &types.DevcontainerConfig{}
	prompter := scriptedPrompter{answers: map[string]any{
		categoryStepKey(types.UICategoryAITools):  []string{"claude-code", "antigravity-cli"},
		categoryStepKey(types.UICategoryDevTools): []string{"chrome"},
	}}

	ids, err := svc.SelectModules(base, prompter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	for _, want := range []string{"claude-code", "antigravity-cli", "chrome"} {
		if !got[want] {
			t.Errorf("expected selected modules to contain %q, got %v", want, ids)
		}
	}
	if len(ids) != 3 {
		t.Errorf("expected exactly 3 selected modules, got %d: %v", len(ids), ids)
	}
	// No services must leak into the result (a preset is a pure module bundle).
	for _, id := range ids {
		if catalog.GetDockerfileModule(types.ModuleID(id)) == nil {
			t.Errorf("selected id %q is not a Dockerfile module", id)
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
