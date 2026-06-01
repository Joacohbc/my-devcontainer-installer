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
