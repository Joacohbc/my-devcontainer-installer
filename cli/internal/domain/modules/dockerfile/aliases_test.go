package dockerfile

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// The aliases module must ship all four of its files — the three embedded
// assets plus the generated CONTEXT.md — and wire both alias layers into every
// rc file.
func TestAliasesModuleRender(t *testing.T) {
	out := AliasesModule.Render(nil)

	for _, frag := range []string{
		"COPY alias.sh /home/devuser/.devcontainer_aliases.sh",
		"COPY get-devcontainer-context.sh /home/devuser/.local/bin/get-devcontainer-context",
		"COPY CONTEXT.md /home/devuser/CONTEXT.md",
		"COPY skill-devcontainer-context.md /home/devuser/.devcontainer-skills/devcontainer-context/SKILL.md",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("aliases module must contain %q:\n%s", frag, out)
		}
	}

	// The context script must land executable, and the alias file, CONTEXT.md
	// and the skill world-readable, all owned by devuser.
	for _, frag := range []string{
		"chmod 0755 /home/devuser/.local/bin/get-devcontainer-context",
		"chmod 0644 /home/devuser/.devcontainer_aliases.sh",
		"/home/devuser/.devcontainer-skills/devcontainer-context/SKILL.md && \\",
		"chown -R devuser:devuser",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("aliases module must set permissions (%q):\n%s", frag, out)
		}
	}

	// Both layers are sourced from every rc file, baked defaults first so the
	// user's own file (sourced last) overrides them.
	if !strings.Contains(out, `. \$HOME/`+BakedAliasFile) {
		t.Errorf("aliases module must source the baked defaults:\n%s", out)
	}
	if !strings.Contains(out, `if [ -r \$HOME/`+UserAliasFile+` ]; then . \$HOME/`+UserAliasFile+`; fi`) {
		t.Errorf("aliases module must guard-source the user alias file:\n%s", out)
	}
	if i, j := strings.Index(out, BakedAliasFile+`' >>`), strings.Index(out, UserAliasFile+`; fi' >>`); i == -1 || j == -1 || i > j {
		t.Errorf("the user alias file must be sourced AFTER the baked defaults (baked=%d user=%d):\n%s", i, j, out)
	}
}

// The module is build-identical for every project: it takes no options and the
// agent aliases live unconditionally in the shipped script, so Render never
// depends on opts.
func TestAliasesModuleRenderIsOptionIndependent(t *testing.T) {
	if len(AliasesModule.Options) != 0 {
		t.Errorf("aliases module must expose no options, got %v", AliasesModule.Options)
	}
	base := AliasesModule.Render(nil)
	for _, opts := range []map[string]any{
		{"yoloAgents": false},
		{"anything": true},
	} {
		if got := AliasesModule.Render(opts); got != base {
			t.Errorf("Render must ignore options; %v changed the output", opts)
		}
	}
	// The gate mechanism is gone: nothing touches a flag file anymore.
	if strings.Contains(base, "devcontainer_agents_yolo") {
		t.Errorf("the agent-yolo flag file must no longer be referenced:\n%s", base)
	}
}

// The global agent skill is baked into the image under a CLI-owned directory,
// never straight into an agent's own config dir: those are symlinks into the
// shared volume, so an image-time write there would either be shadowed at
// runtime or leak a stale skill into every other container. The entrypoint does
// the linking.
func TestAliasesModuleBakesSkillOutsideAgentConfigDirs(t *testing.T) {
	out := AliasesModule.Render(nil)

	skillPath := "/home/devuser/" + SkillsDir + "/" + ContextSkillName + "/SKILL.md"
	if !strings.Contains(out, "COPY "+ContextSkillAsset+" "+skillPath) {
		t.Errorf("the skill must be baked at %s:\n%s", skillPath, out)
	}
	for _, agentDir := range []string{"/home/devuser/.claude", "/home/devuser/.agents", "/home/devuser/.gemini"} {
		if strings.Contains(out, agentDir) {
			t.Errorf("the module must not write into %s (shared volume, wired at runtime):\n%s", agentDir, out)
		}
	}
}

func TestAliasesModuleSpec(t *testing.T) {
	if AliasesModule.ID != types.ModuleAliases {
		t.Errorf("unexpected module id %q", AliasesModule.ID)
	}
	if !AliasesModule.Always {
		t.Error("aliases module must be always-on: every container needs kill_port and ~/CONTEXT.md")
	}
	if AliasesModule.Category != types.CategoryBase {
		t.Errorf("aliases module must be in CategoryBase (it appends to rc files rewritten by base), got %q", AliasesModule.Category)
	}
	// CONTEXT.md is generated, not embedded, so it must NOT be in CopyFiles —
	// preflight would look for it in the embedded FS and report it missing.
	want := []string{"alias.sh", "get-devcontainer-context.sh", "skill-devcontainer-context.md"}
	if len(AliasesModule.CopyFiles) != len(want) {
		t.Fatalf("expected CopyFiles %v, got %v", want, AliasesModule.CopyFiles)
	}
	for _, w := range want {
		found := false
		for _, got := range AliasesModule.CopyFiles {
			if got == w {
				found = true
			}
		}
		if !found {
			t.Errorf("CopyFiles must include %q, got %v", w, AliasesModule.CopyFiles)
		}
	}
	if len(AliasesModule.Options) != 0 {
		t.Errorf("aliases module must expose no options, got %v", AliasesModule.Options)
	}
}
