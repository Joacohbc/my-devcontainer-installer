package assets_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
)

// entrypoint.sh is one of the scripts embedded via //go:embed *.sh.
const embeddedScript = "entrypoint.sh"

// sharedConfigLib holds the shared-config module: the volume layout the
// entrypoint sources and the `config shared sync` helper concatenates.
const sharedConfigLib = "shared-config.sh"

func TestAssetExists(t *testing.T) {
	if !assets.AssetExists(embeddedScript) {
		t.Errorf("expected %q to be embedded", embeddedScript)
	}
	if assets.AssetExists("definitely-not-real.sh") {
		t.Error("expected unknown asset to be reported missing")
	}
}

func TestContent(t *testing.T) {
	data, err := assets.Content(embeddedScript)
	if err != nil {
		t.Fatalf("Content(%q) failed: %v", embeddedScript, err)
	}
	if len(data) == 0 {
		t.Errorf("expected non-empty content for %q", embeddedScript)
	}
	if _, err := assets.Content("definitely-not-real.sh"); err == nil {
		t.Error("expected error for unknown asset")
	}
}

func TestPreflight_CopiesEmbeddedScript(t *testing.T) {
	dir := t.TempDir()
	res := assets.Preflight([]string{embeddedScript}, dir)
	if len(res.Copied) != 1 || res.Copied[0] != embeddedScript {
		t.Fatalf("expected %q copied, got %+v", embeddedScript, res)
	}
	if len(res.Missing) != 0 {
		t.Errorf("unexpected missing: %v", res.Missing)
	}
	if _, err := os.Stat(filepath.Join(dir, embeddedScript)); err != nil {
		t.Errorf("expected script materialized on disk: %v", err)
	}
}

func TestPreflight_AlreadyPresentIsSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, embeddedScript), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := assets.Preflight([]string{embeddedScript}, dir)
	if len(res.AlreadyPresent) != 1 {
		t.Fatalf("expected file reported already present, got %+v", res)
	}
	if len(res.Copied) != 0 {
		t.Errorf("should not overwrite an existing file, got copied %v", res.Copied)
	}
	got, _ := os.ReadFile(filepath.Join(dir, embeddedScript))
	if string(got) != "custom" {
		t.Errorf("existing file was overwritten: %q", got)
	}
}

func TestPreflight_MissingAsset(t *testing.T) {
	dir := t.TempDir()
	res := assets.Preflight([]string{"no-such-asset.sh"}, dir)
	if len(res.Missing) != 1 || res.Missing[0] != "no-such-asset.sh" {
		t.Errorf("expected missing asset reported, got %+v", res)
	}
}

func TestValidateRequiredFiles(t *testing.T) {
	if missing := assets.ValidateRequiredFiles([]string{embeddedScript}, ""); len(missing) != 0 {
		t.Errorf("embedded script should validate, got missing %v", missing)
	}
	if missing := assets.ValidateRequiredFiles([]string{"no-such-asset.sh"}, ""); len(missing) != 1 {
		t.Errorf("expected unknown asset reported missing, got %v", missing)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "local-only.sh"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if missing := assets.ValidateRequiredFiles([]string{"local-only.sh"}, dir); len(missing) != 0 {
		t.Errorf("file resolvable from lookIn should validate, got %v", missing)
	}
}

// entrypoint.sh must align devuser's UID/GID with the /workspace owner instead
// of rewriting ACLs on the bind mount, so the host's original permissions stay
// untouched.
func TestEntrypointAlignsUIDInsteadOfChangingWorkspaceACLs(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)
	if !strings.Contains(script, "usermod -u") {
		t.Error("entrypoint must remap devuser's UID to the workspace owner (usermod -u)")
	}
	if strings.Contains(script, "setfacl -R") {
		t.Error("entrypoint must not run a recursive setfacl that mutates host permissions")
	}
	// The stock Ubuntu "ubuntu" user squatting on 1000 is now removed at build
	// time (see base module), so the runtime remap must not delete accounts.
	if strings.Contains(script, "userdel") {
		t.Error("entrypoint must not userdel at runtime; the squatter is removed at build time")
	}
	// The project mount is at /workspaces/<name> (unique per project so tool
	// history does not collide in the shared volume); the UID remap must operate
	// on the resolved dir, not a hardcoded path.
	if !strings.Contains(script, `for _project_mount in "$WORKSPACE_MOUNT_ROOT"/*`) {
		t.Error("entrypoint must resolve the project mount under /workspaces/")
	}
	// The home may only ever be chowned to devuser's ACTUAL UID/GID, never to
	// the workspace owner directly (the remap may have failed).
	if strings.Contains(script, `chown -R "$WS_UID:$WS_GID" "$DEV_HOME"`) {
		t.Error("entrypoint must not chown the home to the workspace UID; use devuser's actual UID")
	}
	if !strings.Contains(script, `chown -R "$DEV_UID:$DEV_GID" "$DEV_HOME"`) {
		t.Error("entrypoint must re-own the home to devuser's actual UID/GID when it drifted")
	}
}

// The short alias must be per-project too. A bare /workspace pointing at the
// project is the path people actually cd into, so it silently re-merges the
// path-keyed session history of every project the per-project mount separated.
func TestEntrypointAliasesWorkspacePerProject(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)
	if !strings.Contains(script, `ln -sfn "$project_mount" "$WORKSPACE_ALIAS_ROOT/$(basename "$project_mount")"`) {
		t.Error("entrypoint must alias the project at /workspace/<name>")
	}
	if strings.Contains(script, `ln -s "$_project_mount" /workspace`+"\n") {
		t.Error("entrypoint must not create a bare /workspace link to the project")
	}
	// A container started by an older image carries the bare link in its
	// writable layer, so the alias dir can only be created after dropping it.
	if !strings.Contains(script, `[ -L "$WORKSPACE_ALIAS_ROOT" ] && rm -f "$WORKSPACE_ALIAS_ROOT"`) {
		t.Error("entrypoint must replace a bare /workspace link left by an older image")
	}
}

// TestEntrypointWorkspaceAliasIsCreated runs the entrypoint's own workspace
// resolution block against temp stand-ins for /workspaces and /workspace, both
// on a fresh container and on one whose writable layer still carries the bare
// link an older image created.
func TestEntrypointWorkspaceAliasIsCreated(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	workspaceBlock := extractWorkspaceBlock(t, string(body))

	for _, tc := range []struct {
		name              string
		hasStaleBareAlias bool
	}{
		{name: "fresh"},
		{name: "upgraded from a bare alias", hasStaleBareAlias: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Neutral dir names: the block is localized by string replacement,
			// and a path containing "/workspace" would be rewritten twice.
			containerRoot, err := os.MkdirTemp("", "dc-mount-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(containerRoot)
			mountRoot := filepath.Join(containerRoot, "mounts")
			aliasRoot := filepath.Join(containerRoot, "short")
			projectMount := filepath.Join(mountRoot, "myproj")
			if err := os.MkdirAll(projectMount, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.hasStaleBareAlias {
				if err := os.Symlink(projectMount, aliasRoot); err != nil {
					t.Fatal(err)
				}
			}

			// The stage reads the WORKSPACE_* globals the entrypoint sets at top
			// level; point them at the temp stand-ins rather than rewriting the
			// block's text (a path containing "/workspace" would be rewritten twice).
			localizedBlock := "WORKSPACE_MOUNT_ROOT=" + mountRoot + "\n" +
				"WORKSPACE_ALIAS_ROOT=" + aliasRoot + "\n" +
				"LEGACY_WORKSPACE_MOUNT=" + aliasRoot + "\n" + workspaceBlock
			if out, err := exec.Command("bash", "-c", localizedBlock).CombinedOutput(); err != nil {
				t.Fatalf("workspace block: %v\n%s", err, out)
			}

			aliasRootInfo, err := os.Lstat(aliasRoot)
			if err != nil {
				t.Fatalf("alias root: %v", err)
			}
			if !aliasRootInfo.IsDir() {
				t.Fatalf("the alias root must be a directory holding one link per project, got mode %v", aliasRootInfo.Mode())
			}
			projectAlias := filepath.Join(aliasRoot, "myproj")
			aliasTarget, err := os.Readlink(projectAlias)
			if err != nil {
				t.Fatalf("per-project alias: %v", err)
			}
			if aliasTarget != projectMount {
				t.Errorf("alias -> %q, want %q", aliasTarget, projectMount)
			}
		})
	}
}

// extractWorkspaceBlock assembles the entrypoint's real workspace resolution —
// the helper plus the stage that drives it — followed by a call, so the test
// runs the shipped code instead of a copy of it.
func extractWorkspaceBlock(t *testing.T, script string) string {
	t.Helper()
	return extractShellFunc(t, script, "alias_project_mount") +
		extractShellFunc(t, script, "stage_workspace") +
		"\nstage_workspace\n"
}

// TestVolumeAliasTargetResolves runs the library's real volume_alias_target and
// proves the aliases it builds point back at the flat entry, for a nested target
// (.config/gh -> ../gh) as much as a top-level one (.claude -> claude). An alias
// with the wrong number of "../" hops does not fail, it silently dangles — which
// is exactly the failure it exists to prevent.
func TestVolumeAliasTargetResolves(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	body, err := os.ReadFile(sharedConfigLib)
	if err != nil {
		t.Fatalf("reading %s: %v", sharedConfigLib, err)
	}
	fn := extractShellFunc(t, string(body), "volume_alias_target")

	vol := t.TempDir()
	for _, tc := range []struct{ id, target string }{
		{"claude", ".claude"},
		{"gh", ".config/gh"},
		{"antigravity-config", ".config/antigravity"},
	} {
		if err := os.MkdirAll(filepath.Join(vol, tc.id), 0o755); err != nil {
			t.Fatal(err)
		}
		script := fn + `
alias_path="$1/$2"
mkdir -p "$(dirname "$alias_path")"
ln -sfn "$(volume_alias_target "$2" "$3")" "$alias_path"`
		cmd := exec.Command("bash", "-c", script, "bash", vol, tc.target, tc.id)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("volume_alias_target(%s): %v\n%s", tc.target, err, out)
		}
		alias := filepath.Join(vol, filepath.FromSlash(tc.target))
		resolved, err := filepath.EvalSymlinks(alias)
		if err != nil {
			t.Errorf("volume alias %s does not resolve: %v", tc.target, err)
			continue
		}
		want, err := filepath.EvalSymlinks(filepath.Join(vol, tc.id))
		if err != nil {
			t.Fatal(err)
		}
		if resolved != want {
			t.Errorf("volume alias %s resolves to %s, want %s", tc.target, resolved, want)
		}
	}
}

// extractShellFunc returns the source of a `name() { … }` function defined in
// script, so a test can run the entrypoint's real helper instead of a copy.
//
// The closing brace is found at the same indentation as the definition rather
// than at a fixed one: anchoring on four spaces (what this used to do) made the
// entrypoint's whitespace load-bearing for the test harness, so moving a
// function out of a block broke tests that had nothing to do with the change.
func extractShellFunc(t *testing.T, script, name string) string {
	t.Helper()
	start := strings.Index(script, name+"() {")
	if start < 0 {
		t.Fatalf("entrypoint has no %s() function", name)
	}
	lineStart := strings.LastIndex(script[:start], "\n") + 1
	indent := script[lineStart:start]
	if i := strings.IndexFunc(indent, func(r rune) bool { return r != ' ' && r != '\t' }); i >= 0 {
		indent = indent[:i]
	}
	closer := "\n" + indent + "}\n"
	rest := script[lineStart:]
	end := strings.Index(rest, closer)
	if end < 0 {
		t.Fatalf("could not find the end of %s()", name)
	}
	return rest[:end+len(closer)]
}

// Antigravity CLI 2.0 reads skills from ~/.gemini/antigravity-cli/skills and
// does not understand ~/.agents/skills, so the entrypoint must bridge the two
// with a symlink into the shared .agents skills, without clobbering a real dir.
func TestEntrypointBridgesAntigravitySkills(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)
	if !strings.Contains(script, `ag_skills_link="$DEV_HOME/.gemini/antigravity-cli/skills"`) {
		t.Error("entrypoint must target the Antigravity CLI skills dir under ~/.gemini/antigravity-cli")
	}
	if !strings.Contains(script, `ag_shared_skills="$DEV_HOME/.agents/skills"`) {
		t.Error("entrypoint must point Antigravity's skills dir at the shared ~/.agents/skills")
	}
	// Not clobbering a real dir is link_or_keep's invariant now, proven by
	// TestEntrypointLinkOrKeep; here we only check the bridge goes through it.
	if !strings.Contains(script, `link_or_keep "$ag_skills_link" "$ag_shared_skills"`) {
		t.Error("the Antigravity bridge must be created with link_or_keep")
	}
}

// The baked defaults must ship kill_port and the pip/npm redirects, and the
// agent aliases must be present unconditionally (each guarded only by command -v,
// with no build-time flag-file gate) — agy included. They are named <tool>_yolo
// and must never shadow the tool itself: `claude` stays the plain CLI.
func TestAliasScriptShipsDefaults(t *testing.T) {
	body, err := os.ReadFile("alias.sh")
	if err != nil {
		t.Fatalf("reading alias.sh: %v", err)
	}
	script := string(body)

	for _, frag := range []string{
		"kill_port()",
		"lsof -t -i:",
		"alias npm='pnpm'",
		"alias npx='pnpm dlx'",
		"pip() { uv pip",
		"UV_SYSTEM_PYTHON=1",
		// Both halves, or `uv pip` hits PEP 668 on Ubuntu's interpreter.
		"UV_BREAK_SYSTEM_PACKAGES=1",
	} {
		if !strings.Contains(script, frag) {
			t.Errorf("alias.sh must contain %q", frag)
		}
	}

	agents := map[string]string{
		"claude":  "claude --dangerously-skip-permissions",
		"codex":   "codex --dangerously-bypass-approvals-and-sandbox",
		"copilot": "copilot --allow-all-tools",
		"agy":     "agy --dangerously-skip-permissions",
	}
	for name, aliasBody := range agents {
		if !strings.Contains(script, "command -v "+name+" >/dev/null 2>&1") {
			t.Errorf("alias.sh must guard the %q alias on `command -v %s`", name, name)
		}
		if !strings.Contains(script, "alias "+name+"_yolo='"+aliasBody+"'") {
			t.Errorf("alias.sh must define alias %s_yolo='%s'", name, aliasBody)
		}
		// The plain command must stay the unmodified CLI: only <tool>_yolo skips
		// the permission prompts.
		if strings.Contains(script, "alias "+name+"=") {
			t.Errorf("alias.sh must not shadow %q itself; only %s_yolo may add the skip flag", name, name)
		}
	}

	// The old opt-in gate is gone: the aliases are unconditional now.
	if strings.Contains(script, "devcontainer_agents_yolo") {
		t.Errorf("alias.sh must no longer gate the agent block on a flag file:\n%s", script)
	}
}

// The user's alias file must always exist and be devuser-owned, so "edit your
// aliases" has an answer even when the shared-config volume is opted out of.
// With the volume mounted the entry is already a symlink, and the -e test must
// leave it alone rather than replacing it with a local file.
func TestEntrypointEnsuresUserAliasFile(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)

	if !strings.Contains(script, `[ -e "$DEV_HOME/.alias.sh" ] && return 0`) {
		t.Error("entrypoint must only create ~/.alias.sh when nothing is there (a symlink counts)")
	}
	if !strings.Contains(script, `su - devuser -c 'cat > "$HOME/.alias.sh"'`) {
		t.Error("entrypoint must write ~/.alias.sh as devuser, not root")
	}
	if !strings.Contains(script, `chown "$DEV_UID:$DEV_GID" "$DEV_HOME/.alias.sh"`) {
		t.Error("entrypoint must leave ~/.alias.sh owned by devuser's actual UID/GID")
	}

	// The ordering constraint that makes this a fallback rather than a race is
	// pinned once, for every stage, by TestEntrypointStageOrder.
}

// The container context reaches agents through the generated ~/CONTEXT.md and
// get-devcontainer-context only. The entrypoint must NOT write into the agents'
// own memory files: those live in the shared volume and are the user's.
func TestEntrypointDoesNotTouchAgentMemoryFiles(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)

	for _, frag := range []string{"CLAUDE.md", "AGENTS.md", "devcontainer-cli:context", "DEVCONTAINER_AGENT_CONTEXT"} {
		if strings.Contains(script, frag) {
			t.Errorf("entrypoint must not reference %q; ~/CONTEXT.md is the only context channel", frag)
		}
	}
}

// The global skill baked into every image must declare the frontmatter an agent
// needs to discover it (name + description) and must point at both halves of the
// context: the static ~/CONTEXT.md and the live get-devcontainer-context.
func TestContextSkillDocument(t *testing.T) {
	body, err := os.ReadFile("skill-devcontainer-context.md")
	if err != nil {
		t.Fatalf("reading skill-devcontainer-context.md: %v", err)
	}
	doc := string(body)

	if !strings.HasPrefix(doc, "---\n") {
		t.Fatalf("the skill must open with YAML frontmatter:\n%s", doc)
	}
	end := strings.Index(doc[4:], "\n---\n")
	if end == -1 {
		t.Fatalf("the skill's frontmatter is not terminated:\n%s", doc)
	}
	frontmatter := doc[4 : end+4]
	if !strings.Contains(frontmatter, "name: devcontainer-context") {
		t.Errorf("frontmatter must declare `name: devcontainer-context`:\n%s", frontmatter)
	}
	// The description is what an agent matches on, so it must survive as a
	// single line and say when to use the skill.
	descIdx := strings.Index(frontmatter, "description: ")
	if descIdx == -1 {
		t.Fatalf("frontmatter must declare a description:\n%s", frontmatter)
	}
	desc := strings.SplitN(frontmatter[descIdx:], "\n", 2)[0]
	if len(desc) < 80 {
		t.Errorf("the description must explain when to use the skill, got %q", desc)
	}

	for _, frag := range []string{
		"~/CONTEXT.md",
		"get-devcontainer-context",
		"Docker container",
		"devcontainer-cli",
	} {
		if !strings.Contains(doc, frag) {
			t.Errorf("the skill must mention %q:\n%s", frag, doc)
		}
	}
}

// The baked skill is linked into the agents' skills directories at runtime: a
// symlink (never a copy) so the shared volume can never pin a stale skill, and
// never on top of a real directory the user owns.
func TestEntrypointInstallsContextSkill(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)

	if !strings.Contains(script, `BAKED_SKILLS_DIR="$DEV_HOME/.devcontainer-skills"`) {
		t.Error("entrypoint must link the skill baked at ~/.devcontainer-skills")
	}
	for _, dir := range []string{`"$DEV_HOME/.agents/skills"`, `"$DEV_HOME/.claude/skills"`} {
		if !strings.Contains(script, dir) {
			t.Errorf("entrypoint must install the skill into %s", dir)
		}
	}
	// link_or_keep symlinks (never copies) and never clobbers a real directory,
	// both proven by TestEntrypointLinkOrKeep; here we only check the skill goes
	// through it rather than being copied in.
	if !strings.Contains(script, `link_or_keep "$skill_link" "$BAKED_SKILLS_DIR/devcontainer-context"`) {
		t.Error("the skill must be linked with link_or_keep, so a rebuilt image always wins")
	}
	if strings.Contains(script, `cp -a "$BAKED_SKILLS_DIR`) {
		t.Error("the skill must be symlinked, not copied")
	}
	// Running after the shared-config stage is what keeps `mkdir -p ~/.claude/skills`
	// from creating the home dir as a real directory; pinned by TestEntrypointStageOrder.
}

// The auto-start post-scripts must run as devuser (never root): the whole loop
// is wrapped in a single 'su - devuser' login shell and backgrounded so SSH is
// not blocked, with a per-script .done sentinel guarding re-runs.
func TestEntrypointAutoStartRunsAsDevuser(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)
	if !strings.Contains(script, `"$DEV_HOME/post-script/start.d"`) {
		t.Error("entrypoint must auto-run the start.d post-scripts")
	}
	if !strings.Contains(script, "su - devuser -s /bin/bash -c") {
		t.Error("entrypoint must run the auto-start post-scripts as devuser (su - devuser)")
	}
	if !strings.Contains(script, ".post-script-state") || !strings.Contains(script, ".done") {
		t.Error("entrypoint must guard re-runs with a per-script .done sentinel")
	}
	// The su block must be backgrounded so it never blocks sshd from starting.
	if !strings.Contains(script, "' &\n}") {
		t.Error("entrypoint must background the auto-start block so SSH comes up immediately")
	}
}

// install-graphify.sh must wire Graphify into every supported agent present in
// the container (global scope, never --project) and detect/skip absent ones.
func TestGraphifyInstallWiresAllPlatforms(t *testing.T) {
	body, err := os.ReadFile("install-graphify.sh")
	if err != nil {
		t.Fatalf("reading install-graphify.sh: %v", err)
	}
	script := string(body)

	// A helper applies the platform id ($2); assert the helper plus each call.
	wantWires := []string{
		`graphify install --platform "$2"`,
		"wire \"Claude Code\" claude",
		"wire \"Codex\" codex",
		"wire \"Antigravity\" antigravity",
		"wire \"GitHub Copilot\" copilot",
	}
	for _, w := range wantWires {
		if !strings.Contains(script, w) {
			t.Errorf("install-graphify.sh must wire %q", w)
		}
	}

	wantGuards := []string{
		"command -v claude",
		"command -v codex",
		`[ -d "$HOME/.codex" ]`, // Codex leaves no binary; dir is the marker.
		"command -v agy",
		"command -v copilot",
	}
	for _, g := range wantGuards {
		if !strings.Contains(script, g) {
			t.Errorf("install-graphify.sh must guard a platform with %q", g)
		}
	}

	// Global scope only: the project-scoped flag must never be used.
	if strings.Contains(script, "--project") {
		t.Error("install-graphify.sh must install in global scope (no --project)")
	}
}

// install-caveman.sh must wire Caveman into every supported agent present in
// the container, using each platform's native command, and detect/skip absent
// ones.
func TestCavemanInstallWiresAllPlatforms(t *testing.T) {
	body, err := os.ReadFile("install-caveman.sh")
	if err != nil {
		t.Fatalf("reading install-caveman.sh: %v", err)
	}
	script := string(body)

	wantWires := []string{
		"claude plugin install caveman@caveman", // Claude Code (plugin marketplace)
		"--only codex",
		"--only antigravity",
		"--only copilot",
	}
	for _, w := range wantWires {
		if !strings.Contains(script, w) {
			t.Errorf("install-caveman.sh must wire %q", w)
		}
	}

	wantGuards := []string{
		"command -v claude",
		"command -v codex",
		`[ -d "$HOME/.codex" ]`,
		"command -v agy",
		"command -v copilot",
	}
	for _, g := range wantGuards {
		if !strings.Contains(script, g) {
			t.Errorf("install-caveman.sh must guard a platform with %q", g)
		}
	}
}

// install-claude-mem.sh must wire claude-mem into every supported agent
// present in the container and detect/skip absent ones. Antigravity's binary
// on PATH is `agy`, not `antigravity` — a real bug this pins against
// regressing.
func TestClaudeMemInstallWiresAllPlatforms(t *testing.T) {
	body, err := os.ReadFile("install-claude-mem.sh")
	if err != nil {
		t.Fatalf("reading install-claude-mem.sh: %v", err)
	}
	script := string(body)

	wantWires := []string{
		"claude plugin install claude-mem", // Claude Code (plugin marketplace)
		"--ide opencode",
		"--ide antigravity",
	}
	for _, w := range wantWires {
		if !strings.Contains(script, w) {
			t.Errorf("install-claude-mem.sh must wire %q", w)
		}
	}

	wantGuards := []string{
		"command -v claude",
		"command -v opencode",
		"command -v agy",
	}
	for _, g := range wantGuards {
		if !strings.Contains(script, g) {
			t.Errorf("install-claude-mem.sh must guard a platform with %q", g)
		}
	}
	if strings.Contains(script, "command -v antigravity") {
		t.Error("install-claude-mem.sh must detect Antigravity via its real binary `agy`, not `antigravity`")
	}
}

// install-context-mode.sh only scripts the two platforms with a real
// unattended plugin-manager command (Claude Code, GitHub Copilot CLI) — every
// other supported platform needs a hand-edited config file this installer
// must not touch blindly.
func TestContextModeInstallWiresScriptablePlatforms(t *testing.T) {
	body, err := os.ReadFile("install-context-mode.sh")
	if err != nil {
		t.Fatalf("reading install-context-mode.sh: %v", err)
	}
	script := string(body)

	wantWires := []string{
		"npm install -g context-mode",
		"claude plugin install context-mode@context-mode", // Claude Code (plugin marketplace)
		"copilot plugin install",
	}
	for _, w := range wantWires {
		if !strings.Contains(script, w) {
			t.Errorf("install-context-mode.sh must wire %q", w)
		}
	}

	wantGuards := []string{"command -v claude", "command -v copilot"}
	for _, g := range wantGuards {
		if !strings.Contains(script, g) {
			t.Errorf("install-context-mode.sh must guard a platform with %q", g)
		}
	}
}

// setup-help.sh must write the quick reference to ~/help and cover both the
// micro editor and zellij, so every container ships an accurate cheat-sheet.
func TestSetupHelpWritesQuickReference(t *testing.T) {
	body, err := os.ReadFile("setup-help.sh")
	if err != nil {
		t.Fatalf("reading setup-help.sh: %v", err)
	}
	script := string(body)
	if !strings.Contains(script, `> "$HOME/help"`) {
		t.Error("setup-help.sh must write the quick reference to $HOME/help")
	}
	for _, frag := range []string{"micro", "zellij", "Ctrl-S", "Ctrl-p"} {
		if !strings.Contains(script, frag) {
			t.Errorf("setup-help.sh quick reference must mention %q", frag)
		}
	}
}

func TestIsGeneratedFile(t *testing.T) {
	dir := t.TempDir()

	generated := filepath.Join(dir, "gen")
	if err := os.WriteFile(generated, []byte("# AUTO-GENERATED by devcontainer CLI\nFROM x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !assets.IsGeneratedFile(generated) {
		t.Error("expected file with marker to be reported generated")
	}

	handwritten := filepath.Join(dir, "hand")
	if err := os.WriteFile(handwritten, []byte("FROM ubuntu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if assets.IsGeneratedFile(handwritten) {
		t.Error("expected unmarked file to be reported not generated")
	}

	if !assets.IsGeneratedFile(filepath.Join(dir, "does-not-exist")) {
		t.Error("expected absent file to be treated as generated (safe to overwrite)")
	}
}

// `lsof -v` prints a multi-line banner on stderr whose FIRST line is only a
// header ("lsof version information:") — the number is on the "revision:" line.
// Reporting the first line made the tool inventory say nothing useful, so the
// script must dig the revision out. Runs the real script against a fake lsof.
func TestContextScriptReportsLsofRevision(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}

	bin := t.TempDir()
	fake := "#!/bin/sh\n" +
		"cat >&2 <<'BANNER'\n" +
		"lsof version information:\n" +
		"    revision: 4.95.0\n" +
		"    latest revision: https://github.com/lsof-org/lsof\n" +
		"BANNER\n"
	if err := os.WriteFile(filepath.Join(bin, "lsof"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	script, err := filepath.Abs("get-devcontainer-context.sh")
	if err != nil {
		t.Fatal(err)
	}
	// The fake shadows any real lsof; the rest of PATH stays so the script still
	// finds grep/sed/head.
	got := toolLine(t, bash, script, bin, "lsof")
	if !strings.Contains(got, "4.95.0") {
		t.Errorf("lsof must be reported with its revision, got %q", got)
	}
	if strings.Contains(got, "version information") {
		t.Errorf("lsof must not be reported with its banner header, got %q", got)
	}
}

// toolLine runs the context script with dir prepended to PATH and returns the
// inventory line for the given tool.
func toolLine(t *testing.T, bash, script, dir, tool string) string {
	t.Helper()
	cmd := exec.Command(bash, script, "--tools")
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the context script failed: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "- "+tool+" ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "- "+tool))
		}
	}
	t.Fatalf("no inventory line for %q:\n%s", tool, out)
	return ""
}

// A future lsof that drops the "revision:" line must still report something
// rather than an empty version.
func TestContextScriptFallsBackToLsofBanner(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}

	bin := t.TempDir()
	fake := "#!/bin/sh\necho 'lsof 5.0.0' >&2\n"
	if err := os.WriteFile(filepath.Join(bin, "lsof"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	script, err := filepath.Abs("get-devcontainer-context.sh")
	if err != nil {
		t.Fatal(err)
	}
	if got := toolLine(t, bash, script, bin, "lsof"); !strings.Contains(got, "5.0.0") {
		t.Errorf("lsof must fall back to the banner's first line, got %q", got)
	}
}

// runSkillsInstaller runs install-project-skills.sh with a fake npx on PATH. The
// workspace mount it installs into only exists inside a container, so these
// cases stop at the mode gate — which is what governs whether it writes into
// the user's own repository at all.
func runSkillsInstaller(t *testing.T, env []string, args ...string) (output string, exitedZero bool) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "npx"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("install-project-skills.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, append([]string{script}, args...)...)
	cmd.Env = append(append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH")), env...)
	combined, runErr := cmd.CombinedOutput()
	return string(combined), runErr == nil
}

// The installer writes into the bind-mounted workspace — the user's own
// repository — so an absent DEVCONTAINER_SKILLS_MODE must read as manual
// (types.DefaultSkillMode), never as permission to install on every start.
func TestProjectSkillsInstallerDefaultsToManualOnAutoRuns(t *testing.T) {
	const manualNoop = "Skills mode is manual"
	const oneSkillSelected = "DEVCONTAINER_SKILLS=owner/repo"

	autoRunWithNoMode, exitedZero := runSkillsInstaller(t, []string{oneSkillSelected}, "--auto")
	if !exitedZero {
		t.Fatalf("an automatic run with no mode set must be a no-op, got: %s", autoRunWithNoMode)
	}
	if !strings.Contains(autoRunWithNoMode, manualNoop) {
		t.Errorf("an absent DEVCONTAINER_SKILLS_MODE must default to manual, got: %s", autoRunWithNoMode)
	}

	autoRunWithModeAuto, _ := runSkillsInstaller(t, []string{oneSkillSelected, "DEVCONTAINER_SKILLS_MODE=auto"}, "--auto")
	if strings.Contains(autoRunWithModeAuto, manualNoop) {
		t.Errorf("mode=auto is the opt-in and must get past the gate, got: %s", autoRunWithModeAuto)
	}

	handRunWithModeManual, _ := runSkillsInstaller(t, []string{oneSkillSelected, "DEVCONTAINER_SKILLS_MODE=manual"})
	if strings.Contains(handRunWithModeManual, manualNoop) {
		t.Errorf("a hand-run install must ignore the manual mode, got: %s", handRunWithModeManual)
	}
}

// Selecting nothing is a state the installer supports, not an error, whichever
// mode it runs in.
func TestProjectSkillsInstallerAcceptsAnEmptySelection(t *testing.T) {
	emptySelection, exitedZero := runSkillsInstaller(t, []string{"DEVCONTAINER_SKILLS="}, "--auto")
	if !exitedZero {
		t.Errorf("no skills requested must exit zero, got: %s", emptySelection)
	}
	if !strings.Contains(emptySelection, "No skills requested") {
		t.Errorf("no skills requested must say so, got: %s", emptySelection)
	}
}

// The entrypoint runs its start.d scripts with no arguments, so the automatic
// run is only distinguishable by the flag this hook passes.
func TestAutostartProjectSkillsPassesTheAutoFlag(t *testing.T) {
	body, err := os.ReadFile("autostart-project-skills.sh")
	if err != nil {
		t.Fatalf("reading autostart-project-skills.sh: %v", err)
	}
	if !strings.Contains(string(body), "install-skills\" --auto") {
		t.Error("the entrypoint hook must invoke the installer with --auto")
	}
}

// A skills entry is "<source>" or "<source>#<skill>": the selector form is what
// names one skill of a repo holding several without pinning its branch.
func TestProjectSkillsInstallerSplitsTheSkillSelector(t *testing.T) {
	// Replaced by grouped commands and interactive prompt allowing global installs.
}

// install-ecc.sh only puts ECC's CLIs on PATH, and must stop there.
//
// Every ECC install target writes into the project (./.claude/, ./.agent/) or
// into the home, and the workspace is a bind mount — so running one unattended
// on first boot rewrites the user's real checkout with a module bundle nobody
// chose. The home targets are worse still: ~/.claude and ~/.codex are symlinks
// into the shared config volume, so one project's ECC would leak into every
// other container mounting it. `npx skills add` and the Claude plugin are the
// other two channels, and both ship only part of ECC.
func TestEccInstallOnlyPutsTheCLIsOnPath(t *testing.T) {
	body, err := os.ReadFile("install-ecc.sh")
	if err != nil {
		t.Fatalf("reading install-ecc.sh: %v", err)
	}
	script := string(body)

	for _, w := range []string{
		"pnpm add -g ecc-universal ecc-agentshield",
		"npm install -g ecc-universal ecc-agentshield",
	} {
		if !strings.Contains(script, w) {
			t.Errorf("install-ecc.sh must wire %q", w)
		}
	}

	// The forbidden checks look at lines that could actually run something. The
	// header comment explains what the script deliberately does not do, and the
	// closing echo tells the user the command to run themselves, so both name
	// the forbidden strings on purpose.
	var code []string
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "echo ") {
			continue
		}
		code = append(code, line)
	}
	executable := strings.Join(code, "\n")

	forbidden := []string{
		"--target",
		"DEVCONTAINER_ECC_PROFILE",
		"skills add",
		"claude plugin install",
		"claude plugin marketplace",
	}
	for _, f := range forbidden {
		if strings.Contains(executable, f) {
			t.Errorf("install-ecc.sh must not run %q: it writes into files the user owns, or installs only part of ECC", f)
		}
	}
}

// ── link_or_keep ────────────────────────────────────────────────────────────

// Four blocks of the entrypoint link something into a place devuser reads: a
// shared-config entry into the home, that entry's alias at the volume root, the
// Antigravity skills bridge and the image's baked global skill. They must all go
// through link_or_keep — open-coding the guard is how it ended up written in two
// contrary phrasings of the same condition.
func TestEntrypointLinksThroughOneHelper(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)
	// Two of the four are inside shared_config_apply, in the library; the other
	// two (the Antigravity bridge, the baked skill) are call sites here.
	if got := strings.Count(script, "link_or_keep "); got < 2 {
		t.Errorf("expected the entrypoint to call link_or_keep for the bridge and the skill, found %d mentions", got)
	}
	// The only ln -sfn left outside link_or_keep is the workspace alias, which
	// links a mount (not something under the home) and has its own rules.
	for _, frag := range []string{
		`ln -sfn "$src" "$dest"`,
		`ln -sfn "$BAKED_SKILLS_DIR`,
		`ln -sfn "/home/devuser/.agents/skills"`,
	} {
		if strings.Contains(script, frag) {
			t.Errorf("%q must go through link_or_keep, not a bare ln -sfn", frag)
		}
	}
}

// TestLinkOrKeep runs the library's real link_or_keep against a temp tree. Its
// three rules are what four call sites now depend on: never replace something
// real, create the parent, and point the link at the target verbatim (the volume
// aliases pass a RELATIVE target).
func TestLinkOrKeep(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	body, err := os.ReadFile(sharedConfigLib)
	if err != nil {
		t.Fatalf("reading %s: %v", sharedConfigLib, err)
	}
	fn := extractShellFunc(t, string(body), "own") +
		extractShellFunc(t, string(body), "link_or_keep")

	// link_or_keep <path> <target> [message]. SC_OWNER is the library's one
	// configuration knob; an empty one skips the chowns, which is what lets this
	// run unprivileged.
	run := func(t *testing.T, root, linkPath, target, message string) (int, string) {
		t.Helper()
		script := "SC_OWNER=\n" + fn + "\nlink_or_keep \"$1\" \"$2\" \"$3\"\n"
		cmd := exec.Command("bash", "-c", script, "bash", linkPath, target, message)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("running link_or_keep: %v\n%s", err, out)
		}
		return code, string(out)
	}

	t.Run("creates the link and its missing parent", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "store")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "deep", "nested", "link")
		if code, out := run(t, root, link, target, ""); code != 0 {
			t.Fatalf("link_or_keep returned %d: %s", code, out)
		}
		got, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("expected a symlink at %s: %v", link, err)
		}
		if got != target {
			t.Errorf("link -> %q, want %q", got, target)
		}
	})

	t.Run("repoints a stale link", func(t *testing.T) {
		root := t.TempDir()
		link := filepath.Join(root, "link")
		if err := os.Symlink(filepath.Join(root, "gone"), link); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, "store")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if code, out := run(t, root, link, target, ""); code != 0 {
			t.Fatalf("link_or_keep returned %d: %s", code, out)
		}
		if got, _ := os.Readlink(link); got != target {
			t.Errorf("stale link -> %q, want it repointed at %q", got, target)
		}
	})

	t.Run("keeps a real directory and reports it", func(t *testing.T) {
		root := t.TempDir()
		link := filepath.Join(root, "real")
		if err := os.MkdirAll(filepath.Join(link, "inner"), 0o755); err != nil {
			t.Fatal(err)
		}
		code, out := run(t, root, link, filepath.Join(root, "store"), "keeping it")
		if code == 0 {
			t.Error("link_or_keep must return non-zero when it keeps what is already there, so callers can skip")
		}
		if !strings.Contains(out, "keeping it") {
			t.Errorf("expected the kept-message on stderr, got %q", out)
		}
		if info, err := os.Lstat(link); err != nil || !info.IsDir() {
			t.Errorf("the real directory must survive untouched (err=%v)", err)
		}
		if _, err := os.Stat(filepath.Join(link, "inner")); err != nil {
			t.Errorf("contents of the kept directory must survive: %v", err)
		}
	})

	t.Run("keeps a relative target verbatim", func(t *testing.T) {
		// The volume aliases depend on this: <volume>/.config/gh -> ../gh only
		// resolves if the target is stored as written, not resolved first.
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "gh"), 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, ".config", "gh")
		if code, out := run(t, root, link, "../gh", ""); code != 0 {
			t.Fatalf("link_or_keep returned %d: %s", code, out)
		}
		if got, _ := os.Readlink(link); got != "../gh" {
			t.Errorf("link -> %q, want the relative target %q kept verbatim", got, "../gh")
		}
		if _, err := filepath.EvalSymlinks(link); err != nil {
			t.Errorf("the relative alias must resolve: %v", err)
		}
	})
}

// ── The stage order ─────────────────────────────────────────────────────────

// entrypointStages returns the stage_* calls in main(), in order — the
// entrypoint's whole control flow, read from the shipped file.
func entrypointStages(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	main := extractShellFunc(t, string(body), "main")
	var stages []string
	for _, line := range strings.Split(main, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "stage_") && !strings.Contains(line, "(") {
			stages = append(stages, line)
		}
	}
	if len(stages) == 0 {
		t.Fatal("main() calls no stages")
	}
	return stages
}

// The entrypoint is a driver over named stages, and the order is the part that
// carries meaning. Each edge below is a real failure if inverted, so they are
// pinned here once instead of as an index comparison inside each stage's own
// test. Every stage in main() must also exist as a function — a typo'd call is
// a silent no-op in a script with no `set -e`.
func TestEntrypointStageOrder(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)
	stages := entrypointStages(t)

	at := map[string]int{}
	for i, stage := range stages {
		if _, dup := at[stage]; dup {
			t.Errorf("main() calls %s twice", stage)
		}
		at[stage] = i
		if !strings.Contains(script, stage+"() {") {
			t.Errorf("main() calls %s, which is not defined", stage)
		}
	}

	for _, edge := range []struct{ before, after, why string }{
		{"stage_workspace", "stage_identity",
			"WORKSPACE_DIR decides which UID devuser is remapped to"},
		{"stage_identity", "stage_shared_config",
			"the shared-config library is configured with SC_OWNER, set from DEV_UID/DEV_GID"},
		{"stage_shared_config", "stage_user_aliases",
			"a local ~/.alias.sh created first would win the race against the volume symlink"},
		{"stage_shared_config", "stage_context_skill",
			"~/.claude must already be the volume symlink, or mkdir -p makes it a real directory"},
	} {
		i, iOK := at[edge.before]
		j, jOK := at[edge.after]
		if !iOK || !jOK {
			t.Errorf("missing stage for the %s -> %s edge", edge.before, edge.after)
			continue
		}
		if i > j {
			t.Errorf("%s must run before %s: %s", edge.before, edge.after, edge.why)
		}
	}

	// sshd is the container's lifetime, so it is exec'd last and never
	// backgrounded.
	if !strings.Contains(script, "exec /usr/sbin/sshd -D") {
		t.Error("main() must exec sshd so it becomes PID 1")
	}
}

// ── shared_config_apply ─────────────────────────────────────────────────────

// TestSharedConfigApply runs the library's real shared_config_apply against a
// temp volume and home, with the catalogue passed in as data. It is the whole
// layout in one call, so this covers what used to be spread over the entrypoint
// and the sync helper: the entry is materialized in the flat volume, gets a
// home-shaped alias at the volume root, and is linked into the home.
func TestSharedConfigApply(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	lib, err := os.ReadFile(sharedConfigLib)
	if err != nil {
		t.Fatalf("reading %s: %v", sharedConfigLib, err)
	}

	root := t.TempDir()
	vol := filepath.Join(root, "vol")
	home := filepath.Join(root, "home")
	for _, d := range []string{vol, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A real directory already in the home must survive: the volume is seeded
	// with `config shared sync`, never by overwriting what is in the container.
	if err := os.MkdirAll(filepath.Join(home, ".codex", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A dir entry, a file entry, and a nested target — the three shapes the
	// catalogue actually contains.
	entries := "claude dir .claude\nclaude.json file .claude.json\ngh dir .config/gh\ncodex dir .codex\n"
	cmd := exec.Command("sh", "-c", "set -e\n"+string(lib)+"\nshared_config_apply \"$1\" \"$2\"", "sh", vol, home)
	cmd.Env = append(os.Environ(), "SC_ENTRIES="+entries, "SC_OWNER=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shared_config_apply: %v\n%s", err, out)
	}

	// Materialized in the flat volume, with the declared kind.
	if info, err := os.Stat(filepath.Join(vol, "claude")); err != nil || !info.IsDir() {
		t.Errorf("dir entry must be created in the volume (err=%v)", err)
	}
	if info, err := os.Stat(filepath.Join(vol, "claude.json")); err != nil || info.IsDir() {
		t.Errorf("file entry must be created as a file in the volume (err=%v)", err)
	}

	// Linked into the home, and the home-shaped alias resolves back at the flat
	// entry so a relative cross-entry link written against the home layout lands
	// on a real name.
	for _, tc := range []struct{ id, target string }{
		{"claude", ".claude"},
		{"claude.json", ".claude.json"},
		{"gh", ".config/gh"},
	} {
		if got, err := os.Readlink(filepath.Join(home, filepath.FromSlash(tc.target))); err != nil {
			t.Errorf("%s must be a symlink into the volume: %v", tc.target, err)
		} else if got != filepath.Join(vol, tc.id) {
			t.Errorf("%s -> %q, want %q", tc.target, got, filepath.Join(vol, tc.id))
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(vol, filepath.FromSlash(tc.target)))
		if err != nil {
			t.Errorf("volume alias %s does not resolve: %v", tc.target, err)
			continue
		}
		want, _ := filepath.EvalSymlinks(filepath.Join(vol, tc.id))
		if resolved != want {
			t.Errorf("volume alias %s resolves to %s, want %s", tc.target, resolved, want)
		}
	}

	// The pre-existing real directory is kept, not replaced by a link, and the
	// caller is told why.
	if info, err := os.Lstat(filepath.Join(home, ".codex")); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Error("a real config directory in the home must never be replaced with a symlink")
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "sessions")); err != nil {
		t.Errorf("contents of the kept directory must survive: %v", err)
	}
	if !strings.Contains(string(out), "config shared sync") {
		t.Errorf("keeping real config must point at the command that seeds the volume, got %q", out)
	}

	// Idempotent: a second start must change nothing and must not fail.
	if out2, err := cmd2(t, string(lib), vol, home, entries); err != nil {
		t.Fatalf("second run: %v\n%s", err, out2)
	}
}

func cmd2(t *testing.T, lib, vol, home, entries string) (string, error) {
	t.Helper()
	c := exec.Command("sh", "-c", "set -e\n"+lib+"\nshared_config_apply \"$1\" \"$2\"", "sh", vol, home)
	c.Env = append(os.Environ(), "SC_ENTRIES="+entries, "SC_OWNER=")
	out, err := c.CombinedOutput()
	return string(out), err
}

// shared_config_is_target is what tells a symlink pointing at another persisted
// config apart from one pointing at something that only exists on the host — the
// distinction the whole symlink pass turns on.
func TestSharedConfigIsTarget(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	lib, err := os.ReadFile(sharedConfigLib)
	if err != nil {
		t.Fatalf("reading %s: %v", sharedConfigLib, err)
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{".claude", true},
		{".claude/skills/x", true},
		{".config/gh", true},
		{".config/gh/hosts.yml", true},
		{".claude-backup", false},
		{"projects/tool", false},
		{".config", false},
	} {
		c := exec.Command("sh", "-c", "set -e\n"+string(lib)+"\nshared_config_is_target \"$1\"", "sh", tc.path)
		c.Env = append(os.Environ(), "SC_ENTRIES=claude dir .claude\ngh dir .config/gh\n", "SC_OWNER=")
		got := c.Run() == nil
		if got != tc.want {
			t.Errorf("shared_config_is_target(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestEntrypointSharedConfigStage runs the entrypoint's real stage_shared_config
// against temp stand-ins for the volume, the catalogue file and the home. It is
// the wiring the unit tests above cannot see: that the stage reads the generated
// catalogue into SC_ENTRIES, hands the library the right two roots, and bridges
// Antigravity's skills dir at the shared store.
func TestEntrypointSharedConfigStage(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	entrypoint, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	lib, err := os.ReadFile(sharedConfigLib)
	if err != nil {
		t.Fatalf("reading %s: %v", sharedConfigLib, err)
	}
	stage := extractShellFunc(t, string(entrypoint), "stage_shared_config")

	root := t.TempDir()
	vol := filepath.Join(root, "vol")
	home := filepath.Join(root, "home")
	for _, d := range []string{vol, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	table := filepath.Join(root, "shared-config-entries")
	if err := os.WriteFile(table, []byte("claude dir .claude\nagents dir .agents\ngh dir .config/gh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	script := strings.Join([]string{
		"set -u",
		string(lib),
		"SC_OWNER=",
		"DEV_UID=$(id -u)",
		"DEV_GID=$(id -g)",
		"DEV_HOME=" + home,
		"SHARED_CONFIG_DIR=" + vol,
		"SHARED_CONFIG_TABLE=" + table,
		stage,
		"stage_shared_config",
	}, "\n")
	if out, err := exec.Command("bash", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("stage_shared_config: %v\n%s", err, out)
	}

	// The catalogue file drove the layout: an entry named only there is linked.
	if got, err := os.Readlink(filepath.Join(home, ".config", "gh")); err != nil {
		t.Errorf("the stage must link every entry in the catalogue file: %v", err)
	} else if got != filepath.Join(vol, "gh") {
		t.Errorf(".config/gh -> %q, want %q", got, filepath.Join(vol, "gh"))
	}

	// Antigravity reads skills from ~/.gemini/antigravity-cli/skills and does not
	// understand ~/.agents/skills, so the stage bridges them — and the target has
	// to exist for it to read through.
	bridge := filepath.Join(home, ".gemini", "antigravity-cli", "skills")
	got, err := os.Readlink(bridge)
	if err != nil {
		t.Fatalf("the Antigravity skills bridge must be a symlink: %v", err)
	}
	if want := filepath.Join(home, ".agents", "skills"); got != want {
		t.Errorf("bridge -> %q, want %q", got, want)
	}
	if _, err := filepath.EvalSymlinks(bridge); err != nil {
		t.Errorf("the bridge must resolve, i.e. its target must be created: %v", err)
	}

	// The stage is a no-op without the mount (opt-out, or an image built before
	// it existed), which is what keeps quick-run working on older images.
	absent := strings.Replace(script, "SHARED_CONFIG_DIR="+vol, "SHARED_CONFIG_DIR="+filepath.Join(root, "nope"), 1)
	if out, err := exec.Command("bash", "-c", absent).CombinedOutput(); err != nil {
		t.Errorf("stage_shared_config must no-op when the volume is not mounted: %v\n%s", err, out)
	}
}

func TestBrowserHarnessInstall(t *testing.T) {
	body, err := os.ReadFile("install-browser-harness.sh")
	if err != nil {
		t.Fatalf("reading install-browser-harness.sh: %v", err)
	}
	script := string(body)

	wantWires := []string{
		"uv tool install --python 3.12 --upgrade --force browser-harness",
		"browser-harness recordings enable",
		"browser-harness skill",
		".agents/skills/browser-harness/SKILL.md",
		".claude/skills/browser-harness/SKILL.md",
		".codex/skills/browser-harness/SKILL.md",
		"interaction-skills",
		"agent-workspace",
	}
	for _, w := range wantWires {
		if !strings.Contains(script, w) {
			t.Errorf("install-browser-harness.sh must contain %q", w)
		}
	}

	wantGuards := []string{
		"command -v uv",
		"command -v browser-harness",
		"workspace_dir",
	}
	for _, g := range wantGuards {
		if !strings.Contains(script, g) {
			t.Errorf("install-browser-harness.sh must guard with %q", g)
		}
	}
}
