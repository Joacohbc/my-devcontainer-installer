package service

import (
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

// UpdateService refreshes container images for one project or every recorded
// project. Docker access is injected so the logic is testable without Docker.
type UpdateService struct {
	Report  Reporter
	Pull    func(image string) error
	Compose func(projectDir, composeFile string, args []string) error
}

// UpdateOne updates the image for a single project, returning the resolved
// image and whether the update succeeded. Remote configs are pulled; local
// configs are rebuilt (with --pull unless a pure rebuild was requested).
func (s UpdateService) UpdateOne(projectDir string, config *types.DevcontainerConfig, pull, rebuild bool) (string, bool) {
	s.Report.Info("Updating '%s' (mode=%s) at %s", config.Workspace, config.Mode, projectDir)

	if config.Mode == types.BuildModeRemote {
		if config.Remote == nil {
			s.Report.Warn("Remote config missing — skipping.")
			return config.Image, false
		}
		image := domain.ResolveRemoteImage(config.Remote.Variant, domain.ResolveRegistry("", config.Remote.Registry))
		if err := s.Pull(image); err != nil {
			return image, false
		}
		s.Report.Success("Pulled " + image)
		return image, true
	}

	composeFile := project.ProjectPaths(projectDir, config.Workspace).ComposeFile
	if _, err := os.Stat(composeFile); err != nil {
		s.Report.Warn("No compose file at %s — skipping.", composeFile)
		return config.Image, false
	}

	args := []string{"build"}
	if !rebuild || pull {
		args = append(args, "--pull")
	}
	if err := s.Compose(projectDir, composeFile, args); err != nil {
		return config.Image, false
	}
	s.Report.Success("Rebuilt " + config.Image)
	return config.Image, true
}

// UpdateAll updates every recorded project, recording successes and pruning
// catalog entries whose project directory has disappeared. It returns the
// counts of updated, skipped and failed projects.
func (s UpdateService) UpdateAll(pull, rebuild bool) (updated, skipped, failed int) {
	for _, e := range domain.ListEntries() {
		if _, err := os.Stat(e.ProjectDir); err != nil {
			s.Report.Warn("Project directory missing: %s — removing from catalog.", e.ProjectDir)
			domain.RemoveEntry(e.ProjectDir)
			skipped++
			continue
		}
		config, _ := domain.LoadConfig(e.ProjectDir)
		if config == nil {
			s.Report.Warn("No devcontainer.config.json at %s — skipping.", e.ProjectDir)
			skipped++
			continue
		}
		image, ok := s.UpdateOne(e.ProjectDir, config, pull, rebuild)
		if !ok {
			failed++
			continue
		}
		domain.RecordProject(e.ProjectDir, config, image)
		updated++
	}
	return updated, skipped, failed
}
