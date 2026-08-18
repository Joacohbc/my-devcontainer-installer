#!/bin/sh
# The host-side half of the shared-config module: the symlink pass that only the
# `config shared sync` helper runs.
#
# It is a separate file from shared-config.sh because it needs something the
# other caller does not have. Every function here resolves links against the
# HOST home mounted read-only at /host, which exists only inside the throwaway
# helper container; a running devcontainer has no such mount and no use for any
# of it. Shipping it in the image would be dead weight baked into every layer.
#
# It is loaded ON TOP of shared-config.sh (service/sharedconfig.go concatenates
# library + this file + the per-entry body), so shared_config_is_target is
# already defined when fix_symlink calls it.
#
# Inputs, all passed as environment by the caller:
#
#   SC_ENTRIES      the catalogue (read through shared_config_is_target)
#   ENTRY_HOST_HOME the host home an absolute link target is rewritten FROM
#   ENTRY_DEV_HOME  the container home it is rewritten TO
#
# Mounts: the volume at /vol, the host home read-only at /host.
#
# POSIX sh, not bash: the helper image runs `sh -c`, which on Ubuntu is dash.
# Behaviour is covered by TestSyncEntryScriptFixesSymlinks, which runs the
# assembled program against a temp tree.

# fix_symlinks <copied-tree> <source-tree> — makes symlinks inside a copied
# entry mean the same thing in the volume as they did on the host. It matters
# for entries like "claude"/"codex"/"agents": tools such as the skills.sh CLI
# (`npx skills add -g`) install global skills/agents into a canonical
# ~/.agents/skills store and symlink them into each agent's own config dir
# (e.g. ~/.claude/skills/<name> -> ~/.agents/skills/<name>).
#
# Every link is resolved against the SOURCE tree under /host, never against the
# copy — that is the whole point. Resolving in the destination (what this used
# to do) can only ever succeed for links that stay inside the entry, i.e. the
# ones that need no help at all, and silently left every cross-entry link
# dangling in the volume and therefore in every container.
#
# Three outcomes, in order:
#
#   - the target is inside another shared entry -> keep it a LINK, so both sides
#     stay one store instead of drifting copies. A relative link is left
#     verbatim (the volume aliases make it resolve); an absolute host path is
#     repointed at ENTRY_DEV_HOME, the home the volume is symlinked into.
#   - the target is outside the shared entries but exists on the host -> replace
#     the link with a real copy of that content, which is the only way it can
#     survive into a container.
#   - the target does not exist on the host either -> left as-is. `cp -aL` used
#     to error "cannot stat" here and failed the whole sync for entries like
#     ~/.claude, where Claude Code leaves a dangling `debug/latest` pointer;
#     leaving it dangling is no worse than it already was on the host.
UNMATCHABLE_HOST_HOME=/nonexistent-host-home

symlink_target_is_absolute() {
  case "$(readlink "$1")" in
    /*) return 0 ;;
  esac
  return 1
}

resolve_symlink_in_source() {
  _source_link_path="$1"
  _raw_target="$(readlink "$_source_link_path")"
  # An unset host home would leave the pattern below as a bare "/*", matching
  # (and rewriting) every absolute link target.
  _host_home="${ENTRY_HOST_HOME:-$UNMATCHABLE_HOST_HOME}"
  case "$_raw_target" in
    "$_host_home"/*) _raw_target="/host/${_raw_target#"$_host_home"/}" ;;
    /*) ;;
    *) _raw_target="$(dirname "$_source_link_path")/$_raw_target" ;;
  esac
  readlink -m "$_raw_target"
}

home_relative_path() {
  case "$1" in
    /host/*) printf '%s' "${1#/host/}" ;;
  esac
}

source_link_of() {
  _path_within_tree="${1#"$2"}"
  printf '%s' "$3/${_path_within_tree#/}"
}

repoint_at_container_home() {
  rm -rf "$1"
  ln -sfn "$ENTRY_DEV_HOME/$2" "$1"
}

replace_with_real_copy() {
  rm -rf "$1"
  cp -a "$2" "$1"
}

fix_symlink() {
  _copied_link="$1"
  _source_link="$2"
  [ -L "$_copied_link" ] || return 0
  [ -L "$_source_link" ] || return 0

  _resolved_target="$(resolve_symlink_in_source "$_source_link")"
  _shared_entry_path="$(home_relative_path "$_resolved_target")"

  if [ -n "$_shared_entry_path" ] && shared_config_is_target "$_shared_entry_path"; then
    symlink_target_is_absolute "$_source_link" || return 0
    repoint_at_container_home "$_copied_link" "$_shared_entry_path"
    return 0
  fi

  [ -e "$_resolved_target" ] || return 0
  replace_with_real_copy "$_copied_link" "$_resolved_target"
}

fix_symlinks() {
  _copied_tree="$1"
  _source_tree="$2"
  _found_links="$(mktemp)"
  find "$_copied_tree" -type l > "$_found_links"
  while IFS= read -r _found_link; do
    fix_symlink "$_found_link" "$(source_link_of "$_found_link" "$_copied_tree" "$_source_tree")"
  done < "$_found_links"
  rm -f "$_found_links"
  return 0
}
