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
    # Defaults to manual, matching types.DefaultSkillMode: the installer writes
    # into the bind-mounted workspace, which is the user's own repository, so an
    # absent variable must not be read as permission to do that on every start.
    echo "${DEVCONTAINER_SKILLS_MODE:-manual}"
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

GLOBAL_FLAG=""
# Ask for global or project if running interactively
if [ -t 0 ]; then
    read -p "Install skills globally or in this project? [g/P] " -r scope
    if [[ "$scope" =~ ^[Gg] ]]; then
        GLOBAL_FLAG="--global"
        log "Installing agent skills globally"
    else
        log "Installing agent skills into $target"
    fi
else
    log "Installing agent skills into $target"
fi

# Group skills by source to run fewer npx commands
declare -A grouped_args
declare -A grouped_bare

for entry in "${REQUESTED_SKILLS[@]}"; do
    source="${entry%%#*}"
    name="${entry#*#}"
    
    if [ "$name" = "$entry" ]; then
        # If it's a bare source, we flag it so we don't pass --skill flags at all
        grouped_bare["$source"]="1"
    else
        # Append to the args for this source
        grouped_args["$source"]="${grouped_args[$source]:-} --skill $name"
    fi
done

failed=0
declare -A seen_sources
for source in "${!grouped_args[@]}" "${!grouped_bare[@]}"; do
    # Remove duplicates from the loop iteration since a source might be in both
    if [ "${seen_sources[$source]:-0}" = "1" ]; then
        continue
    fi
    seen_sources["$source"]="1"

    if [ "${grouped_bare[$source]:-0}" = "1" ]; then
        # Install the whole source
        log "Installing all from $source..."
        if [ -n "$GLOBAL_FLAG" ]; then
            if ! npx --yes skills add "$source" "$GLOBAL_FLAG"; then
                echo "ERROR: failed to install $source" >&2
                failed=1
            fi
        else
            if ! npx --yes skills add "$source"; then
                echo "ERROR: failed to install $source" >&2
                failed=1
            fi
        fi
    else
        # Install specific skills
        args="${grouped_args[$source]}"
        log "Installing specific skills from $source..."
        # We don't quote $args so the words split into separate arguments
        if [ -n "$GLOBAL_FLAG" ]; then
            # shellcheck disable=SC2086
            if ! npx --yes skills add "$source" $args "$GLOBAL_FLAG"; then
                echo "ERROR: failed to install skills from $source" >&2
                failed=1
            fi
        else
            # shellcheck disable=SC2086
            if ! npx --yes skills add "$source" $args; then
                echo "ERROR: failed to install skills from $source" >&2
                failed=1
            fi
        fi
    fi
done

exit "$failed"
