package compose

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// Every service that declares a Context must produce a placeable section for a
// bare RenderContext: a title with no body renders as a dangling heading.
func TestServiceContextSectionsAreWellFormed(t *testing.T) {
	for _, svc := range []*ServiceSpec{DevcontainerService, PostgresService, RedisService, MongoService} {
		if svc.Context == nil {
			t.Errorf("service %q declares no Context; an agent cannot discover it", svc.ID)
			continue
		}
		sec := svc.Context(RenderContext{})
		if sec == nil {
			t.Errorf("service %q: nil context section for the default RenderContext", svc.ID)
			continue
		}
		if strings.TrimSpace(sec.Title) == "" || strings.TrimSpace(sec.Body) == "" {
			t.Errorf("service %q: context section needs both a title and a body", svc.ID)
		}
	}
}

// The credentials printed in the document must be the ones the service was
// rendered with, including the same fallbacks — an agent that reads a different
// password than the one in the compose file cannot connect.
func TestDatabaseContextMatchesRenderedCredentials(t *testing.T) {
	cases := []struct {
		name string
		svc  *ServiceSpec
		ctx  RenderContext
		envs []string
	}{
		{
			name: "postgres explicit credentials",
			svc:  PostgresService,
			ctx:  RenderContext{DefaultDBUser: "alice", DefaultDBPassword: "s3cret"},
			envs: []string{"POSTGRES_USER", "POSTGRES_PASSWORD"},
		},
		{
			name: "postgres falls back to defaults",
			svc:  PostgresService,
			ctx:  RenderContext{},
			envs: []string{"POSTGRES_USER", "POSTGRES_PASSWORD"},
		},
		{
			name: "mongo explicit credentials",
			svc:  MongoService,
			ctx:  RenderContext{DefaultDBUser: "bob", DefaultDBPassword: "hunter2"},
			envs: []string{"MONGO_INITDB_ROOT_USERNAME", "MONGO_INITDB_ROOT_PASSWORD"},
		},
		{
			name: "mongo falls back to defaults",
			svc:  MongoService,
			ctx:  RenderContext{},
			envs: []string{"MONGO_INITDB_ROOT_USERNAME", "MONGO_INITDB_ROOT_PASSWORD"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			def := c.svc.Render(c.ctx)
			env, ok := def.Environment.(map[string]string)
			if !ok {
				t.Fatalf("expected a map environment, got %T", def.Environment)
			}
			body := c.svc.Context(c.ctx).Body
			for _, key := range c.envs {
				want := env[key]
				if want == "" {
					t.Fatalf("rendered service has no %s", key)
				}
				if !strings.Contains(body, "`"+want+"`") {
					t.Errorf("context must quote the rendered %s (%q):\n%s", key, want, body)
				}
			}
		})
	}
}

// The version an agent reads must be the image tag the service actually runs.
func TestDatabaseContextMentionsRenderedImage(t *testing.T) {
	cases := []struct {
		svc     *ServiceSpec
		options map[string]any
		want    string
	}{
		{PostgresService, map[string]any{"version": "16-alpine"}, "postgres:16-alpine"},
		{RedisService, map[string]any{"version": "8.0-alpine"}, "redis:8.0-alpine"},
		{MongoService, map[string]any{"version": "7.0"}, "mongo:7.0"},
	}
	for _, c := range cases {
		ctx := RenderContext{Options: c.options}
		if got := c.svc.Render(ctx).Image; got != c.want {
			t.Fatalf("service %q rendered image %q, want %q", c.svc.ID, got, c.want)
		}
		if body := c.svc.Context(ctx).Body; !strings.Contains(body, c.want) {
			t.Errorf("service %q context must mention image %q:\n%s", c.svc.ID, c.want, body)
		}
	}
}

// Database services must steer the agent away from localhost — the single most
// common wrong assumption when a database is a sibling container.
func TestDatabaseContextWarnsAgainstLocalhost(t *testing.T) {
	for _, svc := range []*ServiceSpec{PostgresService, RedisService, MongoService} {
		body := svc.Context(RenderContext{}).Body
		if !strings.Contains(body, "not** on") || !strings.Contains(body, "`localhost`") {
			t.Errorf("service %q must warn that it is not on localhost:\n%s", svc.ID, body)
		}
	}
}

// The devcontainer section is what tells an agent whether a server it starts is
// reachable from the host, so both branches must be unambiguous.
func TestDevcontainerContextReportsPorts(t *testing.T) {
	none := DevcontainerService.Context(RenderContext{}).Body
	if !strings.Contains(none, "No ports are published") {
		t.Errorf("with no ports the section must say so:\n%s", none)
	}

	withPorts := DevcontainerService.Context(RenderContext{Ports: []string{"127.0.0.1:8080:80", "3000:3000"}}).Body
	for _, p := range []string{"127.0.0.1:8080:80", "3000:3000"} {
		if !strings.Contains(withPorts, p) {
			t.Errorf("published port %q must be listed:\n%s", p, withPorts)
		}
	}
	if strings.Contains(withPorts, "No ports are published") {
		t.Errorf("must not claim no ports are published when some are:\n%s", withPorts)
	}
}

// The shared-config paragraph must only appear when the volume is mounted:
// promising persistent logins in a container that opted out is a lie.
func TestDevcontainerContextSharedConfig(t *testing.T) {
	off := DevcontainerService.Context(RenderContext{}).Body
	if strings.Contains(off, types.SharedConfigMountPath) {
		t.Errorf("without the mount the section must not mention it:\n%s", off)
	}

	on := DevcontainerService.Context(RenderContext{SharedConfigMount: types.SharedConfigMount()}).Body
	if !strings.Contains(on, types.SharedConfigMountPath) {
		t.Errorf("with the mount the section must mention it:\n%s", on)
	}
	if !strings.Contains(on, types.SharedConfigAliasTarget) {
		t.Errorf("the shared-config paragraph must point at the user alias file:\n%s", on)
	}
}
