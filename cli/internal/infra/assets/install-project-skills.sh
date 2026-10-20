#!/usr/bin/env bash
# Install the project's agent skills into the workspace, with the Skills CLI.
#
# Skills are installed project-scoped, into the bind-mounted workspace. Only a
# hand-run in a terminal is offered a global install, and it says what that
# means: ~/.claude and ~/.agents are symlinks into the shared-config volume, so
# a global install reaches every other container that mounts it. Unattended
# runs are always project-scoped.
#
# Which skills and which mode are read from the environment rather than baked
# into the image, so changing either is a compose change and not a rebuild.
set -euo pipefail

log() { echo "==> $*"; }

skills_mode() {
    # Defaults to manual, matching types.DefaultSkillMode: the installer writes
    # into the bind-mounted workspace, which is the user's own repository, so an
    # absent variable must not be read as permission to do that on every start.
    echo "${DEVCONTAINER_SKILLS_MODE:-manual}"
}

workspace_dir() {
    # SKILLS_TARGET_DIR is a test seam: the workspace mount only exists inside
    # a container.
    if [ -n "${SKILLS_TARGET_DIR:-}" ]; then
        echo "$SKILLS_TARGET_DIR"
        return 0
    fi
    # Three layouts, newest first: the per-project mount, its short alias, and
    # the bare /workspace of images built before the split.
    for candidate in /workspaces/*/ /workspace/*/ /workspace/; do
        [ -d "$candidate" ] && { echo "${candidate%/}"; return 0; }
    done
    return 1
}

read -r -a REQUESTED_SKILLS <<<"${DEVCONTAINER_SKILLS:-}"

if [ "${#REQUESTED_SKILLS[@]}" -eq 0 ]; then
    log "No skills requested (${DEVCONTAINER_SKILLS_MODE:+mode $DEVCONTAINER_SKILLS_MODE})"
    exit 0
fi

# Called by the entrypoint on every container start; in manual mode the user
# runs it themselves, so the automatic pass has nothing to do. Running it by
# hand always installs, which is the whole point of the mode.
if [ "$(skills_mode)" = "manual" ] && [ "${1:-}" = "--auto" ]; then
    log "Skills mode is manual; run '${0##*/}' or the install_skills alias to install skills."
    exit 0
fi

if ! command -v npx >/dev/null 2>&1; then
    echo "ERROR: npx is required to install agent skills but is not installed" >&2
    exit 1
fi

if ! target="$(workspace_dir)"; then
    echo "ERROR: no workspace mount found; cannot install project-scoped skills" >&2
    exit 1
fi

cd "$target"

log "The following skills are configured for this project:"
for entry in "${REQUESTED_SKILLS[@]}"; do
    echo "  - $entry"
done
echo ""

# Flags shared by every `skills add` below. The `--yes` right after `npx` only
# answers npx; the Skills CLI has its own pickers (target agents, which skills
# of a repo), so an unattended run (start.d, `--auto`, no TTY) passes its
# --yes too, or it waits on a prompt nobody can answer.
EXTRA_FLAGS=()
# Ask for global or project if running interactively
if [ -t 0 ] && [ "${1:-}" != "--auto" ]; then
    echo "A global install lands in the shared-config volume and reaches every devcontainer."
    read -p "Install skills globally or in this project? [g/P] " -r scope
    if [[ "$scope" =~ ^[Gg] ]]; then
        EXTRA_FLAGS+=(--global)
        log "Installing agent skills globally"
    else
        log "Installing agent skills into $target"
    fi
else
    EXTRA_FLAGS+=(--yes)
    log "Installing agent skills into $target"
fi

# Group skills by source to run fewer npx commands. Each entry is one token
# (`source` or `source#skill`), so the selectors are kept as a
# newline-separated list and expanded into an array, never word-split.
declare -A grouped_names
declare -A grouped_bare
sources=()

for entry in "${REQUESTED_SKILLS[@]}"; do
    source="${entry%%#*}"
    name="${entry#*#}"
    if [ -z "${grouped_names[$source]+set}" ] && [ -z "${grouped_bare[$source]+set}" ]; then
        sources+=("$source")
    fi
    if [ "$name" = "$entry" ]; then
        # A bare source installs everything it ships: no --skill selectors.
        grouped_bare["$source"]="1"
    else
        grouped_names["$source"]="${grouped_names[$source]:-}${name}"$'\n'
    fi
done

failed=0
for source in "${sources[@]}"; do
    args=()
    if [ "${grouped_bare[$source]:-0}" = "1" ]; then
        log "Installing all from $source..."
    else
        log "Installing specific skills from $source..."
        while IFS= read -r name; do
            [ -n "$name" ] && args+=(--skill "$name")
        done <<<"${grouped_names[$source]:-}"
    fi
    if ! npx --yes skills add "$source" "${args[@]}" "${EXTRA_FLAGS[@]}"; then
        echo "ERROR: failed to install skills from $source" >&2
        failed=1
    fi
done

exit "$failed"
