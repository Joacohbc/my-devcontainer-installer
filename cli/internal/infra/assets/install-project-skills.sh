#!/usr/bin/env bash
# Install the project's agent skills into the workspace, with the Skills CLI.
#
# Skills are installed project-scoped, into the bind-mounted workspace, never
# globally: ~/.claude and ~/.agents are symlinks into the shared-config volume,
# so a global install would leak this project's skills into every other
# container that mounts it.
#
# Which skills and which mode are read from the environment rather than baked
# into the image, so changing either is a compose change and not a rebuild.
set -euo pipefail

log() { echo "==> $*"; }

skills_mode() {
    echo "${DEVCONTAINER_SKILLS_MODE:-auto}"
}

workspace_dir() {
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
    log "Skills mode is manual; run '${0##*/}' or the install_skills alias to install: ${REQUESTED_SKILLS[*]}"
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
log "Installing agent skills into $target"

# An entry is "<source>" or "<source>#<skill>". The selector form is what lets a
# repo holding several skills be named without pinning its branch, which a
# directory URL would.
install_entry() {
    local entry="$1" source="${1%%#*}" name="${1#*#}"
    if [ "$name" = "$entry" ]; then
        npx --yes skills add "$source"
        return
    fi
    npx --yes skills add "$source" --skill "$name"
}

failed=0
for entry in "${REQUESTED_SKILLS[@]}"; do
    log "  $entry"
    # Exiting non-zero leaves the entrypoint's .done sentinel unwritten, so a
    # failure that is really a network blip is retried on the next start.
    if ! install_entry "$entry"; then
        echo "ERROR: failed to install skill $entry" >&2
        failed=1
    fi
done

exit "$failed"
