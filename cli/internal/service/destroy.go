package service

import (
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
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
	s.removeSSHConfigBlock(t.Workspace)

	s.Report.Success("Destroyed.")
	return nil
}

// removeSSHConfigBlock drops the managed Host block setup-ssh wrote for this
// workspace from ~/.ssh/config. Failures here are non-fatal: a missing or locked
// ssh config must not abort the destroy.
func (s DestroyService) removeSSHConfigBlock(workspace string) {
	if workspace == "" {
		return
	}
	ssh := SshService{Report: s.Report}
	removed, backup, err := ssh.RemoveManagedBlock(workspace)
	if err != nil {
		s.Report.Warn("Could not update ~/.ssh/config: %v", err)
		return
	}
	if removed {
		s.Report.Info("Removed SSH host block for '%s' from ~/.ssh/config (backup: %s)", workspace, backup)
	}
}

// RunContainer tears down a single loose container by name (the --container
// counterpart to Run, for containers set up outside a workspace project via
// 'setup-ssh --container' / 'ssh --container'): stop it, remove it, and prune its
// managed SSH host block. Unlike Run there is no compose stack, project
// directory, or config file to remove.
func (s DestroyService) RunContainer(container string) error {
	if err := (LifecycleService{Report: s.Report}).RemoveContainer(container); err != nil {
		return err
	}

	ssh := SshService{Report: s.Report}
	removed, backup, err := ssh.RemoveManagedBlockByRef(sshdefaults.KindContainer, container)
	if err != nil {
		s.Report.Warn("Could not update ~/.ssh/config: %v", err)
	} else if removed {
		s.Report.Info("Removed SSH host block for '%s' from ~/.ssh/config (backup: %s)", container, backup)
	}

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
