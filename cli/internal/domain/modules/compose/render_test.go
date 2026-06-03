package compose_test

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/compose"
)

func TestPostgresRender(t *testing.T) {
	def := compose.PostgresService.Render(compose.RenderContext{})
	if def == nil {
		t.Fatal("expected a service definition")
	}
	if def.Image != "postgres:17-alpine" {
		t.Errorf("default image = %q, want postgres:17-alpine", def.Image)
	}
	if def.ContainerName != "postgres" {
		t.Errorf("ContainerName = %q, want postgres", def.ContainerName)
	}
	env, ok := def.Environment.(map[string]string)
	if !ok || env["POSTGRES_DB"] != "devdb" {
		t.Errorf("expected POSTGRES_DB=devdb, got %+v", def.Environment)
	}
	if env["POSTGRES_USER"] != "devuser" || env["POSTGRES_PASSWORD"] != "devpass" {
		t.Errorf("expected default devuser/devpass, got %s/%s", env["POSTGRES_USER"], env["POSTGRES_PASSWORD"])
	}

	custom := compose.PostgresService.Render(compose.RenderContext{Options: map[string]any{"version": "16-alpine"}})
	if custom.Image != "postgres:16-alpine" {
		t.Errorf("custom image = %q, want postgres:16-alpine", custom.Image)
	}

	customCreds := compose.PostgresService.Render(compose.RenderContext{
		DefaultDBUser:     "alice",
		DefaultDBPassword: "password123",
	})
	envCustom, ok := customCreds.Environment.(map[string]string)
	if !ok || envCustom["POSTGRES_USER"] != "alice" || envCustom["POSTGRES_PASSWORD"] != "password123" {
		t.Errorf("expected custom alice/password123, got %s/%s", envCustom["POSTGRES_USER"], envCustom["POSTGRES_PASSWORD"])
	}
}

func TestMongoRender(t *testing.T) {
	def := compose.MongoService.Render(compose.RenderContext{})
	if def == nil {
		t.Fatal("expected a service definition")
	}
	env, ok := def.Environment.(map[string]string)
	if !ok || env["MONGO_INITDB_ROOT_USERNAME"] != "devuser" || env["MONGO_INITDB_ROOT_PASSWORD"] != "devpass" {
		t.Errorf("expected default devuser/devpass, got %+v", def.Environment)
	}

	customCreds := compose.MongoService.Render(compose.RenderContext{
		DefaultDBUser:     "bob",
		DefaultDBPassword: "secretpassword",
	})
	envCustom, ok := customCreds.Environment.(map[string]string)
	if !ok || envCustom["MONGO_INITDB_ROOT_USERNAME"] != "bob" || envCustom["MONGO_INITDB_ROOT_PASSWORD"] != "secretpassword" {
		t.Errorf("expected custom bob/secretpassword, got %+v", customCreds.Environment)
	}
}

func TestRedisRender(t *testing.T) {
	def := compose.RedisService.Render(compose.RenderContext{})
	if def.Image != "redis:7.4-alpine" {
		t.Errorf("default image = %q, want redis:7.4-alpine", def.Image)
	}
	custom := compose.RedisService.Render(compose.RenderContext{Options: map[string]any{"version": "8.0-alpine"}})
	if custom.Image != "redis:8.0-alpine" {
		t.Errorf("custom image = %q, want redis:8.0-alpine", custom.Image)
	}
}

func TestDevcontainerRender_DependsOnEnabledDatabases(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName:         "myimg:local",
		EnabledServiceIDs: []string{"devcontainer", "postgres", "redis", "tmux"},
	})
	if def.Image != "myimg:local" {
		t.Errorf("Image = %q, want myimg:local", def.Image)
	}
	want := map[string]bool{"postgres": true, "redis": true}
	if len(def.DependsOn) != len(want) {
		t.Fatalf("DependsOn = %v, want %v", def.DependsOn, want)
	}
	for _, d := range def.DependsOn {
		if !want[d] {
			t.Errorf("unexpected depends_on entry %q", d)
		}
	}
}

func TestDevcontainerRender_PersistVolumeMounts(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName:           "img",
		EnabledServiceIDs:   []string{"devcontainer"},
		PersistVolumeMounts: []string{"devcontainer_etc:/etc", "devcontainer_home:/home"},
	})
	want := []string{"../..:/workspace", "devcontainer_etc:/etc", "devcontainer_home:/home"}
	if len(def.Volumes) != len(want) {
		t.Fatalf("Volumes = %v, want %v", def.Volumes, want)
	}
	for i, v := range want {
		if def.Volumes[i] != v {
			t.Errorf("Volumes[%d] = %q, want %q", i, def.Volumes[i], v)
		}
	}
}

func TestDevcontainerRender_NoPersistVolumesKeepsWorkspace(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName:         "img",
		EnabledServiceIDs: []string{"devcontainer"},
	})
	if len(def.Volumes) != 1 || def.Volumes[0] != "../..:/workspace" {
		t.Errorf("expected only the workspace mount, got %v", def.Volumes)
	}
}

func TestDevcontainerRender_NoDatabasesNoDependsOn(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName:         "img",
		EnabledServiceIDs: []string{"devcontainer"},
	})
	if def.DependsOn != nil {
		t.Errorf("expected no depends_on without databases, got %v", def.DependsOn)
	}
}

// Every compose service must render a definition with a container name and must
// not panic on an empty render context.
func TestAllComposeServicesRender(t *testing.T) {
	for _, s := range catalog.ComposeServices {
		if s.Render == nil {
			continue
		}
		def := s.Render(compose.RenderContext{})
		if def == nil {
			t.Errorf("service %q rendered nil", s.ID)
			continue
		}
		if def.ContainerName == "" {
			t.Errorf("service %q rendered without a container name", s.ID)
		}
	}
}
