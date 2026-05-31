package dockerfile_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
)

func TestBaseModuleRender(t *testing.T) {
	out := dockerfile.BaseModule.Render(nil)
	if !strings.Contains(out, "FROM ubuntu:24.04") {
		t.Errorf("default base should pin ubuntu 24.04:\n%s", out)
	}
	out = dockerfile.BaseModule.Render(map[string]any{"ubuntu": "22.04"})
	if !strings.Contains(out, "FROM ubuntu:22.04") {
		t.Errorf("base should honor ubuntu option:\n%s", out)
	}
}

func TestNodejsModuleRender(t *testing.T) {
	cases := []struct {
		name    string
		opts    map[string]any
		want    []string
		notWant []string
	}{
		{
			name: "default nvm lts",
			opts: nil,
			want: []string{"NVM", "nvm install --lts"},
		},
		{
			name: "nvm pinned version",
			opts: map[string]any{"manager": "nvm", "version": "22"},
			want: []string{"nvm install 22"},
		},
		{
			name:    "fnm manager",
			opts:    map[string]any{"manager": "fnm"},
			want:    []string{"fnm", "fnm install --lts"},
			notWant: []string{"nvm install"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := dockerfile.NodejsModule.Render(tc.opts)
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("expected output to contain %q:\n%s", w, out)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(out, nw) {
					t.Errorf("expected output NOT to contain %q:\n%s", nw, out)
				}
			}
		})
	}
}

func TestPythonModuleRender(t *testing.T) {
	withUv := dockerfile.PythonModule.Render(map[string]any{"uv": true})
	if !strings.Contains(withUv, "astral.sh/uv") {
		t.Errorf("expected uv install block when uv=true:\n%s", withUv)
	}
	withoutUv := dockerfile.PythonModule.Render(map[string]any{"uv": false})
	if strings.Contains(withoutUv, "astral.sh/uv") {
		t.Errorf("did not expect uv block when uv=false:\n%s", withoutUv)
	}
	if !strings.Contains(withoutUv, "python3") {
		t.Errorf("python module must always install python3:\n%s", withoutUv)
	}
}

func TestPhpModuleRender(t *testing.T) {
	defaultOut := dockerfile.PhpModule.Render(nil)
	if !strings.Contains(defaultOut, "php-cli") {
		t.Errorf("expected php-cli package to be installed:\n%s", defaultOut)
	}
	if !strings.Contains(defaultOut, "php-fpm") {
		t.Errorf("expected php-fpm package to be installed:\n%s", defaultOut)
	}
	if !strings.Contains(defaultOut, "getcomposer.org/installer") {
		t.Errorf("expected composer to be installed by default:\n%s", defaultOut)
	}

	noComposer := dockerfile.PhpModule.Render(map[string]any{"composer": false})
	if !strings.Contains(noComposer, "php-cli") {
		t.Errorf("expected php-cli package to be installed:\n%s", noComposer)
	}
	if strings.Contains(noComposer, "getcomposer.org/installer") {
		t.Errorf("expected composer NOT to be installed when composer=false:\n%s", noComposer)
	}
}

func TestRustModuleRender(t *testing.T) {
	out := dockerfile.RustModule.Render(nil)
	if !strings.Contains(out, "rustup.rs") {
		t.Errorf("expected rustup.rs install script:\n%s", out)
	}
	if !strings.Contains(out, "build-essential") {
		t.Errorf("expected build-essential dependencies:\n%s", out)
	}
	if !strings.Contains(out, ".rust_init.sh") {
		t.Errorf("expected rust shell init script:\n%s", out)
	}
}

func TestDatabaseClientModulesRender(t *testing.T) {
	cases := []struct {
		name   string
		module *dockerfile.ModuleSpec
		want   []string
	}{
		{"postgres", dockerfile.PostgresClientModule, []string{"POSTGRESQL CLIENT", "postgresql-client"}},
		{"redis", dockerfile.RedisClientModule, []string{"REDIS CLIENT", "redis-tools"}},
		{"mysql", dockerfile.MysqlClientModule, []string{"MYSQL CLIENT", "default-mysql-client"}},
		{"mongo", dockerfile.MongoClientModule, []string{"MONGODB CLIENT", "mongodb-mongosh", "repo.mongodb.org"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.module.Render(nil)
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("expected output to contain %q:\n%s", w, out)
				}
			}
		})
	}
}

// Every Dockerfile module must render a non-empty fragment with default options
// and must not panic on a nil options map.
func TestAllDockerfileModulesRenderNonEmpty(t *testing.T) {
	for _, m := range catalog.DockerfileModules {
		if m.Render == nil {
			continue
		}
		out := m.Render(nil)
		if strings.TrimSpace(out) == "" {
			t.Errorf("module %q rendered an empty fragment", m.ID)
		}
	}
}
