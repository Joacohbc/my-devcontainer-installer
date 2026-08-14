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
// Every hint must name a command that does not prompt: 'up', 'start',
// 'agent create'.
//
// This walks the source rather than the built help, because the strings live in
// fmt.Errorf calls spread across two layers.
func TestErrorMessagesDoNotPointAtTheWizard(t *testing.T) {
	// The bare binary name inside quotes, not followed by a subcommand.
	bareInvocation := regexp.MustCompile(`'devcontainer-cli'`)

	for _, dir := range []string{".", "../../service"} {
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
				if !strings.Contains(line, "fmt.Errorf") && !strings.Contains(line, "Report.Warn") {
					continue
				}
				if bareInvocation.MatchString(line) {
					t.Errorf("%s:%d suggests the interactive wizard, which hangs a non-interactive caller.\n"+
						"Name a command that does not prompt ('devcontainer-cli up', 'devcontainer-cli agent create'):\n  %s",
						path, i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}
