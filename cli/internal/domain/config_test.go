package domain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestLoadConfig_NotExistReturnsNil(t *testing.T) {
	cfg, err := domain.LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config for missing file, got %+v", cfg)
	}
}

func TestSaveAndLoadConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := domain.DefaultConfig(dir)
	want.Image = "myimg:local"
	want.Env = map[string]string{"FOO": "bar"}

	if err := domain.SaveConfig(want, dir); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, types.ConfigFile)); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	got, err := domain.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got == nil {
		t.Fatal("expected loaded config, got nil")
	}
	if got.Image != want.Image {
		t.Errorf("Image = %q, want %q", got.Image, want.Image)
	}
	if got.Env["FOO"] != "bar" {
		t.Errorf("Env not round-tripped: %+v", got.Env)
	}
	if got.Mode != types.BuildModeLocalCached {
		t.Errorf("Mode = %q, want %q", got.Mode, types.BuildModeLocalCached)
	}
}

func TestLoadConfig_MigratesLegacyModes(t *testing.T) {
	for _, legacy := range []string{"custom", "standalone", ""} {
		dir := t.TempDir()
		body := `{"mode":"` + legacy + `","image":"x:local","workspace":"ws"}`
		if err := os.WriteFile(filepath.Join(dir, types.ConfigFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := domain.LoadConfig(dir)
		if err != nil {
			t.Fatalf("LoadConfig(%q): %v", legacy, err)
		}
		if cfg.Mode != types.BuildModeLocalCached {
			t.Errorf("legacy mode %q migrated to %q, want %q", legacy, cfg.Mode, types.BuildModeLocalCached)
		}
	}
}

func TestLoadConfig_MigratesLegacyDbclients(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "explicit clients",
			body: `{"mode":"local-cached","image":"x:local","workspace":"ws","dockerfile":{"modules":[{"id":"dbclients","options":{"clients":["postgres","redis"]}}]}}`,
			want: []string{"postgres-client", "redis-client"},
		},
		{
			name: "missing clients option defaults to psql/redis/mongo",
			body: `{"mode":"local-cached","image":"x:local","workspace":"ws","dockerfile":{"modules":[{"id":"dbclients"}]}}`,
			want: []string{"postgres-client", "redis-client", "mongo-client"},
		},
		{
			name: "none selected drops the module",
			body: `{"mode":"local-cached","image":"x:local","workspace":"ws","dockerfile":{"modules":[{"id":"dbclients","options":{"clients":["none"]}}]}}`,
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, types.ConfigFile), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := domain.LoadConfig(dir)
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			var got []string
			for _, m := range cfg.Dockerfile.Modules {
				got = append(got, string(m.ID))
				if m.ID == "dbclients" {
					t.Errorf("legacy dbclients module survived migration")
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("modules = %v, want %v", got, tc.want)
			}
			for i, id := range tc.want {
				if got[i] != id {
					t.Errorf("module[%d] = %q, want %q", i, got[i], id)
				}
			}
		})
	}
}

func TestLoadConfig_KeepsRemoteMode(t *testing.T) {
	dir := t.TempDir()
	body := `{"mode":"remote","image":"x:local","workspace":"ws"}`
	if err := os.WriteFile(filepath.Join(dir, types.ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := domain.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Mode != types.BuildModeRemote {
		t.Errorf("Mode = %q, want remote", cfg.Mode)
	}
}

func TestLoadConfig_DefaultsWorkspaceFromDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "My Cool Project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"mode":"remote","image":"x:local"}`
	if err := os.WriteFile(filepath.Join(dir, types.ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := domain.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Workspace != "my-cool-project" {
		t.Errorf("Workspace = %q, want sanitized %q", cfg.Workspace, "my-cool-project")
	}
}

func TestLoadConfig_InvalidJSONErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, types.ConfigFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := domain.LoadConfig(dir); err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}

func TestDefaultConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := domain.DefaultConfig(dir)
	if cfg.Mode != types.BuildModeLocalCached {
		t.Errorf("Mode = %q, want local-cached", cfg.Mode)
	}
	if cfg.Workspace != "proj" {
		t.Errorf("Workspace = %q, want proj", cfg.Workspace)
	}
	if cfg.Compose.Subnet == "" {
		t.Error("expected a default subnet")
	}
	if cfg.Env == nil || cfg.Dockerfile.Modules == nil {
		t.Error("expected non-nil Env and Modules slices")
	}
}
