package commands

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func writeUserProfile(t *testing.T, id, manifest string) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	dir := filepath.Join(configDir, "devcontainer-cli", "profiles", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profile.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func applyProfileFlag(t *testing.T, profile string, extra map[string]string) (*types.DevcontainerConfig, error) {
	t.Helper()
	cmd := NewRootCommand("dev")
	if err := cmd.Flags().Set("profile", profile); err != nil {
		t.Fatal(err)
	}
	for k, v := range extra {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	flags, err := parseGenFlags(cmd)
	if err != nil {
		return nil, err
	}
	config := domain.DefaultConfig(t.TempDir())
	return config, applyGenFlags(config, flags)
}

// A profile is a project template: its services, volumes and env reach the
// project config just like modules do.
func TestProfileTemplateAppliesServicesVolumesAndEnv(t *testing.T) {
	writeUserProfile(t, "web-db", `id: web-db
modules:
  - nodejs
services:
  - id: postgres
  - redis
volumes:
  - cache:/home/devuser/.cache
env:
  APP_ENV: development
`)
	config, err := applyProfileFlag(t, "web-db", nil)
	if err != nil {
		t.Fatal(err)
	}
	var ids []types.ServiceID
	for _, s := range config.Compose.Services {
		ids = append(ids, s.ID)
	}
	if !slices.Equal(ids, []types.ServiceID{types.ServicePostgres, "redis"}) {
		t.Errorf("services = %v", ids)
	}
	if !slices.Contains(config.Compose.Volumes, "cache:/home/devuser/.cache") {
		t.Errorf("volumes = %v", config.Compose.Volumes)
	}
	if config.Env["APP_ENV"] != "development" {
		t.Errorf("env = %v", config.Env)
	}
}

// Explicit flags win over the template, as they do over a profile's modules.
func TestProfileTemplateLosesToExplicitFlags(t *testing.T) {
	writeUserProfile(t, "web-db", `id: web-db
modules: [nodejs]
services: [postgres]
volumes: ["a:/a"]
`)
	config, err := applyProfileFlag(t, "web-db", map[string]string{"services": "redis", "volumes": "b:/b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Compose.Services) != 1 || config.Compose.Services[0].ID != "redis" {
		t.Errorf("services = %+v", config.Compose.Services)
	}
	if !slices.Equal(config.Compose.Volumes, []string{"b:/b"}) {
		t.Errorf("volumes = %v", config.Compose.Volumes)
	}
}

// A profile that starts from an image puts the project in mode=image.
func TestProfileTemplateWithImageUsesImageMode(t *testing.T) {
	writeUserProfile(t, "android-dev", `id: android-dev
image: ghcr.io/acme/devcontainer-android:1
`)
	config, err := applyProfileFlag(t, "android-dev", nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Mode != types.BuildModeImage || config.Image != "ghcr.io/acme/devcontainer-android:1" {
		t.Errorf("mode=%s image=%s", config.Mode, config.Image)
	}
}

func TestProfileTemplateRejectsUnknownService(t *testing.T) {
	writeUserProfile(t, "bad", `id: bad
modules: [nodejs]
services: [nope]
`)
	if _, err := applyProfileFlag(t, "bad", nil); err == nil {
		t.Fatal("expected an unknown service error")
	}
}
