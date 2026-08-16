package dockerfile

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func contextBodyFor(t *testing.T, m *ModuleSpec, opts map[string]any) string {
	t.Helper()
	if m.Context == nil {
		t.Fatalf("module %q declares no Context", m.ID)
	}
	sec := m.Context(opts)
	if sec == nil {
		t.Fatalf("module %q returned no context section for %v", m.ID, opts)
	}
	return sec.Body
}

// Python's section is the sharpest option-driven case: with uv the agent is
// told to stop calling pip, without it pip is the real thing.
func TestPythonContextFollowsUvOption(t *testing.T) {
	withUv := contextBodyFor(t, PythonModule, map[string]any{"uv": true})
	// All three commands, because they are not interchangeable: a document that
	// names only `uv pip install` steers project work at the system interpreter
	// and hides `uv add`, which is the one that belongs in a project.
	for _, frag := range []string{
		"uv add X", "uv tool install X", "uv pip install X",
		"UV_SYSTEM_PYTHON=1", "UV_BREAK_SYSTEM_PACKAGES=1",
	} {
		if !strings.Contains(withUv, frag) {
			t.Errorf("python+uv context must mention %q:\n%s", frag, withUv)
		}
	}

	withoutUv := contextBodyFor(t, PythonModule, map[string]any{"uv": false})
	if strings.Contains(withoutUv, "uv pip install") {
		t.Errorf("python without uv must not tell the agent to run uv:\n%s", withoutUv)
	}
	// PEP 668 applies whether or not uv is installed, so the no-uv branch has to
	// name a destination pip can actually write to. It used to say
	// `pip install --user X`, which Debian's pip refuses on an externally
	// managed interpreter just like a plain install.
	if !strings.Contains(withoutUv, "python3 -m venv") {
		t.Errorf("python without uv must point at a venv, the one place pip can write:\n%s", withoutUv)
	}
	if strings.Contains(withoutUv, "pip install --user") {
		t.Errorf("--user is refused on an externally managed interpreter too:\n%s", withoutUv)
	}

	// The module default is uv=true, so nil options must match the uv branch.
	if contextBodyFor(t, PythonModule, nil) != withUv {
		t.Error("nil options must render the same section as uv=true (the module default)")
	}
}

// The node section names the manager and version actually installed, so an
// agent does not reach for apt or the wrong version manager.
func TestNodejsContextReportsManagerAndVersion(t *testing.T) {
	cases := []struct {
		opts map[string]any
		want []string
	}{
		{nil, []string{"fnm", "the latest LTS"}},
		{map[string]any{"manager": "nvm", "version": "22"}, []string{"nvm", "Node 22"}},
		{map[string]any{"manager": "fnm", "version": "24"}, []string{"fnm", "Node 24"}},
	}
	for _, c := range cases {
		body := contextBodyFor(t, NodejsModule, c.opts)
		for _, frag := range c.want {
			if !strings.Contains(body, frag) {
				t.Errorf("nodejs context for %v must mention %q:\n%s", c.opts, frag, body)
			}
		}
	}
}

// A Java module with every version deselected installs nothing, so it must stay
// out of the document entirely rather than announce an empty toolchain.
func TestJavaContextSilentWhenNoJdkSelected(t *testing.T) {
	for _, m := range []*ModuleSpec{JavaTemurinModule, JavaOpenjdkModule} {
		if sec := m.Context(map[string]any{"versions": []string{"none"}}); sec != nil {
			t.Errorf("module %q must contribute nothing when no JDK is selected, got %+v", m.ID, sec)
		}
		body := contextBodyFor(t, m, map[string]any{"versions": []string{"17", "21"}})
		for _, frag := range []string{"JDK 17", "JDK 21", "update-alternatives"} {
			if !strings.Contains(body, frag) {
				t.Errorf("module %q context must mention %q:\n%s", m.ID, frag, body)
			}
		}
	}
}

// Maven presence flips a claim an agent would otherwise act on.
func TestJavaContextReportsMaven(t *testing.T) {
	on := contextBodyFor(t, JavaTemurinModule, map[string]any{"versions": []string{"17"}, "maven": true})
	if !strings.Contains(on, "Maven (`mvn`) is installed") {
		t.Errorf("maven=true must be stated:\n%s", on)
	}
	off := contextBodyFor(t, JavaTemurinModule, map[string]any{"versions": []string{"17"}, "maven": false})
	if strings.Contains(off, "Maven (`mvn`) is installed") {
		t.Errorf("maven=false must not claim maven is installed:\n%s", off)
	}
}

func TestPhpContextReportsComposer(t *testing.T) {
	on := contextBodyFor(t, PhpModule, map[string]any{"composer": true})
	if !strings.Contains(on, "Composer is installed") {
		t.Errorf("composer=true must be stated:\n%s", on)
	}
	off := contextBodyFor(t, PhpModule, map[string]any{"composer": false})
	if !strings.Contains(off, "Composer is **not** installed") {
		t.Errorf("composer=false must be stated:\n%s", off)
	}
}

// The psql section names where the client came from, since a PGDG-pinned major
// and Ubuntu's generic client behave differently.
func TestPostgresClientContextReportsVersionSource(t *testing.T) {
	pinned := contextBodyFor(t, PostgresClientModule, map[string]any{"version": "17"})
	if !strings.Contains(pinned, "PostgreSQL 17 from the PGDG repository") {
		t.Errorf("a pinned version must name PGDG:\n%s", pinned)
	}
	auto := contextBodyFor(t, PostgresClientModule, map[string]any{"version": "auto"})
	if !strings.Contains(auto, "Ubuntu repositories") {
		t.Errorf("an unpinned version must name the Ubuntu repos:\n%s", auto)
	}
}

// The aliases section is where an agent learns which file to edit; naming the
// wrong one sends it to a file whose edits are discarded on recreate.
func TestAliasesContextPointsAtTheUserFile(t *testing.T) {
	body := contextBodyFor(t, AliasesModule, nil)
	for _, frag := range []string{
		"kill_port",
		"~/" + BakedAliasFile,
		"~/" + UserAliasFile,
		"the user's own aliases",
	} {
		if !strings.Contains(body, frag) {
			t.Errorf("aliases context must mention %q:\n%s", frag, body)
		}
	}
	baked := strings.Index(body, BakedAliasFile)
	user := strings.Index(body, UserAliasFile)
	if baked == -1 || user == -1 || baked > user {
		t.Errorf("the files must be listed in source order, defaults first (baked=%d user=%d):\n%s", baked, user, body)
	}
}

func TestAliasesContextDocumentsAgentAliases(t *testing.T) {
	body := contextBodyFor(t, AliasesModule, nil)
	if !strings.Contains(body, "skip") || !strings.Contains(body, "permission prompts") {
		t.Errorf("the section must explain the agents skip permission prompts:\n%s", body)
	}
	// Every aliased agent must be named with its _yolo alias, agy included, and
	// the plain command must be documented as untouched.
	for _, agent := range []string{"claude", "codex", "copilot", "agy"} {
		if !strings.Contains(body, "`"+agent+"_yolo`") {
			t.Errorf("the section must name the %q_yolo alias:\n%s", agent, body)
		}
		if !strings.Contains(body, "`"+agent+"`") {
			t.Errorf("the section must name the plain %q command:\n%s", agent, body)
		}
	}
	// The section no longer depends on any option.
	if contextBodyFor(t, AliasesModule, map[string]any{"yoloAgents": false}) != body {
		t.Error("the aliases context must not depend on options anymore")
	}
}

// agentCtx is shared by every AI CLI module, so its two branches are worth
// pinning directly.
func TestAgentCtx(t *testing.T) {
	auto := agentCtx("Tool", "Run it with `tool`.", true)
	if !strings.Contains(auto.Body, "runs in the background the") {
		t.Errorf("an auto-started agent must warn about the install delay:\n%s", auto.Body)
	}

	manual := agentCtx("Tool", "Run it with `tool`.", false)
	if strings.Contains(manual.Body, "runs in the background") {
		t.Errorf("a manual agent must not claim it auto-installs:\n%s", manual.Body)
	}

	extra := agentCtx("Tool", "Run it with `tool`.", false, "Extra line.")
	if !strings.Contains(extra.Body, "Extra line.") {
		t.Errorf("extra lines must be appended:\n%s", extra.Body)
	}
	if extra.Title != "Tool" {
		t.Errorf("unexpected title %q", extra.Title)
	}
}

// The docker-outside-of-docker section must be explicit that the daemon is the
// host's; an agent that assumes a nested daemon will mount the wrong paths.
func TestDodContextExplainsSiblingContainers(t *testing.T) {
	body := contextBodyFor(t, DodModule, nil)
	for _, frag := range []string{"host's", "siblings", "not a nested"} {
		if !strings.Contains(body, frag) {
			t.Errorf("dod context must mention %q:\n%s", frag, body)
		}
	}
}

func TestCtxBodyJoinsWithNewlines(t *testing.T) {
	if got := ctxBody("a", "", "b"); got != "a\n\nb" {
		t.Errorf("ctxBody = %q, want %q", got, "a\n\nb")
	}
	if got := ctxBody(); got != "" {
		t.Errorf("ctxBody() = %q, want empty", got)
	}
}

// Guard the field's contract across the whole package: a section that is all
// whitespace would be dropped by the generator, silently losing the module.
func TestModuleContextSectionsAreNonEmpty(t *testing.T) {
	for _, m := range []*ModuleSpec{
		BaseModule, AliasesModule, GithubCliModule, PythonModule, NodejsModule,
		PnpmModule, YarnModule, BunModule, GolangModule, RustModule, PhpModule,
		CCppModule, SqliteModule, ZellijModule, ChromeModule, FfmpegModule,
		NgrokModule, CloudflaredModule, DodModule, ClaudeCodeModule, CodexCliModule,
		AntigravityCliModule, CopilotCliModule, OpencodeModule, GraphifyModule,
		CavemanModule, ClaudeMemModule, ContextModeModule,
		PostgresClientModule, RedisClientModule, MongoClientModule,
	} {
		sec := m.Context(nil)
		if sec == nil {
			t.Errorf("module %q: nil section for default options", m.ID)
			continue
		}
		if strings.TrimSpace(sec.Title) == "" {
			t.Errorf("module %q: empty section title", m.ID)
		}
		if strings.TrimSpace(sec.Body) == "" {
			t.Errorf("module %q: empty section body", m.ID)
		}
		if strings.HasPrefix(strings.TrimSpace(sec.Title), "#") {
			t.Errorf("module %q: title must not carry its own heading marker (%q)", m.ID, sec.Title)
		}
	}
}

// The cleanup module is pure build plumbing and has nothing to tell an agent;
// keeping it silent is deliberate, not an oversight.
func TestCleanupModuleHasNoContext(t *testing.T) {
	if CleanupModule.Context != nil {
		t.Error("the cleanup module should contribute nothing to ~/CONTEXT.md")
	}
	_ = types.ModuleCleanup
}
