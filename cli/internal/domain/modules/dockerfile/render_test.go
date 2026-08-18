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
		t.Errorf("base should pin the Ubuntu LTS (24.04):\n%s", out)
	}
	// The Ubuntu version is no longer user-selectable: options are ignored and
	// the LTS is always used. Only the p10k default style is configurable.
	if len(dockerfile.BaseModule.Options) != 1 || dockerfile.BaseModule.Options[0].ID != "p10kStyle" {
		t.Errorf("base module must expose exactly the p10kStyle option, got %v", dockerfile.BaseModule.Options)
	}
	out = dockerfile.BaseModule.Render(map[string]any{"ubuntu": "22.04"})
	if !strings.Contains(out, "FROM ubuntu:24.04") {
		t.Errorf("base must ignore any ubuntu option and stay on the LTS:\n%s", out)
	}
	// chmod + run + rm of the zsh installer must be a single consolidated RUN so
	// the script is removed in the same layer it is used. Without a p10kStyle
	// option, the default style must still be installed — remote images are
	// generated with --no-interactive, which never fills option defaults in.
	if !strings.Contains(out, "RUN chmod +x /tmp/zsh-installer.sh && \\\n    su - devuser -c \"/tmp/zsh-installer.sh "+dockerfile.DefaultP10kStyle+"\" && \\\n    rm /tmp/zsh-installer.sh") {
		t.Errorf("zsh installer chmod/run/rm should be a single RUN installing the default style:\n%s", out)
	}
	// sshd must keep long-lived sessions (e.g. a --via jump) alive and reclaim
	// dead ones, via /etc/ssh/sshd_config rather than a runtime -o flag.
	if !strings.Contains(out, "ClientAliveInterval 60") || !strings.Contains(out, "ClientAliveCountMax 3") {
		t.Errorf("base must set sshd ClientAliveInterval/ClientAliveCountMax:\n%s", out)
	}
	if !strings.Contains(out, "/etc/ssh/sshd_config") {
		t.Errorf("base must edit /etc/ssh/sshd_config for the keepalive settings:\n%s", out)
	}
	// The default is a real preset, so the shell is themed on first login instead
	// of dropping into `p10k configure`.
	if dockerfile.DefaultP10kStyle == "none" {
		t.Fatalf("default p10k style must be a real preset, got %q", dockerfile.DefaultP10kStyle)
	}
	offered := false
	for _, c := range dockerfile.BaseModule.Options[0].Choices {
		if c.Value == dockerfile.DefaultP10kStyle {
			offered = true
		}
	}
	if !offered {
		t.Errorf("default p10k style %q is not one of the offered choices %v", dockerfile.DefaultP10kStyle, dockerfile.BaseModule.Options[0].Choices)
	}
	if dockerfile.BaseModule.Options[0].Default != dockerfile.DefaultP10kStyle {
		t.Errorf("p10kStyle option default = %v, want %q", dockerfile.BaseModule.Options[0].Default, dockerfile.DefaultP10kStyle)
	}
	// "none" is the explicit opt-out: it must still run the script with no
	// arguments so p10k stays unconfigured.
	outNone := dockerfile.BaseModule.Render(map[string]any{"p10kStyle": "none"})
	if !strings.Contains(outNone, `su - devuser -c "/tmp/zsh-installer.sh"`) {
		t.Errorf("p10kStyle=none must run the zsh installer without arguments:\n%s", outNone)
	}
	// A recognized style must be forwarded as the script's argument.
	outRainbow := dockerfile.BaseModule.Render(map[string]any{"p10kStyle": "rainbow"})
	if !strings.Contains(outRainbow, `su - devuser -c "/tmp/zsh-installer.sh rainbow"`) {
		t.Errorf("p10kStyle=rainbow must be forwarded to the zsh installer:\n%s", outRainbow)
	}
	// An unrecognized style must fall back to the default rather than injecting
	// an arbitrary value into the shell command.
	outBogus := dockerfile.BaseModule.Render(map[string]any{"p10kStyle": "not-a-real-style"})
	if !strings.Contains(outBogus, `su - devuser -c "/tmp/zsh-installer.sh `+dockerfile.DefaultP10kStyle+`"`) {
		t.Errorf("unrecognized p10kStyle must fall back to the default style:\n%s", outBogus)
	}
	// devuser's UID/GID come from build args so a local-cached image can match
	// the host owner of the workspace without a runtime remap; the stock Ubuntu
	// "ubuntu" user squatting on 1000 is removed so the ids are free.
	for _, frag := range []string{"ARG USER_UID=1000", "ARG USER_GID=1000", `useradd -m -u "${USER_UID}" -g "${USER_GID}" -s /bin/zsh devuser`, "userdel -r ubuntu"} {
		if !strings.Contains(out, frag) {
			t.Errorf("base must contain %q:\n%s", frag, out)
		}
	}
	// micro ships in the base image by default (alongside nano) so an editor is
	// always available in every container.
	if !strings.Contains(out, "\n    micro \\") {
		t.Errorf("base must install the micro editor by default:\n%s", out)
	}
	// lsof ships in the base image so the kill_port helper from the aliases
	// module (kill -9 $(lsof -t -i:<port>)) works in every container.
	if !strings.Contains(out, "\n    lsof \\") {
		t.Errorf("base must install lsof by default:\n%s", out)
	}
	// The ~/help quick reference (micro + zellij shortcuts) is materialized into
	// devuser's home at build time, then its installer script is removed.
	for _, frag := range []string{
		"COPY setup-help.sh /tmp/setup-help.sh",
		`su - devuser -c "/tmp/setup-help.sh"`,
		"rm /tmp/setup-help.sh",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("base must install the ~/help quick reference (%q):\n%s", frag, out)
		}
	}
}

func TestBaseModuleProvidesLocalBinOnPath(t *testing.T) {
	// ~/.local/bin must be on PATH unconditionally (post-script installers and
	// pip/uv --user binaries land there). It is declared, not exported from the
	// module's own Dockerfile fragment, so it reaches the image ENV too.
	env := dockerfile.BaseModule.ProvidesEnv(nil)
	if len(env.PathEntries) != 1 || env.PathEntries[0] != "$HOME/.local/bin" {
		t.Errorf("base must declare ~/.local/bin on PATH, got %v", env.PathEntries)
	}
	if strings.Contains(dockerfile.BaseModule.Render(nil), "export PATH") {
		t.Error("base must not export PATH from its Dockerfile fragment; ProvidesEnv owns that")
	}
}

// Neither fnm nor nvm has a stable path to the active Node: fnm resolves it
// from `fnm env` and nvm nests it under a version-named directory, so a command
// running without a shell could not find node at all. The build leaves a link
// at a fixed path and the module declares that path instead.
func TestNodejsModuleMakesNodeReachableWithoutAShell(t *testing.T) {
	for _, manager := range []string{"fnm", "nvm"} {
		t.Run(manager, func(t *testing.T) {
			opts := map[string]any{"manager": manager}

			out := dockerfile.NodejsModule.Render(opts)
			if !strings.Contains(out, `ln -sfn "$(dirname "$(command -v node)")"`) {
				t.Errorf("the build must record the active node bin dir:\n%s", out)
			}
			if !strings.Contains(out, dockerfile.NodeCurrentLink) {
				t.Errorf("expected the link to be %q:\n%s", dockerfile.NodeCurrentLink, out)
			}

			env := dockerfile.NodejsModule.ProvidesEnv(opts)
			if !containsPathEntry(env, dockerfile.PathEntry("$HOME/"+dockerfile.NodeCurrentLink)) {
				t.Errorf("%s must declare the node link on PATH, got %v", manager, env.PathEntries)
			}
			// fnm is a binary of its own; nvm is a shell function with nothing
			// to put on PATH.
			hasFnm := containsPathEntry(env, "$HOME/.fnm")
			if hasFnm != (manager == "fnm") {
				t.Errorf("%s: fnm on PATH = %v, want %v", manager, hasFnm, manager == "fnm")
			}
		})
	}
}

// The fnm build step must select a version before recording the link, or
// `command -v node` finds nothing to point at.
func TestNodejsFnmSelectsVersionBeforeLinking(t *testing.T) {
	out := dockerfile.NodejsModule.Render(map[string]any{"manager": "fnm"})
	useAt := strings.Index(out, "fnm use ")
	linkAt := strings.Index(out, "ln -sfn")
	if useAt == -1 || linkAt == -1 || useAt > linkAt {
		t.Errorf("expected `fnm use` before the link:\n%s", out)
	}
}

func containsPathEntry(env dockerfile.ContainerEnv, want dockerfile.PathEntry) bool {
	for _, entry := range env.PathEntries {
		if entry == want {
			return true
		}
	}
	return false
}

func TestNodejsModuleRender(t *testing.T) {
	cases := []struct {
		name    string
		opts    map[string]any
		want    []string
		notWant []string
	}{
		{
			name:    "default fnm lts",
			opts:    nil,
			want:    []string{"fnm", "fnm install --lts"},
			notWant: []string{"nvm install"},
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
	if env := dockerfile.RustModule.ProvidesEnv(nil); len(env.PathEntries) != 1 || env.PathEntries[0] != "$HOME/.cargo/bin" {
		t.Errorf("rust must declare ~/.cargo/bin on PATH, got %v", env.PathEntries)
	}
}

func TestModulesProvidingEnvDeclareItAsData(t *testing.T) {
	cases := []struct {
		module      *dockerfile.ModuleSpec
		opts        map[string]any
		assignments map[string]dockerfile.EnvValue
		pathEntries []dockerfile.PathEntry
	}{
		{
			module:      dockerfile.BunModule,
			assignments: map[string]dockerfile.EnvValue{"BUN_INSTALL": "$HOME/.bun"},
			pathEntries: []dockerfile.PathEntry{"$HOME/.bun/bin"},
		},
		{
			module:      dockerfile.PnpmModule,
			assignments: map[string]dockerfile.EnvValue{"PNPM_HOME": "$HOME/.local/share/pnpm"},
			pathEntries: []dockerfile.PathEntry{"$HOME/.local/share/pnpm/bin", "$HOME/.local/share/pnpm"},
		},
		{
			module:      dockerfile.PythonModule,
			opts:        map[string]any{"uv": true},
			assignments: map[string]dockerfile.EnvValue{"UV_SYSTEM_PYTHON": "1"},
			pathEntries: []dockerfile.PathEntry{"$HOME/.local/bin"},
		},
		{
			module:      dockerfile.GolangModule,
			pathEntries: []dockerfile.PathEntry{"/usr/local/go/bin", "$HOME/go/bin"},
		},
	}
	for _, c := range cases {
		t.Run(string(c.module.ID), func(t *testing.T) {
			env := c.module.ProvidesEnv(c.opts)
			for name, want := range c.assignments {
				if got := assignmentValue(env, name); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
			if len(env.PathEntries) != len(c.pathEntries) {
				t.Fatalf("PATH entries = %v, want %v", env.PathEntries, c.pathEntries)
			}
			for i, want := range c.pathEntries {
				if env.PathEntries[i] != want {
					t.Errorf("PATH entry %d = %q, want %q", i, env.PathEntries[i], want)
				}
			}
			// The declaration replaces the module's own shell-init file; keeping
			// both would put the same entry on PATH twice and let the two drift
			// apart. An export inside an install step is fine — that one is for
			// the build, which runs before the image environment exists.
			if strings.Contains(c.module.Render(c.opts), "_init.sh") {
				t.Errorf("%s must not write a shell-init file for its environment", c.module.ID)
			}
		})
	}
}

func TestPythonWithoutUvProvidesNoEnv(t *testing.T) {
	env := dockerfile.PythonModule.ProvidesEnv(map[string]any{"uv": false})
	if !env.IsEmpty() {
		t.Errorf("without uv the python module has no environment to declare, got %+v", env)
	}
}

func assignmentValue(env dockerfile.ContainerEnv, name string) dockerfile.EnvValue {
	for _, assignment := range env.Assignments {
		if assignment.Name == name {
			return assignment.Value
		}
	}
	return ""
}

func TestCCppModuleRender(t *testing.T) {
	out := dockerfile.CCppModule.Render(nil)
	for _, pkg := range []string{"build-essential", "gdb", "cmake", "clang"} {
		if !strings.Contains(out, pkg) {
			t.Errorf("expected C/C++ module to install %q:\n%s", pkg, out)
		}
	}
	// Single install layer with inline cleanup.
	if got := strings.Count(out, "RUN "); got != 1 {
		t.Errorf("expected a single RUN layer, got %d:\n%s", got, out)
	}
}

func TestYarnModuleRender(t *testing.T) {
	out := dockerfile.YarnModule.Render(nil)
	// Yarn is installed through Corepack, which ships with Node.
	if !strings.Contains(out, "corepack enable") {
		t.Errorf("expected yarn to enable corepack:\n%s", out)
	}
	if !strings.Contains(out, "corepack prepare yarn@stable --activate") {
		t.Errorf("expected yarn to pin the stable release via corepack:\n%s", out)
	}
	// Build step must source the node init so node/corepack are on PATH,
	// independent of whether nvm or fnm manages Node.
	if !strings.Contains(out, ".nodejs_init.sh") {
		t.Errorf("expected yarn to source the node init script:\n%s", out)
	}
}

func TestChromeModuleRender(t *testing.T) {
	out := dockerfile.ChromeModule.Render(nil)
	// Installs the same container-friendly Chromium .deb from the xtradeb/apps
	// PPA on every arch, so the binary name and behaviour are identical across
	// amd64 and arm64.
	for _, frag := range []string{
		"ppa:xtradeb/apps",
		"apt-get install -y chromium",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("chrome module must install chromium from the xtradeb PPA, missing %q:\n%s", frag, out)
		}
	}
	// Always Chromium — no architecture branch and no amd64-only Google Chrome.
	for _, nope := range []string{"google-chrome", "dl.google.com", `"$ARCH" = "amd64"`} {
		if strings.Contains(out, nope) {
			t.Errorf("chrome module must always install chromium, found %q:\n%s", nope, out)
		}
	}
	// Chromium is decoupled from any automation framework — no Playwright/Selenium
	// client is baked in.
	for _, nope := range []string{"playwright", "selenium", "puppeteer", "npm install", "pip install"} {
		if strings.Contains(out, nope) {
			t.Errorf("chrome module must stay framework-agnostic, found %q:\n%s", nope, out)
		}
	}
	// Single install layer with inline cleanup.
	if got := strings.Count(out, "RUN "); got != 1 {
		t.Errorf("expected a single RUN layer, got %d:\n%s", got, out)
	}
}

func TestFfmpegModuleRender(t *testing.T) {
	out := dockerfile.FfmpegModule.Render(nil)
	if !strings.Contains(out, "apt-get install -y ffmpeg") {
		t.Errorf("ffmpeg module must install the ffmpeg package:\n%s", out)
	}
}

// cloudflared replaced the sibling "tunnel" compose service, so it installs the
// binary into the image and declares the token it reads at runtime.
func TestCloudflaredModuleRender(t *testing.T) {
	out := dockerfile.CloudflaredModule.Render(nil)
	if !strings.Contains(out, "apt-get install -y cloudflared") {
		t.Errorf("cloudflared module must install the cloudflared package:\n%s", out)
	}
	if !strings.Contains(out, "signed-by=/etc/apt/keyrings/cloudflare-main.gpg") {
		t.Errorf("cloudflared apt list must be signed by the installed keyring:\n%s", out)
	}
	// The declared env var is what the generator passes through to the container
	// and what the wizard prompts for; without it the token can never arrive.
	if len(dockerfile.CloudflaredModule.RequiresEnv) != 1 {
		t.Fatalf("cloudflared must declare exactly its token env var, got %v", dockerfile.CloudflaredModule.RequiresEnv)
	}
	env := dockerfile.CloudflaredModule.RequiresEnv[0]
	// The name must stay TUNNEL_TOKEN: it is what the removed tunnel service used,
	// so a migrated project's existing .env keeps working.
	if dockerfile.TunnelTokenEnv != "TUNNEL_TOKEN" {
		t.Errorf("TunnelTokenEnv = %q, want TUNNEL_TOKEN", dockerfile.TunnelTokenEnv)
	}
	if env.Name != dockerfile.TunnelTokenEnv {
		t.Errorf("cloudflared env var = %q, want %q", env.Name, dockerfile.TunnelTokenEnv)
	}
	// An empty token is a supported answer, so it must carry no Default that
	// would silently pre-fill the prompt.
	if env.Default != "" {
		t.Errorf("TUNNEL_TOKEN must have no default, got %q", env.Default)
	}
	// The context has to name all three ways to run a tunnel, since which one
	// applies depends on whether the user supplied a token — and an agent that
	// only knows the token path would report "no token" as a dead end when a free
	// quick tunnel needs no account at all.
	sec := dockerfile.CloudflaredModule.Context(nil)
	if sec == nil {
		t.Fatal("cloudflared must document itself for agents")
	}
	for _, frag := range []string{
		"TUNNEL_TOKEN",
		"cloudflared tunnel run",
		"cloudflared tunnel login",
		"cloudflared tunnel --url",
		"trycloudflare.com",
	} {
		if !strings.Contains(sec.Body, frag) {
			t.Errorf("cloudflared context must mention %q:\n%s", frag, sec.Body)
		}
	}
	// A quick tunnel is still a public URL; the warning must not read as if it
	// only applied to the token path.
	if !strings.Contains(sec.Body, "public internet") {
		t.Errorf("cloudflared context must warn that a tunnel is public:\n%s", sec.Body)
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
		{"mongo", dockerfile.MongoClientModule, nil, []string{"MONGODB CLIENT", "mongodb-mongosh", "repo.mongodb.org"}, nil},
		// Pinned versions -> vendor apt repos.
		{"postgres 16 (PGDG)", dockerfile.PostgresClientModule, map[string]any{"version": "16"},
			[]string{"postgresql-client-16", "apt.postgresql.org", "pgdg main"}, nil},
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
		{"php", dockerfile.PhpModule, nil},
		{"rust", dockerfile.RustModule, nil},
		{"c-cpp", dockerfile.CCppModule, nil},
		{"sqlite", dockerfile.SqliteModule, nil},
		{"ffmpeg", dockerfile.FfmpegModule, nil},
		{"pnpm", dockerfile.PnpmModule, nil},
		{"github-cli", dockerfile.GithubCliModule, nil},
		{"ngrok", dockerfile.NgrokModule, nil},
		{"cloudflared", dockerfile.CloudflaredModule, nil},
		{"dod", dockerfile.DodModule, nil},
		{"chrome", dockerfile.ChromeModule, nil},
		{"java-temurin", dockerfile.JavaTemurinModule, nil},
		{"java-openjdk", dockerfile.JavaOpenjdkModule, nil},
		{"postgres-client", dockerfile.PostgresClientModule, nil},
		{"redis-client", dockerfile.RedisClientModule, nil},
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
		// Modules whose only shell init was a set of exports render one layer
		// now: the exports moved to ProvidesEnv, which the generator emits once
		// for the whole image. Only genuinely dynamic init (fnm/nvm) still costs
		// a module its own shell-init layer.
		{"nodejs nvm", dockerfile.NodejsModule, nil, 2}, // install + shell-init
		{"nodejs fnm", dockerfile.NodejsModule, map[string]any{"manager": "fnm"}, 2},
		{"python with uv", dockerfile.PythonModule, map[string]any{"uv": true}, 1},
		{"python no uv", dockerfile.PythonModule, map[string]any{"uv": false}, 1},
		{"rust", dockerfile.RustModule, nil, 1},
		{"c-cpp", dockerfile.CCppModule, nil, 1},
		{"pnpm", dockerfile.PnpmModule, nil, 1},
		{"bun", dockerfile.BunModule, nil, 1},
		{"yarn", dockerfile.YarnModule, nil, 1}, // corepack enable + prepare (single RUN)
		{"sqlite", dockerfile.SqliteModule, nil, 1},
		{"ffmpeg", dockerfile.FfmpegModule, nil, 1},
		{"zellij", dockerfile.ZellijModule, nil, 1},
		{"github-cli", dockerfile.GithubCliModule, nil, 1},
		{"ngrok", dockerfile.NgrokModule, nil, 1},
		{"cloudflared", dockerfile.CloudflaredModule, nil, 1},
		{"dod", dockerfile.DodModule, nil, 1},
		{"chrome", dockerfile.ChromeModule, nil, 1},
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

// The non-interactive installer modules are marked PostScriptAutoStart so the
// entrypoint runs them on start; the agent-wiring tools (graphify/caveman) carry
// a higher run order so they run after the agents. Interactive scripts (codex,
// the github login under cleanup) must NOT be auto-start.
// The ECC installer takes no arguments — the entrypoint runs start.d scripts
// with none — so the selected profile has to reach it as image environment.
func TestEccModuleDeclaresProfileEnv(t *testing.T) {
	cases := []struct {
		name string
		opts map[string]any
		want string
	}{
		{"default", nil, "developer"},
		{"explicit", map[string]any{"profile": "full"}, "full"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := dockerfile.EccModule.ProvidesEnv(c.opts)
			if len(env.Assignments) != 1 {
				t.Fatalf("ecc must declare exactly one assignment, got %d", len(env.Assignments))
			}
			got := env.Assignments[0]
			if got.Name != "DEVCONTAINER_ECC_PROFILE" {
				t.Errorf("assignment name = %q, want DEVCONTAINER_ECC_PROFILE", got.Name)
			}
			if string(got.Value) != c.want {
				t.Errorf("profile = %q, want %q", got.Value, c.want)
			}
		})
	}
	if env := dockerfile.EccModule.ProvidesEnv(nil); len(env.PathEntries) != 0 {
		t.Error("ecc must not put anything on PATH; the global npm/pnpm bin dir already is")
	}
}

func TestPostScriptAutoStartFlags(t *testing.T) {
	cases := []struct {
		name      string
		module    *dockerfile.ModuleSpec
		autoStart bool
		order     int // expected explicit order; 0 means "module default"
	}{
		{"claude-code", dockerfile.ClaudeCodeModule, true, 0},
		{"antigravity", dockerfile.AntigravityCliModule, true, 0},
		{"copilot", dockerfile.CopilotCliModule, true, 0},
		{"opencode", dockerfile.OpencodeModule, true, 0},
		{"graphify", dockerfile.GraphifyModule, true, 90},
		{"caveman", dockerfile.CavemanModule, true, 90},
		{"claude-mem", dockerfile.ClaudeMemModule, true, 90},
		{"context-mode", dockerfile.ContextModeModule, true, 90},
		{"ecc", dockerfile.EccModule, true, 90},
		{"skills", dockerfile.SkillsModule, true, 95},
		{"codex", dockerfile.CodexCliModule, false, 0},
		{"cleanup (github login)", dockerfile.CleanupModule, false, 0},
		{"golang", dockerfile.GolangModule, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.module.PostScriptAutoStart != tc.autoStart {
				t.Errorf("module %q PostScriptAutoStart = %v, want %v", tc.name, tc.module.PostScriptAutoStart, tc.autoStart)
			}
			if tc.module.PostScriptStartOrder != tc.order {
				t.Errorf("module %q PostScriptStartOrder = %d, want %d", tc.name, tc.module.PostScriptStartOrder, tc.order)
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
