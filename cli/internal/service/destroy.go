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
// workspace from the managed SSH config, and with it the container host keys that block
// had pinned. Failures here are non-fatal: a missing or locked ssh config must
// not abort the destroy.
func (s DestroyService) removeSSHConfigBlock(workspace string) {
	s.removeManagedSSH(sshdefaults.KindWorkspace, workspace)
}

// removeManagedSSH prunes the managed SSH state of one target: its Host block
// and the host keys pinned for the address that block dialed. Doing both here is
// what stops a destroyed target from leaving an orphaned key behind until the
// next 'clean ssh'. The address must be read before the block goes.
func (s DestroyService) removeManagedSSH(kind sshdefaults.Kind, ref string) {
	if ref == "" {
		return
	}
	ssh := SshService{Report: s.Report}
	// Managed blocks live in the CLI's own SSH config now. Lift any that an
	// older version left inside the user's ~/.ssh/config first, or destroy would
	// look in the wrong file and leave the block (and its pinned key) behind.
	if _, err := ssh.MigrateManagedBlocks(); err != nil {
		s.Report.Warn("Could not migrate managed SSH blocks: %v", err)
	}
	pinnedHost, err := ssh.ManagedHostName(kind, ref)
	if err != nil {
		s.Report.Warn("Could not read the managed SSH config: %v", err)
		return
	}
	removed, backup, err := ssh.RemoveManagedBlockByRef(kind, ref)
	if err != nil {
		s.Report.Warn("Could not update the managed SSH config: %v", err)
		return
	}
	if !removed {
		return
	}
	s.Report.Info("Removed SSH host block for '%s' from the managed SSH config (backup: %s)", ref, backup)
	s.forgetPinnedHostKey(ssh, pinnedHost)
}

// forgetPinnedHostKey drops host's keys from the CLI-managed known_hosts unless
// another Host block still dials that address — two aliases may point at the
// same container.
func (s DestroyService) forgetPinnedHostKey(ssh SshService, host string) {
	if host == "" {
		return
	}
	isStillReferenced, err := ssh.HostIsReferenced(host)
	if err != nil || isStillReferenced {
		return
	}
	dropped, err := ssh.ForgetHostKeys([]string{host})
	if err != nil || dropped == 0 {
		return
	}
	s.Report.Info("Forgot the pinned host key of %s", host)
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

	s.removeManagedSSH(sshdefaults.KindContainer, container)

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
