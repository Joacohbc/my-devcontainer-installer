package service

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

// StaleRegenerateHint is the non-prompting command that regenerates a project's
// build files from its saved configuration and rebuilds the image.
const StaleRegenerateHint = "devcontainer-cli workspace init --no-interactive --build"

// StaleBuildReasons compares a custom project's generated build files with
// what this CLI version would generate from the same configuration. A CLI
// upgrade can change a module's Dockerfile or an embedded helper script; until
// the files are regenerated the project keeps building the old image. It
// returns nothing for prebuilt modes or when the files cannot be read.
func StaleBuildReasons(config *types.DevcontainerConfig, paths project.Paths) []string {
	if config == nil || config.Mode != types.BuildModeCustom {
		return nil
	}
	onDisk, err := os.ReadFile(paths.DockerfilePath)
	if err != nil {
		return nil
	}
	var reasons []string
	if fresh, err := domain.GenerateDockerfile(config); err == nil && !domain.SameDockerfile(string(onDisk), fresh) {
		reasons = append(reasons, "the Dockerfile")
	}

	copyFiles, _ := domain.CollectRequiredCopyFiles(config)
	postFiles, _ := domain.CollectRequiredPostScriptFiles(config)
	for _, f := range slices.Concat(copyFiles, postFiles) {
		embedded, err := assets.ReadAsset(f)
		if err != nil {
			continue
		}
		built, err := os.ReadFile(filepath.Join(paths.BuildDir, f))
		if err != nil || string(built) != string(embedded) {
			reasons = append(reasons, f)
		}
	}
	if table, err := os.ReadFile(filepath.Join(paths.BuildDir, types.SharedConfigTableFileName)); err == nil && string(table) != types.RenderSharedConfigTable() {
		reasons = append(reasons, types.SharedConfigTableFileName)
	}
	return reasons
}
