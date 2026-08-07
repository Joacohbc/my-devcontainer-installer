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
	if strings.Contains(script, `chown -R "$WS_UID:$WS_GID" /home/devuser`) {
		t.Error("entrypoint must not chown the home to the workspace UID; use devuser's actual UID")
	}
	if !strings.Contains(script, `chown -R "$DEV_UID:$DEV_GID" /home/devuser`) {
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

			localizedBlock := strings.ReplaceAll(workspaceBlock, "/workspaces", mountRoot)
			localizedBlock = strings.ReplaceAll(localizedBlock, "/workspace", aliasRoot)
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

// extractWorkspaceBlock returns the entrypoint's workspace-resolution block
// (the WORKSPACE_DIR assignment through the end of its loop), so the test runs
// the real thing instead of a copy.
func extractWorkspaceBlock(t *testing.T, script string) string {
	t.Helper()
	const start = "WORKSPACE_MOUNT_ROOT=/workspaces\n"
	i := strings.Index(script, start)
	if i < 0 {
		t.Fatal("entrypoint has no workspace resolution block")
	}
	rest := script[i:]
	end := strings.Index(rest, "\ndone\n")
	if end < 0 {
		t.Fatal("could not find the end of the workspace resolution loop")
	}
	return rest[:end+len("\ndone\n")]
}

// The shared-config volume is flat (one dir per entry id) while the home it is
// symlinked into is not, so a relative cross-entry link written against the home
// layout (~/.claude/skills/x -> ../../.agents/skills/x, what `npx skills add -g`
// writes) has no name to land on inside the volume and dangles. The entrypoint
// must mirror the home layout at the volume root so it does — without clobbering
// anything real sitting at that name.
func TestEntrypointMirrorsHomeLayoutAtVolumeRoot(t *testing.T) {
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	script := string(body)
	for _, frag := range []string{
		`mirror_entry_at_home_name "$entry_id" "$entry_target"`,
		`if [ -e "$alias_path" ] && [ ! -L "$alias_path" ]; then return 0; fi`,
		`ln -sfn "$(prefix_to_volume_root "$entry_target")$entry_id" "$alias_path"`,
	} {
		if !strings.Contains(script, frag) {
			t.Errorf("entrypoint must mirror the home layout at the volume root (missing %q)", frag)
		}
	}
}

// TestEntrypointVolumeAliasesResolve runs the entrypoint's own prefix_to_volume_root
// helper to prove the aliases it builds point back at the flat entry, for a
// nested target (.config/gh -> ../gh) as much as a top-level one (.claude ->
// claude). An alias with the wrong number of "../" hops is silently dangling,
// which is exactly the failure it exists to prevent.
func TestEntrypointVolumeAliasesResolve(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	body, err := os.ReadFile(embeddedScript)
	if err != nil {
		t.Fatalf("reading %s: %v", embeddedScript, err)
	}
	fn := extractShellFunc(t, string(body), "prefix_to_volume_root")

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
ln -sfn "$(prefix_to_volume_root "$2")$3" "$alias_path"`
		cmd := exec.Command("bash", "-c", script, "bash", vol, tc.target, tc.id)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("prefix_to_volume_root(%s): %v\n%s", tc.target, err, out)
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
func extractShellFunc(t *testing.T, script, name string) string {
	t.Helper()
	start := strings.Index(script, name+"() {")
	if start < 0 {
		t.Fatalf("entrypoint has no %s() function", name)
	}
	rest := script[start:]
	end := strings.Index(rest, "\n    }\n")
	if end < 0 {
		t.Fatalf("could not find the end of %s()", name)
	}
	return rest[:end+len("\n    }\n")]
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
	if !strings.Contains(script, "/home/devuser/.gemini/antigravity-cli") {
		t.Error("entrypoint must target the Antigravity CLI skills dir under ~/.gemini/antigravity-cli")
	}
	if !strings.Contains(script, `ln -sfn "/home/devuser/.agents/skills"`) {
		t.Error("entrypoint must symlink Antigravity's skills dir to the shared ~/.agents/skills")
	}
	if !strings.Contains(script, `[ ! -e "$ag_skills_link" ] || [ -L "$ag_skills_link" ]`) {
		t.Error("entrypoint must not clobber a real (non-symlink) Antigravity skills dir")
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

	if !strings.Contains(script, `[ ! -e /home/devuser/.alias.sh ]`) {
		t.Error("entrypoint must only create ~/.alias.sh when nothing is there (a symlink counts)")
	}
	if !strings.Contains(script, `su - devuser -c 'cat > "$HOME/.alias.sh"'`) {
		t.Error("entrypoint must write ~/.alias.sh as devuser, not root")
	}
	if !strings.Contains(script, `chown "$DEV_UID:$DEV_GID" /home/devuser/.alias.sh`) {
		t.Error("entrypoint must leave ~/.alias.sh owned by devuser's actual UID/GID")
	}

	// The fallback must come after the shared-config block, or it would win the
	// race and the volume symlink would never be created.
	if shared, alias := strings.Index(script, "SHARED_CONFIG_ENTRIES"), strings.Index(script, "/home/devuser/.alias.sh"); shared == -1 || alias == -1 || shared > alias {
		t.Errorf("the ~/.alias.sh fallback must run after the shared-config symlinks (shared=%d alias=%d)", shared, alias)
	}
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

	if !strings.Contains(script, "/home/devuser/.devcontainer-skills") {
		t.Error("entrypoint must link the skill baked at ~/.devcontainer-skills")
	}
	for _, dir := range []string{"/home/devuser/.agents/skills", "/home/devuser/.claude/skills"} {
		if !strings.Contains(script, dir) {
			t.Errorf("entrypoint must install the skill into %s", dir)
		}
	}
	if !strings.Contains(script, `ln -sfn "$BAKED_SKILLS_DIR/devcontainer-context" "$skill_link"`) {
		t.Error("the skill must be symlinked, not copied, so a rebuilt image always wins")
	}
	if !strings.Contains(script, `if [ -e "$skill_link" ] && [ ! -L "$skill_link" ]; then`) {
		t.Error("entrypoint must not clobber a real (non-symlink) skill of the same name")
	}
	// The .claude/.agents dirs become symlinks into the shared volume in the
	// block above; linking earlier would create them as real dirs and the volume
	// entries would never be wired up.
	if shared, skill := strings.Index(script, "SHARED_CONFIG_ENTRIES"), strings.Index(script, "BAKED_SKILLS_DIR"); shared == -1 || skill == -1 || shared > skill {
		t.Errorf("the skill links must be created after the shared-config block (shared=%d skill=%d)", shared, skill)
	}
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
	if !strings.Contains(script, "/home/devuser/post-script/start.d") {
		t.Error("entrypoint must auto-run the start.d post-scripts")
	}
	if !strings.Contains(script, "su - devuser -s /bin/bash -c") {
		t.Error("entrypoint must run the auto-start post-scripts as devuser (su - devuser)")
	}
	if !strings.Contains(script, ".post-script-state") || !strings.Contains(script, ".done") {
		t.Error("entrypoint must guard re-runs with a per-script .done sentinel")
	}
	// The su block must be backgrounded so it never blocks sshd from starting.
	if !strings.Contains(script, "' &\nfi") {
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
		"command -v antigravity",
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
		"command -v antigravity",
		"command -v copilot",
	}
	for _, g := range wantGuards {
		if !strings.Contains(script, g) {
			t.Errorf("install-caveman.sh must guard a platform with %q", g)
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
