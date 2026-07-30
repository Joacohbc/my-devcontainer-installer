package dockerfile

import (
	"strings"
	"testing"
)

// emitShellInit must collapse the init-file write and the rc-file sourcing into
// a single RUN layer (both run as devuser).
func TestEmitShellInitSingleRun(t *testing.T) {
	out := emitShellInit(".x_init.sh", []string{`export PATH="$HOME/.x:$PATH"`})
	if got := strings.Count(out, "RUN "); got != 1 {
		t.Errorf("expected a single RUN, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "for f in .zshrc .bashrc .profile") {
		t.Errorf("expected the rc files to be sourced:\n%s", out)
	}
	if !strings.Contains(out, ".x_init.sh") {
		t.Errorf("expected the init file name to appear:\n%s", out)
	}
}

// emitShellSources only appends source lines — it never writes the sourced
// files — and folds every source into one RUN layer.
func TestEmitShellSources(t *testing.T) {
	out := emitShellSources(
		shellSource{File: ".a.sh"},
		shellSource{File: ".b.sh", Guarded: true},
	)
	if got := strings.Count(out, "RUN "); got != 1 {
		t.Errorf("expected a single RUN, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "for f in .zshrc .bashrc .profile") {
		t.Errorf("expected the rc files to be sourced:\n%s", out)
	}
	if strings.Contains(out, "printf") {
		t.Errorf("emitShellSources must not write the sourced files:\n%s", out)
	}
	if !strings.Contains(out, `. \$HOME/.a.sh`) {
		t.Errorf("expected an unguarded source for .a.sh:\n%s", out)
	}
	// A guarded source uses `if …; then …; fi` (not `[ … ] && …`) so a missing
	// file leaves the rc file's exit status at 0.
	if !strings.Contains(out, `if [ -r \$HOME/.b.sh ]; then . \$HOME/.b.sh; fi`) {
		t.Errorf("expected a guarded source for .b.sh:\n%s", out)
	}
	// Order is preserved: later sources win, so they must be appended last.
	if i, j := strings.Index(out, ".a.sh"), strings.Index(out, ".b.sh"); i > j {
		t.Errorf("sources must be appended in the given order:\n%s", out)
	}
}
