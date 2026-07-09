package domain

import (
	"path/filepath"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func ResolveWorkspace(cwd string, config *types.DevcontainerConfig) string {
	if config != nil && config.Workspace != "" {
		return config.Workspace
	}
	return SanitizeDockerName(filepath.Base(cwd), "devcontainer")
}

// UniqueWorkspaceName returns base unless another project directory in the image
// registry already claims that workspace name, in which case it disambiguates by
// appending a short hash of this project's path (e.g. "api" -> "api-3f9a"). Because
// the workspace name flows into the compose project and all container/network/volume
// names — which are global to the Docker daemon — two directories sharing a base name
// would otherwise collide. Same-directory entries never collide, so re-running
// generate on a project keeps its name stable.
func UniqueWorkspaceName(cwd, base string) string {
	for _, e := range LoadRegistry() {
		if e.Workspace == base && e.ProjectDir != cwd {
			return disambiguateWorkspace(base, cwd)
		}
	}
	return base
}

// disambiguateWorkspace appends a deterministic 6-hex-char suffix derived from the
// project path, keeping the result within Docker's 63-character name limit.
func disambiguateWorkspace(base, cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	suffix := "-" + sha256Hex(abs)[:6]
	if len(base)+len(suffix) > 63 {
		base = base[:63-len(suffix)]
	}
	return base + suffix
}
