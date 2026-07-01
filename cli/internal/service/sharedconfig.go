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

// SharedConfigService seeds the shared tool-config volume from the host.
type SharedConfigService struct {
	Report Reporter
}

// SyncResult summarizes a SyncFromHost run.
type SyncResult struct {
	Copied  []string // entries copied into the volume
	Skipped []string // entries that already had data in the volume (use force)
	Missing []string // entries with no config present on the host
}

// syncHelperImage is the throwaway image used to copy host configs into the
// volume: the same Ubuntu base every local image builds FROM, so it is usually
// already present in the daemon.
const syncHelperImage = "ubuntu:24.04"

// syncEntryScript runs inside the helper. It reads the entry from the
// environment, mounts the volume at /vol and the host home (read-only) at
// /host, skips when the volume already has data (unless ENTRY_FORCE), otherwise
// replaces the entry with the host copy and re-owns it. It prints COPIED or
// SKIPPED so the caller can classify the result.
//
// cp -aL (not plain -a) dereferences symlinks instead of copying them as-is.
// This matters for entries like "claude"/"codex"/"agents": tools such as the
// skills.sh CLI (`npx skills add -g`) install global skills/agents into a
// canonical ~/.agents/skills store and symlink them into each agent's own
// config dir (e.g. ~/.claude/skills/<name> -> ~/.agents/skills/<name>). A
// plain `cp -a` would copy that symlink verbatim, pointing at a host path
// that does not exist inside the volume/container, leaving a dangling link.
// Dereferencing copies the real skill/agent content instead, so it persists
// regardless of the host's symlink layout.
const syncEntryScript = `set -e
dst="/vol/$ENTRY_ID"
src="/host/$ENTRY_TARGET"
if [ -z "$ENTRY_FORCE" ]; then
  if [ "$ENTRY_KIND" = dir ]; then
    if [ -d "$dst" ] && [ -n "$(ls -A "$dst" 2>/dev/null)" ]; then echo SKIPPED; exit 0; fi
  else
    if [ -s "$dst" ]; then echo SKIPPED; exit 0; fi
  fi
fi
if [ "$ENTRY_KIND" = dir ]; then
  rm -rf "$dst"; mkdir -p "$dst"; cp -aL "$src/." "$dst/"
else
  rm -f "$dst"; cp -aL "$src" "$dst"
fi
[ -n "$ENTRY_OWNER" ] && chown -R "$ENTRY_OWNER" "$dst"
echo COPIED`

// SyncFromHost copies each entry's host config (hostHome + entry.Target) into
// the shared volume via a throwaway helper container that mounts the volume and
// the host home. Entries that already hold data in the volume are skipped unless
// force is true; entries absent on the host are reported missing. Copied content
// is owned by the host uid/gid — exactly what devuser is remapped to on boot.
func (s SharedConfigService) SyncFromHost(entries []types.SharedConfigEntry, hostHome string, force bool) (SyncResult, error) {
	res := SyncResult{}
	if err := docker.EnsureDocker(); err != nil {
		return res, err
	}
	if err := EnsureSharedConfigVolume(s.Report); err != nil {
		return res, err
	}

	owner := ""
	if uid := os.Getuid(); uid >= 0 {
		owner = fmt.Sprintf("%d:%d", uid, os.Getgid())
	}

	for _, e := range entries {
		hostPath := filepath.Join(hostHome, filepath.FromSlash(e.Target))
		if _, statErr := os.Stat(hostPath); statErr != nil {
			s.Report.Warn("  - %s: not found on host (%s), skipped", e.ID, hostPath)
			res.Missing = append(res.Missing, e.ID)
			continue
		}

		copied, err := s.syncEntry(e, hostHome, owner, force)
		if err != nil {
			return res, fmt.Errorf("sync %s: %w", e.ID, err)
		}
		if copied {
			s.Report.Success("  ✓ %s: %s → volume", e.ID, hostPath)
			res.Copied = append(res.Copied, e.ID)
		} else {
			s.Report.Warn("  - %s: volume already has data, skipped (use --force to replace)", e.ID)
			res.Skipped = append(res.Skipped, e.ID)
		}
	}
	return res, nil
}

// syncEntry runs the helper for one entry and reports whether it copied (vs
// skipped because the volume already had data).
func (s SharedConfigService) syncEntry(e types.SharedConfigEntry, hostHome, owner string, force bool) (bool, error) {
	forceVal := ""
	if force {
		forceVal = "1"
	}
	status, stdout, stderr, err := docker.DockerCapture([]string{
		"run", "--rm",
		"-v", types.SharedConfigVolumeName + ":/vol",
		"-v", hostHome + ":/host:ro",
		"-e", "ENTRY_ID=" + e.ID,
		"-e", "ENTRY_TARGET=" + e.Target,
		"-e", "ENTRY_KIND=" + string(e.Kind),
		"-e", "ENTRY_FORCE=" + forceVal,
		"-e", "ENTRY_OWNER=" + owner,
		syncHelperImage, "sh", "-c", syncEntryScript,
	})
	if err != nil {
		return false, err
	}
	if status != 0 {
		return false, fmt.Errorf("helper failed (status %d): %s", status, strings.TrimSpace(stderr))
	}
	return strings.Contains(stdout, "COPIED"), nil
}
