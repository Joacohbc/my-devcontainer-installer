#!/bin/sh
# The shared-config module: ONE implementation of the volume's layout, shared by
# both hosts that build it.
#
# Two very different callers create the same layout, and before this file
# existed they each carried their own copy of it — the entrypoint in shell, the
# sync helper in shell embedded in Go string constants. The "one directory hop
# per level" alias computation was written out twice, byte for byte, and the
# catalogue was encoded twice in two different grammars. Neither copy existed
# because anything varied between the callers: they want exactly the same
# layout. They existed because it was written twice.
#
#   caller                        runs in                       needs
#   ────────────────────────────  ───────────────────────────   ─────────────────
#   entrypoint.sh                 every managed container       all of it
#   `config shared sync` helper   a throwaway ubuntu container  the aliases only
#
# INTERFACE
#
#   shared_config_apply <volume-root> <home>
#       Materializes every catalogue entry inside <volume-root>, mirrors the
#       home layout at its root, and links each entry into <home>. Idempotent.
#       NEVER destroys a real (non-symlink) thing sitting at a link path — it
#       keeps it and says so.
#   shared_config_volume_aliases <volume-root>
#       Only the root aliases. The host-side sync needs them before any
#       container has ever started, so it calls this half on its own.
#   shared_config_is_target <home-relative-path>
#       True when the path lies inside one of the entries. It is what tells a
#       symlink pointing at another persisted config apart from one pointing at
#       something that only exists on the host.
#   link_or_keep <link-path> <target> [message-when-kept]
#   own <path>... / own_link <path>...
#       The primitives the verbs above are built from. They are part of the
#       interface because the entrypoint links two things that are NOT shared
#       config (the Antigravity skills bridge, the image's baked global skill)
#       and must not open-code these rules a fifth time.
#
#   SC_ENTRIES  the catalogue: one "<id> <kind> <target>" per line, where <kind>
#               is dir|file and <target> is relative to the home. Rendered by
#               types.RenderSharedConfigTable() — Go is the only place the
#               catalogue is declared, and both callers are handed the result.
#   SC_OWNER    "uid:gid" applied to everything written, or empty to skip
#               chowning entirely (the sync helper runs as root against a volume
#               that must end up owned by the host user; a test runs as neither).
#
# POSIX sh, not bash: the sync helper runs `sh -c`, which on Ubuntu is dash.
# Hence `_`-prefixed globals in place of `local`. The entrypoint is bash and
# sources this file, so anything added here has to stay valid in both.

# Every input is read as "${VAR:-}": a library cannot know whether its caller
# runs under `set -u` (the entrypoint does not, a test does), and an unbound
# variable there would abort a container start rather than skip a chown.
#
# own / own_link apply SC_OWNER, and are no-ops without one. Best-effort by
# design: a chown that fails (an unwritable volume, a test running unprivileged)
# must not abort a container start.
own() {
  [ -n "${SC_OWNER:-}" ] || return 0
  chown "${SC_OWNER}" "$@" 2>/dev/null || true
}

own_link() {
  [ -n "${SC_OWNER:-}" ] || return 0
  chown -h "${SC_OWNER}" "$@" 2>/dev/null || true
}

# link_or_keep <link-path> <target> [message-when-kept]
#
# Keeps whatever real (non-symlink) thing already sits at <link-path>, printing
# <message-when-kept> on stderr when one is given, and returns 1 so the caller
# can skip the rest of its work. Otherwise creates the parent directory, points
# the link at <target> with `ln -sfn` (which both creates a missing link and
# repoints a stale one), and applies SC_OWNER to the link and its parent.
#
# <target> is used VERBATIM, never resolved: the volume aliases below pass a
# relative one (.config/gh -> ../gh) and depend on it surviving as written.
link_or_keep() {
  _link_path="$1"
  _link_target="$2"
  _kept_message="${3:-}"
  if [ -e "$_link_path" ] && [ ! -L "$_link_path" ]; then
    [ -n "$_kept_message" ] && echo "$_kept_message" >&2
    return 1
  fi
  _link_parent="$(dirname "$_link_path")"
  mkdir -p "$_link_parent"
  own "$_link_parent"
  ln -sfn "$_link_target" "$_link_path"
  own_link "$_link_path"
  return 0
}

# volume_alias_target <home-relative-target> <entry-id>
#
# The RELATIVE target of an entry's alias at the volume root: one "../" per
# directory level in <target>, then the flat entry id.
#
#   .claude     -> claude
#   .config/gh  -> ../gh
#
# An alias with the wrong number of hops does not fail, it silently dangles —
# which is why this is a named function with its own test rather than an
# expression inlined at the two places that need it.
volume_alias_target() {
  _remaining_dir="$(dirname "$1")"
  _parent_hops=""
  while [ "$_remaining_dir" != "." ] && [ "$_remaining_dir" != "/" ]; do
    _parent_hops="../$_parent_hops"
    _remaining_dir="$(dirname "$_remaining_dir")"
  done
  printf '%s%s' "$_parent_hops" "$2"
}

# shared_config_volume_aliases <volume-root>
#
# Mirrors the home layout at the volume root: every entry also gets a link under
# its HOME-relative name (.claude -> claude, .config/gh -> ../gh, ...).
#
# The volume is FLAT (one directory per entry id) while the home it is symlinked
# into is not, and ~/.claude is itself a link into that flat root — so a
# RELATIVE cross-entry symlink such as ~/.claude/skills/x -> ../../.agents/skills/x
# (what `npx skills add -g` writes) lands on <volume>/.agents/skills/x, a name
# the flat layout does not have, and dangles. These aliases give it one.
#
# Both callers create them, and that is deliberate rather than redundant: the
# entrypoint repairs an existing volume on every container start without
# needing a re-sync, and the sync helper makes a volume consistent before any
# container has ever mounted it.
shared_config_volume_aliases() {
  _vol_root="$1"
  echo "${SC_ENTRIES:-}" | while read -r _entry_id _entry_kind _entry_target; do
    [ -n "$_entry_id" ] || continue
    # A kept alias is a normal outcome, not a failure. Without this the loop's
    # status would be the LAST link_or_keep's, and a caller running under
    # `set -e` (the sync helper does) would abort on it.
    link_or_keep "$_vol_root/$_entry_target" \
      "$(volume_alias_target "$_entry_target" "$_entry_id")" || :
  done
  return 0
}

# shared_config_apply <volume-root> <home>
#
# The whole layout in one pass per entry: materialize it inside the volume, give
# it its home-shaped alias at the volume root, then link it into the home.
#
# Pre-existing REAL config in the home is never destroyed — the volume is
# seeded from the host with `devcontainer-cli config shared sync`, not by
# silently overwriting what is already in the container.
shared_config_apply() {
  _vol_root="$1"
  _home_root="$2"
  echo "${SC_ENTRIES:-}" | while read -r _entry_id _entry_kind _entry_target; do
    [ -n "$_entry_id" ] || continue
    _entry_src="$_vol_root/$_entry_id"

    # Materialize the entry inside the volume (a directory, or an empty file).
    if [ "$_entry_kind" = "dir" ]; then
      mkdir -p "$_entry_src"
    elif [ ! -e "$_entry_src" ]; then
      : > "$_entry_src"
    fi
    own "$_entry_src"

    link_or_keep "$_vol_root/$_entry_target" \
      "$(volume_alias_target "$_entry_target" "$_entry_id")" || :

    # Keeping real config is the documented outcome, so it must not fail the
    # loop — see the note in shared_config_volume_aliases.
    link_or_keep "$_home_root/$_entry_target" "$_entry_src" \
      "shared-config: keeping existing $_home_root/$_entry_target (not a symlink); run 'devcontainer-cli config shared sync' to seed the volume" || :
  done
  return 0
}

# shared_config_is_target <home-relative-path>
#
# True when the path lies inside one of the entries. The pipeline's exit status
# is the subshell's, so the match exits 0 from inside the loop and the fallthrough
# exits 1 — a `while` cannot set a variable the caller would see.
shared_config_is_target() {
  echo "${SC_ENTRIES:-}" | {
    while read -r _entry_id _entry_kind _entry_target; do
      [ -n "$_entry_target" ] || continue
      case "$1" in
        "$_entry_target"|"$_entry_target"/*) exit 0 ;;
      esac
    done
    exit 1
  }
}
