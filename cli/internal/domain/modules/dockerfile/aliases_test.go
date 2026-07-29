package dockerfile

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// The aliases module must ship all three of its assets and wire both alias
// layers into every rc file.
func TestAliasesModuleRender(t *testing.T) {
	out := AliasesModule.Render(nil)

	for _, frag := range []string{
		"COPY alias.sh /home/devuser/.devcontainer_aliases.sh",
		"COPY get-devcontainer-context.sh /home/devuser/.local/bin/get-devcontainer-context",
		"COPY setup-context.sh /tmp/setup-context.sh",
		"rm /tmp/setup-context.sh",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("aliases module must contain %q:\n%s", frag, out)
		}
	}

	// The context script must land executable, and the alias file world-readable.
	for _, frag := range []string{
		"chmod 0755 /home/devuser/.local/bin/get-devcontainer-context",
		"chmod 0644 /home/devuser/.devcontainer_aliases.sh",
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

// yoloAgents only ever changes whether the flag file alias.sh keys off is
// created — the shipped script itself is never templated.
func TestAliasesModuleYoloAgentsOption(t *testing.T) {
	flagTouch := `touch \$HOME/` + YoloAgentsFlagFile

	on := AliasesModule.Render(map[string]any{"yoloAgents": true})
	if !strings.Contains(on, flagTouch) {
		t.Errorf("yoloAgents=true must create the flag file:\n%s", on)
	}

	off := AliasesModule.Render(map[string]any{"yoloAgents": false})
	if strings.Contains(off, flagTouch) {
		t.Errorf("yoloAgents=false must NOT create the flag file:\n%s", off)
	}

	// Default (nil options) is on.
	if !strings.Contains(AliasesModule.Render(nil), flagTouch) {
		t.Error("yoloAgents must default to true")
	}

	// Everything else is identical between the two, so the option can never
	// change which files are installed.
	if strings.Count(on, "COPY ") != strings.Count(off, "COPY ") {
		t.Errorf("yoloAgents must not change the COPY set:\non=%s\noff=%s", on, off)
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
	want := []string{"alias.sh", "setup-context.sh", "get-devcontainer-context.sh"}
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
	if len(AliasesModule.Options) != 1 || AliasesModule.Options[0].ID != "yoloAgents" {
		t.Errorf("aliases module must expose exactly the yoloAgents option, got %v", AliasesModule.Options)
	}
}
