package compose_test

import (
	"strings"
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

// The version a service's option declares as its default must be the version
// Render falls back to when nothing is selected. Each service now routes both
// through one constant, and this is what keeps the wizard's preselected choice,
// the generated compose file and the ~/CONTEXT.md section from drifting apart if
// someone reintroduces a literal.
func TestDatabaseVersionOptionDefaultMatchesRenderedImage(t *testing.T) {
	cases := []struct {
		svc  *compose.ServiceSpec
		repo string
	}{
		{compose.PostgresService, "postgres"},
		{compose.MongoService, "mongo"},
		{compose.RedisService, "redis"},
	}
	for _, c := range cases {
		t.Run(c.repo, func(t *testing.T) {
			declared, ok := versionOptionDefault(c.svc)
			if !ok {
				t.Fatalf("%s declares no string default for its version option", c.repo)
			}
			def := c.svc.Render(compose.RenderContext{})
			if want := c.repo + ":" + declared; def.Image != want {
				t.Errorf("Render with no options = %q, want %q (the version option's declared default)", def.Image, want)
			}
			if !strings.Contains(c.svc.Context(compose.RenderContext{}).Body, declared) {
				t.Errorf("context section does not mention the default version %q", declared)
			}
		})
	}
}

// versionOptionDefault reads the declared default of a service's "version"
// option, reporting whether it exists and is a string.
func versionOptionDefault(svc *compose.ServiceSpec) (string, bool) {
	for _, opt := range svc.Options {
		if opt.ID != "version" {
			continue
		}
		value, ok := opt.Default.(string)
		return value, ok
	}
	return "", false
}

// Every choice a version option offers must be a usable image tag, so a value
// picked in the wizard can never render an image the registry does not have.
func TestDatabaseVersionChoicesIncludeTheDefault(t *testing.T) {
	for _, svc := range []*compose.ServiceSpec{compose.PostgresService, compose.MongoService, compose.RedisService} {
		declared, ok := versionOptionDefault(svc)
		if !ok {
			t.Fatalf("%s declares no string default for its version option", svc.ID)
		}
		found := false
		for _, opt := range svc.Options {
			if opt.ID != "version" {
				continue
			}
			for _, choice := range opt.Choices {
				if choice.Value == declared {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s default %q is not among its version choices", svc.ID, declared)
		}
	}
}

// depends_on is now assembled by the generator (from the catalog's IsDatabase
// flag), not by Render; the generator_test suite covers that. Render itself
// leaves DependsOn unset.
func TestDevcontainerRender_DoesNotSetDependsOn(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "myimg:local",
	})
	if def.Image != "myimg:local" {
		t.Errorf("Image = %q, want myimg:local", def.Image)
	}
	if def.DependsOn != nil {
		t.Errorf("DependsOn = %v, want nil (assembled by the generator)", def.DependsOn)
	}
}

// The main devcontainer service must not carry a restart policy: it should
// stop with the host/daemon like any other process, but never auto-start on
// the next daemon/system boot without the user explicitly starting it again.
func TestDevcontainerRender_NoRestartPolicy(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "img",
	})
	if def.Restart != "" {
		t.Errorf("Restart = %q, want empty (no auto-start on daemon/system boot)", def.Restart)
	}
}

func TestDevcontainerRender_NoPersistVolumesKeepsWorkspace(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "img",
	})
	if len(def.Volumes) != 1 || def.Volumes[0] != "../..:/workspace" {
		t.Errorf("expected only the workspace mount, got %v", def.Volumes)
	}
}

func TestDevcontainerRender_WorkspaceDir(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName:    "img",
		WorkspaceDir: "/workspaces/myproj",
	})
	if len(def.Volumes) == 0 || def.Volumes[0] != "../..:/workspaces/myproj" {
		t.Errorf("expected workspace mount at /workspaces/myproj, got %v", def.Volumes)
	}
}

func TestDevcontainerRender_WorkspaceDirDefaults(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "img",
	})
	if len(def.Volumes) == 0 || def.Volumes[0] != "../..:/workspace" {
		t.Errorf("empty WorkspaceDir should default to /workspace, got %v", def.Volumes)
	}
}

func TestDevcontainerRender_SharedConfigMount(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName:         "img",
		SharedConfigMount: "devcontainer-shared-config:/mnt/shared-config",
	})
	want := []string{"../..:/workspace", "devcontainer-shared-config:/mnt/shared-config"}
	if len(def.Volumes) != len(want) {
		t.Fatalf("Volumes = %v, want %v", def.Volumes, want)
	}
	for i, v := range want {
		if def.Volumes[i] != v {
			t.Errorf("Volumes[%d] = %q, want %q", i, def.Volumes[i], v)
		}
	}
}

func TestDevcontainerRender_NoSharedConfigByDefault(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "img",
	})
	for _, v := range def.Volumes {
		if v == "devcontainer-shared-config:/mnt/shared-config" {
			t.Errorf("shared-config mount must be absent without RenderContext.SharedConfigMount, got %v", def.Volumes)
		}
	}
}

func TestDevcontainerRender_Ports(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "img",
		Ports:     []string{"127.0.0.1:8080:80", "0.0.0.0:9090:90"},
	})
	want := []string{"127.0.0.1:8080:80", "0.0.0.0:9090:90"}
	if len(def.Ports) != len(want) {
		t.Fatalf("Ports = %v, want %v", def.Ports, want)
	}
	for i, p := range want {
		if def.Ports[i] != p {
			t.Errorf("Ports[%d] = %q, want %q", i, def.Ports[i], p)
		}
	}
}

func TestDevcontainerRender_NoPortsByDefault(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "img",
	})
	if len(def.Ports) != 0 {
		t.Errorf("expected no ports without RenderContext.Ports, got %v", def.Ports)
	}
}

func TestDevcontainerRender_NoDatabasesNoDependsOn(t *testing.T) {
	def := compose.DevcontainerService.Render(compose.RenderContext{
		ImageName: "img",
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
