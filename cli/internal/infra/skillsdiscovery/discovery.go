package skillsdiscovery

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNodeRequirement is returned when node or npx is missing or older than 22.20.
var ErrNodeRequirement = fmt.Errorf("requires Node >= 22.20 on the host; or add a single skill with 'config skill add <id> --ref <source> --skill <name>'")

// DiscoveredRaw holds raw discovery output from npx skills add.
type DiscoveredRaw struct {
	RawJSON  []byte
	SkillMDs map[string][]byte
}

// Runner discovers skills using the host npx / Skills CLI.
type Runner struct{}

// NewRunner returns a default discovery runner.
func NewRunner() *Runner {
	return &Runner{}
}

// DiscoverSkills executes the Skills CLI in a temporary directory and extracts the JSON output
// along with any SKILL.md contents keyed by their skill directory path.
func (r *Runner) DiscoverSkills(source string) (*DiscoveredRaw, error) {
	if strings.HasPrefix(source, "-") {
		return nil, fmt.Errorf("invalid source %q: source cannot start with '-'", source)
	}

	if err := checkNodeRequirement(); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "devcontainer-skills-discovery-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory for skills discovery: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("npx", "-y", "skills", "add", source, "--skill", "*", "-a", "claude-code", "--copy", "-y", "--json")
	cmd.Dir = tmpDir
	// Telemetry stays enabled on purpose (DISABLE_TELEMETRY also disables the security audit).

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("skills discovery failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("skills discovery failed: %w", err)
	}

	skillMDs := make(map[string][]byte)
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "SKILL.md" {
			if data, rerr := os.ReadFile(path); rerr == nil {
				skillDir := filepath.Dir(path)
				skillMDs[skillDir] = data
			}
		}
		return nil
	})

	return &DiscoveredRaw{
		RawJSON:  out,
		SkillMDs: skillMDs,
	}, nil
}

func checkNodeRequirement() error {
	if _, err := exec.LookPath("node"); err != nil {
		return ErrNodeRequirement
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return ErrNodeRequirement
	}

	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return ErrNodeRequirement
	}

	if !isSupportedNodeVersion(string(out)) {
		return ErrNodeRequirement
	}
	return nil
}

func isSupportedNodeVersion(vStr string) bool {
	vStr = strings.TrimSpace(vStr)
	vStr = strings.TrimPrefix(vStr, "v")
	parts := strings.Split(vStr, ".")
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	if major > 22 {
		return true
	}
	if major == 22 && minor >= 20 {
		return true
	}
	return false
}
