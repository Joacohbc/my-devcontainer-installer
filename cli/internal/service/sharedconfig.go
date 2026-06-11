package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// SharedConfigService seeds and maintains the shared tool-config volume from
// the host. Copies go through `docker cp` on a container that mounts the
// volume: a running managed one when available, otherwise a short-lived helper.
type SharedConfigService struct {
	Report Reporter
}

// SyncResult summarizes a SyncFromHost run.
type SyncResult struct {
	Copied  []string // entries copied into the volume
	Skipped []string // entries that already had data in the volume (use force)
	Missing []string // entries with no config present on the host
}

// syncHelperName is the throwaway container used when no managed container
// mounting the shared volume is running.
const syncHelperName = "devcontainer-sync-config"

// syncHelperImage is the helper's image: the same Ubuntu base every local
// image builds FROM, so it is usually already present in the daemon.
const syncHelperImage = "ubuntu:24.04"

// SyncFromHost copies each entry's host config (hostHome + entry.Target) into
// the shared volume. Entries that already hold data in the volume are skipped
// unless force is true (force replaces them); entries absent on the host are
// reported missing.
func (s SharedConfigService) SyncFromHost(entries []types.SharedConfigEntry, hostHome string, force bool) (SyncResult, error) {
	res := SyncResult{}
	if err := docker.EnsureDocker(); err != nil {
		return res, err
	}
	if err := EnsureSharedConfigVolume(s.Report); err != nil {
		return res, err
	}

	carrier, isManaged, cleanup, err := s.carrierContainer()
	if err != nil {
		return res, err
	}
	defer cleanup()

	for _, e := range entries {
		hostPath := filepath.Join(hostHome, filepath.FromSlash(e.Target))
		if _, statErr := os.Stat(hostPath); statErr != nil {
			s.Report.Warn("  - %s: not found on host (%s), skipped", e.ID, hostPath)
			res.Missing = append(res.Missing, e.ID)
			continue
		}

		volPath := types.SharedConfigMountPath + "/" + e.ID
		if !force && volumeEntryHasData(carrier, e, volPath) {
			s.Report.Warn("  - %s: volume already has data, skipped (use --force to replace)", e.ID)
			res.Skipped = append(res.Skipped, e.ID)
			continue
		}

		if err := s.copyEntry(carrier, isManaged, e, hostPath, volPath); err != nil {
			return res, fmt.Errorf("sync %s: %w", e.ID, err)
		}
		s.Report.Success("  ✓ %s: %s → volume", e.ID, hostPath)
		res.Copied = append(res.Copied, e.ID)
	}
	return res, nil
}

// carrierContainer returns a container mounting the shared volume to `docker
// cp` through: the first running managed one, or a temporary helper (cleaned
// up by the returned func).
func (s SharedConfigService) carrierContainer() (name string, managed bool, cleanup func(), err error) {
	nop := func() {}
	status, stdout, _, cerr := docker.DockerCapture([]string{
		"ps", "--filter", managedFilter,
		"--filter", "volume=" + types.SharedConfigVolumeName,
		"--format", "{{.Names}}",
	})
	if cerr == nil && status == 0 {
		if lines := strings.Fields(strings.TrimSpace(stdout)); len(lines) > 0 {
			return lines[0], true, nop, nil
		}
	}

	s.Report.Warn("No running container mounts %s — starting a temporary helper (%s)...", types.SharedConfigVolumeName, syncHelperImage)
	runStatus, runErr := docker.DockerInherit([]string{
		"run", "-d", "--rm", "--name", syncHelperName,
		"-v", types.SharedConfigMount(),
		syncHelperImage, "sleep", "infinity",
	})
	if runErr != nil {
		return "", false, nop, runErr
	}
	if runStatus != 0 {
		return "", false, nop, fmt.Errorf("could not start helper container %s", syncHelperName)
	}
	cleanup = func() {
		_, _, _, _ = docker.DockerCapture([]string{"rm", "-f", syncHelperName})
	}
	return syncHelperName, false, cleanup, nil
}

// volumeEntryHasData reports whether the entry already holds content inside
// the shared volume (non-empty dir, or non-empty file).
func volumeEntryHasData(carrier string, e types.SharedConfigEntry, volPath string) bool {
	var script string
	if e.Kind == types.SharedConfigDir {
		script = fmt.Sprintf(`[ -d %q ] && [ -n "$(ls -A %q 2>/dev/null)" ] && echo nonempty`, volPath, volPath)
	} else {
		script = fmt.Sprintf(`[ -s %q ] && echo nonempty`, volPath)
	}
	_, stdout, _, err := docker.DockerCapture([]string{"exec", carrier, "sh", "-c", script})
	return err == nil && strings.Contains(stdout, "nonempty")
}

// copyEntry replaces the entry inside the volume with the host content and
// re-owns it: to devuser when the carrier is a managed container, otherwise to
// the host uid/gid (which is what devuser gets remapped to at boot).
func (s SharedConfigService) copyEntry(carrier string, isManaged bool, e types.SharedConfigEntry, hostPath, volPath string) error {
	var prep string
	src := hostPath
	if e.Kind == types.SharedConfigDir {
		prep = fmt.Sprintf("rm -rf %q && mkdir -p %q", volPath, volPath)
		src = hostPath + string(os.PathSeparator) + "." // copy contents, not the dir itself
	} else {
		prep = fmt.Sprintf("rm -f %q", volPath)
	}
	if status, err := docker.DockerInherit([]string{"exec", carrier, "sh", "-c", prep}); err != nil || status != 0 {
		return fmt.Errorf("prepare %s failed (status %d): %v", volPath, status, err)
	}

	if status, err := docker.DockerInherit([]string{"cp", src, carrier + ":" + volPath}); err != nil || status != 0 {
		return fmt.Errorf("docker cp failed (status %d): %v", status, err)
	}

	owner := ""
	if isManaged {
		owner = "devuser:devuser"
	} else if uid := os.Getuid(); uid >= 0 {
		owner = fmt.Sprintf("%d:%d", uid, os.Getgid())
	}
	if owner != "" {
		if status, err := docker.DockerInherit([]string{"exec", carrier, "chown", "-R", owner, volPath}); err != nil || status != 0 {
			return fmt.Errorf("chown %s failed (status %d): %v", volPath, status, err)
		}
	}
	return nil
}
