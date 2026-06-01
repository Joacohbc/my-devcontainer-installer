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
	// chmod + run + rm of the zsh installer must be a single consolidated RUN so
	// the script is removed in the same layer it is used.
	if !strings.Contains(out, "RUN chmod +x /tmp/zsh-installer.sh && \\\n    su - devuser -c \"/tmp/zsh-installer.sh\" && \\\n    rm /tmp/zsh-installer.sh") {
		t.Errorf("zsh installer chmod/run/rm should be a single RUN:\n%s", out)
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
		name    string
		module  *dockerfile.ModuleSpec
		opts    map[string]any
		want    []string
		notWant []string
	}{
		// Default / auto (no version pin) -> generic Ubuntu packages.
		{"postgres generic", dockerfile.PostgresClientModule, nil, []string{"POSTGRESQL CLIENT", "postgresql-client"}, []string{"apt.postgresql.org"}},
		{"redis", dockerfile.RedisClientModule, nil, []string{"REDIS CLIENT", "redis-tools"}, nil},
		{"mysql generic", dockerfile.MysqlClientModule, nil, []string{"MYSQL CLIENT", "default-mysql-client"}, []string{"repo.mysql.com"}},
		{"mongo", dockerfile.MongoClientModule, nil, []string{"MONGODB CLIENT", "mongodb-mongosh", "repo.mongodb.org"}, nil},
		// Pinned versions -> vendor apt repos.
		{"postgres 16 (PGDG)", dockerfile.PostgresClientModule, map[string]any{"version": "16"},
			[]string{"postgresql-client-16", "apt.postgresql.org", "pgdg main"}, nil},
		{"mysql 8.4 (MySQL repo)", dockerfile.MysqlClientModule, map[string]any{"version": "8.4"},
			[]string{"mysql-community-client", "repo.mysql.com", "mysql-8.4-lts"}, []string{"default-mysql-client"}},
		{"mysql 9.0 innovation", dockerfile.MysqlClientModule, map[string]any{"version": "9.0"},
			[]string{"mysql-community-client", "mysql-innovation"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.module.Render(tc.opts)
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
			// Every variant installs in a single RUN with inline cleanup.
			if got := strings.Count(out, "RUN "); got != 1 {
				t.Errorf("expected a single RUN layer, got %d:\n%s", got, out)
			}
		})
	}
}

// standardCleanupFragments are the commands every apt install RUN must chain so
// the package cache never lands in that layer. They mirror the single source of
// truth in helpers.go (aptCleanupCommands) and CleanupModule's final pass.
var standardCleanupFragments = []string{
	"apt-get autoremove -y",
	"apt-get autoclean",
	"rm -rf /var/lib/apt/lists/*",
	"rm -rf /tmp/*",
	"rm -rf /var/tmp/*",
}

// Every module that installs apt packages must append the standardized cleanup
// inline so the caches are purged in the same layer that created them.
func TestAptModulesIncludeStandardCleanup(t *testing.T) {
	cases := []struct {
		name   string
		module *dockerfile.ModuleSpec
		opts   map[string]any
	}{
		{"base", dockerfile.BaseModule, nil},
		{"python", dockerfile.PythonModule, nil},
		{"tmux", dockerfile.TmuxModule, nil},
		{"php", dockerfile.PhpModule, nil},
		{"rust", dockerfile.RustModule, nil},
		{"sqlite", dockerfile.SqliteModule, nil},
		{"pnpm", dockerfile.PnpmModule, nil},
		{"github-cli", dockerfile.GithubCliModule, nil},
		{"dod", dockerfile.DodModule, nil},
		{"java-temurin", dockerfile.JavaTemurinModule, nil},
		{"java-openjdk", dockerfile.JavaOpenjdkModule, nil},
		{"postgres-client", dockerfile.PostgresClientModule, nil},
		{"redis-client", dockerfile.RedisClientModule, nil},
		{"mysql-client", dockerfile.MysqlClientModule, nil},
		{"mongo-client", dockerfile.MongoClientModule, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.module.Render(tc.opts)
			for _, frag := range standardCleanupFragments {
				if !strings.Contains(out, frag) {
					t.Errorf("module %q must include cleanup fragment %q:\n%s", tc.name, frag, out)
				}
			}
		})
	}
}

// CleanupModule's final pass must keep emitting the same standardized cleanup.
func TestCleanupModuleRender(t *testing.T) {
	out := dockerfile.CleanupModule.Render(nil)
	for _, frag := range standardCleanupFragments {
		if !strings.Contains(out, frag) {
			t.Errorf("cleanup module must include fragment %q:\n%s", frag, out)
		}
	}
	if !strings.Contains(out, `ENTRYPOINT ["/entrypoint.sh"]`) {
		t.Errorf("cleanup module must keep the entrypoint:\n%s", out)
	}
}

// Consecutive RUN steps within a module are consolidated so each renders the
// minimum number of layers. These counts guard against regressions that would
// re-split the install steps.
func TestModuleRunLayerCounts(t *testing.T) {
	cases := []struct {
		name    string
		module  *dockerfile.ModuleSpec
		opts    map[string]any
		wantRun int
	}{
		{"nodejs nvm", dockerfile.NodejsModule, nil, 2}, // install + shell-init
		{"nodejs fnm", dockerfile.NodejsModule, map[string]any{"manager": "fnm"}, 2},
		{"python with uv", dockerfile.PythonModule, map[string]any{"uv": true}, 2}, // install+uv + shell-init
		{"python no uv", dockerfile.PythonModule, map[string]any{"uv": false}, 1},
		{"rust", dockerfile.RustModule, nil, 2}, // install+rustup + shell-init
		{"pnpm", dockerfile.PnpmModule, nil, 1}, // install+pnpm
		{"bun", dockerfile.BunModule, nil, 2},   // install + shell-init
		{"sqlite", dockerfile.SqliteModule, nil, 1},
		{"tmux", dockerfile.TmuxModule, nil, 1},
		{"github-cli", dockerfile.GithubCliModule, nil, 1},
		{"dod", dockerfile.DodModule, nil, 1},
		{"java-temurin", dockerfile.JavaTemurinModule, nil, 1},
		{"java-openjdk", dockerfile.JavaOpenjdkModule, nil, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Count(tc.module.Render(tc.opts), "RUN ")
			if got != tc.wantRun {
				t.Errorf("module %q rendered %d RUN layers, want %d:\n%s",
					tc.name, got, tc.wantRun, tc.module.Render(tc.opts))
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
