package service

import (
	"archive/zip"
	"fmt"
	"io"
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

// sharedTargetsFunc defines `is_shared_target <home-relative-path>`, true when
// the path lies inside one of the shared entries (ENTRY_SHARED_MAP holds every
// entry as "<id>=<home-relative target>"). It is what tells a symlink pointing
// at another persisted config (~/.claude/skills/x -> ~/.agents/skills/x) apart
// from one pointing at something that only exists on the host.
const sharedTargetsFunc = `is_shared_target() {
  for _pair in $ENTRY_SHARED_MAP; do
    _t="${_pair#*=}"
    case "$1" in
      "$_t"|"$_t"/*) return 0 ;;
    esac
  done
  return 1
}
`

// volumeAliasesFunc defines `ensure_volume_aliases`, which mirrors the home
// layout at the volume root: every entry also gets a relative link under its
// HOME-relative name (.claude -> claude, .config/gh -> ../gh, …).
//
// The volume is flat (one dir per entry id) while the home it is symlinked
// into is not, and ~/.claude is itself a link into that flat root — so a
// RELATIVE cross-entry symlink such as ~/.claude/skills/x ->
// ../../.agents/skills/x lands on <volume>/.agents/skills/x, a name the flat
// layout does not have. These aliases give it one, which is what keeps such a
// link alive both when it is copied in from the host and when a tool inside a
// container writes it straight onto the volume. Keep in sync with the matching
// block in entrypoint.sh (which creates them on every container start).
const volumeAliasesFunc = `ensure_volume_aliases() {
  for _pair in $ENTRY_SHARED_MAP; do
    _id="${_pair%%=*}"
    _target="${_pair#*=}"
    _alias="/vol/$_target"
    if [ -e "$_alias" ] && [ ! -L "$_alias" ]; then continue; fi
    mkdir -p "$(dirname "$_alias")"
    _prefix=""
    _dir="$(dirname "$_target")"
    while [ "$_dir" != "." ] && [ "$_dir" != "/" ]; do
      _prefix="../$_prefix"
      _dir="$(dirname "$_dir")"
    done
    ln -sfn "$_prefix$_id" "$_alias"
    if [ -n "$ENTRY_OWNER" ]; then chown -h "$ENTRY_OWNER" "$_alias" 2>/dev/null || true; fi
  done
  return 0
}
`

// fixSymlinksFunc defines `fix_symlinks <copied-tree> <source-tree>`, the pass
// that makes symlinks inside a copied entry mean the same thing in the volume
// as they did on the host. It matters for entries like
// "claude"/"codex"/"agents": tools such as the skills.sh CLI (`npx skills add -g`)
// install global skills/agents into a canonical ~/.agents/skills store and
// symlink them into each agent's own config dir (e.g. ~/.claude/skills/<name> ->
// ~/.agents/skills/<name>).
//
// Every link is resolved against the SOURCE tree under /host, never against the
// copy — that is the whole point. Resolving in the destination (what this used
// to do) can only ever succeed for links that stay inside the entry, i.e. the
// ones that need no help at all, and silently left every cross-entry link
// dangling in the volume and therefore in every container.
//
// Three outcomes, in order:
//
//   - the target is inside another shared entry → keep it a LINK, so both sides
//     stay one store instead of drifting copies. A relative link is left
//     verbatim (the volume aliases make it resolve); an absolute host path is
//     repointed at ENTRY_DEV_HOME, the home the volume is symlinked into.
//   - the target is outside the shared entries but exists on the host → replace
//     the link with a real copy of that content, which is the only way it can
//     survive into a container.
//   - the target does not exist on the host either → left as-is. `cp -aL` used
//     to error "cannot stat" here and failed the whole sync for entries like
//     ~/.claude, where Claude Code leaves a dangling `debug/latest` pointer;
//     leaving it dangling is no worse than it already was on the host.
const fixSymlinksFunc = `fix_symlinks() {
  _root="$1"; _src="$2"
  # An unset host home must never degrade into the "/*" pattern below, which
  # would match (and rewrite) every absolute link target.
  _hh="${ENTRY_HOST_HOME:-/nonexistent-host-home}"
  _list="$(mktemp)"
  find "$_root" -type l > "$_list"
  while IFS= read -r _link; do
    [ -L "$_link" ] || continue
    _rel="${_link#"$_root"}"
    _rel="${_rel#/}"
    _srclink="$_src/$_rel"
    [ -L "$_srclink" ] || continue
    _target="$(readlink "$_srclink")"
    _absolute=1
    case "$_target" in
      "$_hh"/*) _target="/host/${_target#"$_hh"/}" ;;
      /*) ;;
      *) _target="$(dirname "$_srclink")/$_target"; _absolute=0 ;;
    esac
    _resolved="$(readlink -m "$_target")"
    _home_rel=""
    case "$_resolved" in
      /host/*) _home_rel="${_resolved#/host/}" ;;
    esac
    if [ -n "$_home_rel" ] && is_shared_target "$_home_rel"; then
      if [ "$_absolute" = 1 ]; then
        rm -rf "$_link"
        ln -sfn "$ENTRY_DEV_HOME/$_home_rel" "$_link"
      fi
      continue
    fi
    [ -e "$_resolved" ] || continue
    rm -rf "$_link"
    cp -a "$_resolved" "$_link"
  done < "$_list"
  rm -f "$_list"
  return 0
}
`

// syncEntryScript runs inside the helper. It reads the entry from the
// environment, mounts the volume at /vol and the host home (read-only) at
// /host, skips when the volume already has data (unless ENTRY_FORCE), otherwise
// replaces the entry with the host copy and re-owns it. It prints COPIED or
// SKIPPED so the caller can classify the result. See fixSymlinksFunc for why
// the copy is a plain `cp -a` followed by a symlink pass rather than a single
// `cp -aL`; the volume aliases are refreshed on every run, skipped entries
// included, since they cost one symlink each and nothing else creates them
// before a container first starts.
const syncEntryScript = `set -e
` + sharedTargetsFunc + volumeAliasesFunc + fixSymlinksFunc + `dst="/vol/$ENTRY_ID"
src="/host/$ENTRY_TARGET"
ensure_volume_aliases
if [ -z "$ENTRY_FORCE" ]; then
  if [ "$ENTRY_KIND" = dir ]; then
    if [ -d "$dst" ] && [ -n "$(ls -A "$dst" 2>/dev/null)" ]; then echo SKIPPED; exit 0; fi
  else
    if [ -s "$dst" ]; then echo SKIPPED; exit 0; fi
  fi
fi
if [ "$ENTRY_KIND" = dir ]; then
  rm -rf "$dst"; mkdir -p "$dst"; cp -a "$src/." "$dst/"
  fix_symlinks "$dst" "$src"
else
  rm -f "$dst"
  if [ -L "$src" ] && [ ! -e "$src" ]; then
    cp -a "$src" "$dst"
  else
    cp -aL "$src" "$dst"
  fi
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

	owner := hostOwnerString()

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

// sharedEntryMap renders every shared entry as "<id>=<home-relative target>",
// space-separated. The helper needs the WHOLE catalog, not just the entries
// being synced: a link inside one entry routinely points into another, and the
// volume aliases must exist for all of them regardless of what was requested.
func sharedEntryMap() string {
	pairs := make([]string, len(types.SharedConfigEntries))
	for i, e := range types.SharedConfigEntries {
		pairs[i] = e.ID + "=" + e.Target
	}
	return strings.Join(pairs, " ")
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
		// Symlink resolution inputs: the host home is what an absolute link
		// target is rewritten from, DevUserHome what it is rewritten to, and the
		// map says which targets are shared entries.
		"-e", "ENTRY_HOST_HOME=" + strings.TrimSuffix(filepath.ToSlash(hostHome), "/"),
		"-e", "ENTRY_DEV_HOME=" + types.DevUserHome,
		"-e", "ENTRY_SHARED_MAP=" + sharedEntryMap(),
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

// writeFileEntryScript writes $ENTRY_CONTENT into /vol/$ENTRY_ID and re-owns it.
// Unlike syncEntryScript it always overwrites: the caller (config alias) holds
// the single source of truth, so there is nothing in the volume to protect.
const writeFileEntryScript = `set -e
dst="/vol/$ENTRY_ID"
printf '%s' "$ENTRY_CONTENT" > "$dst"
[ -n "$ENTRY_OWNER" ] && chown "$ENTRY_OWNER" "$dst"
echo COPIED`

// SyncAliases writes the rendered alias script into the shared volume's
// alias.sh entry, so every container that mounts the volume picks it up as
// ~/.alias.sh on its next start (or immediately in already-running containers,
// since the file is symlinked). The content is authored by the CLI, not read
// from a host file, which is why it always overwrites. Ownership matches the
// host uid/gid, i.e. what devuser is remapped to on boot.
func (s SharedConfigService) SyncAliases(content string) error {
	if err := docker.EnsureDocker(); err != nil {
		return err
	}
	if err := EnsureSharedConfigVolume(s.Report); err != nil {
		return err
	}
	entry, ok := types.SharedConfigEntryByID(types.SharedConfigAliasID)
	if !ok {
		return fmt.Errorf("no shared-config entry for %q", types.SharedConfigAliasID)
	}
	status, _, stderr, err := docker.DockerCapture([]string{
		"run", "--rm",
		"-v", types.SharedConfigVolumeName + ":/vol",
		"-e", "ENTRY_ID=" + entry.ID,
		"-e", "ENTRY_CONTENT=" + content,
		"-e", "ENTRY_OWNER=" + hostOwnerString(),
		syncHelperImage, "sh", "-c", writeFileEntryScript,
	})
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("writing aliases into the volume failed (status %d): %s", status, strings.TrimSpace(stderr))
	}
	return nil
}

// hostOwnerString returns "uid:gid" for the current host user, used to
// re-own volume content copied in from (or extracted for) the host so it
// matches devuser's uid/gid remap on container boot.
func hostOwnerString() string {
	if uid := os.Getuid(); uid >= 0 {
		return fmt.Sprintf("%d:%d", uid, os.Getgid())
	}
	return ""
}

// stageOutScript copies each requested entry from the volume (/vol) into the
// host-mounted staging dir (/stage) for zipping, one docker run for every
// entry in $ENTRY_IDS (space-separated) rather than one run per entry.
// Entries absent from the volume are silently skipped; the caller classifies
// Included/Missing by checking what actually landed in the staging dir.
const stageOutScript = `set -e
for id in $ENTRY_IDS; do
  src="/vol/$id"
  [ -e "$src" ] || continue
  cp -a "$src" "/stage/$id"
done`

// stageInScript is the inverse of stageOutScript: copies each entry from the
// extracted-zip staging dir (/stage) into the volume (/vol). An entry that
// already has data in the volume is left alone (echoing SKIPPED) unless
// $ENTRY_FORCE is set, mirroring syncEntryScript's skip-unless-force rule;
// dir-vs-file is detected with -d/-s directly instead of a passed ENTRY_KIND,
// since one invocation now handles every requested entry.
const stageInScript = `set -e
for id in $ENTRY_IDS; do
  src="/stage/$id"
  [ -e "$src" ] || continue
  dst="/vol/$id"
  has_data=0
  if [ -d "$dst" ]; then
    [ -n "$(ls -A "$dst" 2>/dev/null)" ] && has_data=1
  elif [ -s "$dst" ]; then
    has_data=1
  fi
  if [ -z "$ENTRY_FORCE" ] && [ "$has_data" = 1 ]; then
    echo "SKIPPED $id"
    continue
  fi
  rm -rf "$dst"
  cp -a "$src" "$dst"
  [ -n "$ENTRY_OWNER" ] && chown -R "$ENTRY_OWNER" "$dst"
  echo "RESTORED $id"
done`

// BackupResult summarizes a BackupToZip run.
type BackupResult struct {
	Included []string // entries written into the zip
	Missing  []string // entries absent from the volume, not included
}

// entryIDs extracts the id of each entry, preserving order.
func entryIDs(entries []types.SharedConfigEntry) []string {
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	return ids
}

// sharedConfigVolumeExists reports whether the shared-config volume exists,
// without creating it (unlike EnsureSharedConfigVolume).
func sharedConfigVolumeExists() bool {
	status, _, _, err := docker.DockerCapture([]string{"volume", "inspect", types.SharedConfigVolumeName})
	return err == nil && status == 0
}

// BackupToZip writes a zip archive of the given entries' current content in
// the shared volume (types.SharedConfigVolumeName) to destZip via a throwaway
// helper container. The zip's top-level names match entry ids, mirroring the
// volume's root layout, so RestoreFromZip can restore it unchanged. Entries
// absent from the volume are skipped and reported in Missing.
func (s SharedConfigService) BackupToZip(entries []types.SharedConfigEntry, destZip string) (BackupResult, error) {
	res := BackupResult{}
	if err := docker.EnsureDocker(); err != nil {
		return res, err
	}
	if !sharedConfigVolumeExists() {
		return res, fmt.Errorf("shared-config volume %s not found: nothing to back up", types.SharedConfigVolumeName)
	}

	stageDir, err := os.MkdirTemp("", "dc-shared-config-backup-")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(stageDir)

	ids := entryIDs(entries)
	status, _, stderr, err := docker.DockerCapture([]string{
		"run", "--rm",
		"-v", types.SharedConfigVolumeName + ":/vol:ro",
		"-v", stageDir + ":/stage",
		"-e", "ENTRY_IDS=" + strings.Join(ids, " "),
		syncHelperImage, "sh", "-c", stageOutScript,
	})
	if err != nil {
		return res, err
	}
	if status != 0 {
		return res, fmt.Errorf("helper failed (status %d): %s", status, strings.TrimSpace(stderr))
	}

	for _, id := range ids {
		if _, statErr := os.Lstat(filepath.Join(stageDir, id)); statErr != nil {
			res.Missing = append(res.Missing, id)
			continue
		}
		res.Included = append(res.Included, id)
	}
	if len(res.Included) == 0 {
		return res, fmt.Errorf("no data found in the shared volume for the requested entries")
	}

	if err := writeZipFromDir(stageDir, destZip, res.Included); err != nil {
		return res, err
	}
	return res, nil
}

// RestoreResult summarizes a RestoreFromZip run.
type RestoreResult struct {
	Restored []string // entries copied from the zip into the volume
	Skipped  []string // entries the volume already had data for (use force)
	Missing  []string // requested entries not present in the zip
}

// RestoreFromZip loads a zip produced by BackupToZip back into the shared
// volume via a throwaway helper container. Entries already holding data in
// the volume are left untouched unless force is true. Requested entries not
// present in the zip are reported in Missing.
func (s SharedConfigService) RestoreFromZip(zipPath string, entries []types.SharedConfigEntry, force bool) (RestoreResult, error) {
	res := RestoreResult{}
	if err := docker.EnsureDocker(); err != nil {
		return res, err
	}
	if err := EnsureSharedConfigVolume(s.Report); err != nil {
		return res, err
	}

	stageDir, err := os.MkdirTemp("", "dc-shared-config-restore-")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(stageDir)

	if err := extractZipToDir(zipPath, stageDir); err != nil {
		return res, err
	}

	var ids []string
	for _, e := range entries {
		if _, statErr := os.Lstat(filepath.Join(stageDir, e.ID)); statErr != nil {
			res.Missing = append(res.Missing, e.ID)
			continue
		}
		ids = append(ids, e.ID)
	}
	if len(ids) == 0 {
		return res, fmt.Errorf("zip contains none of the requested entries")
	}

	forceVal := ""
	if force {
		forceVal = "1"
	}
	status, stdout, stderr, err := docker.DockerCapture([]string{
		"run", "--rm",
		"-v", types.SharedConfigVolumeName + ":/vol",
		"-v", stageDir + ":/stage:ro",
		"-e", "ENTRY_IDS=" + strings.Join(ids, " "),
		"-e", "ENTRY_FORCE=" + forceVal,
		"-e", "ENTRY_OWNER=" + hostOwnerString(),
		syncHelperImage, "sh", "-c", stageInScript,
	})
	if err != nil {
		return res, err
	}
	if status != 0 {
		return res, fmt.Errorf("helper failed (status %d): %s", status, strings.TrimSpace(stderr))
	}

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "RESTORED "):
			res.Restored = append(res.Restored, strings.TrimPrefix(line, "RESTORED "))
		case strings.HasPrefix(line, "SKIPPED "):
			res.Skipped = append(res.Skipped, strings.TrimPrefix(line, "SKIPPED "))
		}
	}
	return res, nil
}

// writeZipFromDir zips the given top-level entries (subpaths of srcDir) into
// destZip, preserving relative paths and file modes.
func writeZipFromDir(srcDir, destZip string, topLevel []string) error {
	f, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, name := range topLevel {
		root := filepath.Join(srcDir, name)
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(srcDir, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if info.IsDir() {
				_, err := zw.Create(rel + "/")
				return err
			}
			// Symlinks (e.g. ~/.claude/debug/latest, which tools create inside
			// the container directly on the volume, often left dangling) must be
			// archived as symlinks: store the link target as the entry content
			// with the symlink mode preserved. os.Open would follow the link and
			// fail on a dangling one, aborting the whole backup.
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				hdr, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}
				hdr.Name = rel
				hdr.Method = zip.Deflate
				w, err := zw.CreateHeader(hdr)
				if err != nil {
					return err
				}
				_, err = io.WriteString(w, target)
				return err
			}
			hdr, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			hdr.Name = rel
			hdr.Method = zip.Deflate
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(w, in)
			return err
		})
		if walkErr != nil {
			zw.Close()
			return walkErr
		}
	}
	return zw.Close()
}

// extractZipToDir extracts every entry of the zip at zipPath into destDir,
// rejecting any entry whose path would resolve outside destDir (zip-slip).
func extractZipToDir(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir)
	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || filepath.IsAbs(cleanName) {
			return fmt.Errorf("zip entry has unsafe path: %s", f.Name)
		}
		target := filepath.Join(cleanDest, cleanName)
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry escapes destination: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		// Recreate symlinks (written by writeZipFromDir with the link target as
		// content) instead of writing a regular file, so backups round-trip
		// faithfully. The link target is restored verbatim; only the entry name
		// is validated against zip-slip above.
		if f.Mode()&os.ModeSymlink != 0 {
			if err := extractZipSymlink(f, target); err != nil {
				return err
			}
			continue
		}
		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

// extractZipSymlink recreates a symlink zip entry at target, reading the link
// target from the entry's content (as written by writeZipFromDir).
func extractZipSymlink(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	linkTarget, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	return os.Symlink(string(linkTarget), target)
}

// extractZipFile writes a single zip entry's content to target.
func extractZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := f.Mode()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
