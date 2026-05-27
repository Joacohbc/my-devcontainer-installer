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
