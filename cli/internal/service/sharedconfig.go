package service

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// EnsureSharedConfigVolume creates the single daemon-level shared tool-config
// volume (devcontainer-shared-config) if it does not already exist, labelling it
// managed + shared-config. It is idempotent and never deletes or relabels an
// existing volume. The compose document declares this volume `external: true`,
// so it must exist before `docker compose up`; quick-run also calls this so the
// label is applied (a bare `docker run -v` would otherwise create it unlabeled).
func EnsureSharedConfigVolume(report Reporter) error {
	if status, _, _, err := docker.DockerCapture([]string{"volume", "inspect", types.SharedConfigVolumeName}); err == nil && status == 0 {
		return nil
	}
	status, _, stderr, err := docker.DockerCapture([]string{
		"volume", "create",
		"--label", types.LabelManaged + "=true",
		"--label", types.LabelSharedConfig + "=true",
		types.SharedConfigVolumeName,
	})
	if err != nil {
		return err
	}
	if status != 0 {
		report.Warn("Could not create shared-config volume %s: %s", types.SharedConfigVolumeName, stderr)
	}
	return nil
}
