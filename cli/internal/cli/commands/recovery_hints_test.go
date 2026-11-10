package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A recovery hint is the highest-signal documentation there is: it arrives at
// the exact moment the caller is stuck. Pointing one at the bare
// 'devcontainer-cli' — the interactive wizard — is therefore the worst possible
// advice for a non-interactive caller, which hangs on it with no way to answer.
// Every hint must name a command that does not prompt: 'workspace up', 'workspace start',
// 'agent create'.
//
// This walks the source rather than the built help, because the strings live in
// fmt.Errorf and warning calls spread across internal layers.
func TestErrorMessagesDoNotPointAtTheWizard(t *testing.T) {
	// The bare binary name inside quotes, not followed by a subcommand.
	bareInvocation := regexp.MustCompile(`'devcontainer-cli'`)
	// Obsolete flat root commands that must now use canonical 6 domains.
	legacyFlatCmds := regexp.MustCompile(`'devcontainer-cli (up|start|down|ssh|config profile list|destroy)'`)
	// Removed or obsolete flags that must never be suggested to callers.
	removedFlags := []string{"--with", "--service ", "--skill ", "--no-build", "--preset", "--non-interactive", "ssh -f"}

	for _, dir := range []string{".", "../../service", "../../domain"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir %s: %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile %s: %v", path, err)
			}
			for i, line := range strings.Split(string(data), "\n") {
				if !strings.Contains(line, "fmt.Errorf") && !strings.Contains(line, "Report.Warn") && !strings.Contains(line, "console.Warn") {
					continue
				}
				if bareInvocation.MatchString(line) {
					t.Errorf("%s:%d suggests the interactive wizard, which hangs a non-interactive caller.\n"+
						"Name a command that does not prompt ('devcontainer-cli workspace up', 'devcontainer-cli agent create'):\n  %s",
						path, i+1, strings.TrimSpace(line))
				}
				if legacyFlatCmds.MatchString(line) {
					t.Errorf("%s:%d suggests a legacy flat command.\n"+
						"Name a canonical 6-domain command ('devcontainer-cli workspace up', 'devcontainer-cli workspace start', 'devcontainer-cli profile list'):\n  %s",
						path, i+1, strings.TrimSpace(line))
				}
				for _, rf := range removedFlags {
					if strings.Contains(line, rf) {
						t.Errorf("%s:%d suggests removed flag %q:\n  %s", path, i+1, rf, strings.TrimSpace(line))
					}
				}
			}
		}
	}
}
