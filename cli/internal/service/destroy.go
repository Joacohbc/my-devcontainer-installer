package service

import (
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// DestroyService tears down a project: it brings the stack down with its
// volumes and deletes the generated artifacts and config. Confirmation is the
// caller's responsibility; this runs unconditionally.
type DestroyService struct {
	Report Reporter
}

// DestroyTarget names everything a destroy removes for one project.
type DestroyTarget struct {
	Workspace   string
	ComposeFile string
	ProjectDir  string
	ConfigPath  string
	ProjectKey  string
}

// Run brings the stack down (with volumes) when a compose file exists, deletes
// the project directory and config file, and drops the catalog entry.
func (s DestroyService) Run(t DestroyTarget) error {
	if fileExists(t.ComposeFile) {
		s.Report.Warn("Bringing down '%s' (with volumes)...", t.Workspace)
		if err := docker.DockerComposeOrThrow(t.ComposeFile, []string{"down", "-v"}, nil); err != nil {
			return err
		}
	} else {
		s.Report.Info("No compose file at %s; skipping 'docker compose down'.", t.ComposeFile)
	}

	s.removeIfExists(t.ProjectDir)
	s.removeIfExists(t.ConfigPath)
	domain.RemoveEntry(t.ProjectKey)

	s.Report.Success("Destroyed.")
	return nil
}

func (s DestroyService) removeIfExists(path string) {
	if !fileExists(path) {
		return
	}
	if err := os.RemoveAll(path); err == nil {
		s.Report.Info("Removed %s", path)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
