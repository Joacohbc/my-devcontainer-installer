package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// currentDir returns the project working directory. When invoked from inside a
// generated ".dc_<workspace>/…" build tree it climbs back out to the project
// root, so project-scoped commands (compose, status, logs, up, …) resolve the
// real project instead of deriving a bogus nested workspace from the build
// dir's own name (e.g. running from ".dc_mcp/build" must not target
// ".dc_mcp/build/.dc_build/build"). The rare os.Getwd failure is wrapped with
// context so callers can surface it instead of silently proceeding with an
// empty path.
func currentDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine current directory: %w", err)
	}
	return projectRoot(dir), nil
}

// projectRoot strips a generated ".dc_<workspace>" segment (and anything below
// it) from dir, returning the project directory that contains it. A path with
// no such segment is returned unchanged.
func projectRoot(dir string) string {
	sep := string(filepath.Separator)
	parts := strings.Split(dir, sep)
	for i, p := range parts {
		if strings.HasPrefix(p, ".dc_") && len(p) > len(".dc_") {
			root := strings.Join(parts[:i], sep)
			if root == "" {
				// The ".dc_<ws>" segment was the first component of an absolute
				// path; the project root is the filesystem root.
				return sep
			}
			return root
		}
	}
	return dir
}

// warnIfBuildStale tells the user when this CLI version would generate a
// different image than the project's build files describe (typically after a
// CLI upgrade), and how to regenerate without prompting.
func warnIfBuildStale(cwd, workspace string) {
	config, err := domain.LoadConfig(cwd)
	if err != nil || config == nil {
		return
	}
	reasons := service.StaleBuildReasons(config, project.ProjectPaths(cwd, workspace))
	if len(reasons) == 0 {
		return
	}
	console.Warn("This CLI version generates a different image (%s changed). Regenerate and rebuild with '%s'.", strings.Join(reasons, ", "), service.StaleRegenerateHint)
}
