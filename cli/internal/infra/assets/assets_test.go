package assets_test

import (
	"os"
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
	// history does not collide in the shared volume) with /workspace aliased to
	// it; the UID remap must operate on the resolved dir, not a hardcoded path.
	if !strings.Contains(script, "for _ws in /workspaces/*") {
		t.Error("entrypoint must resolve the project mount under /workspaces/")
	}
	if !strings.Contains(script, "ln -s \"$_ws\" /workspace") {
		t.Error("entrypoint must alias /workspace to the resolved project dir")
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
