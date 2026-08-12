#!/usr/bin/env bash
# Scraper profile — the agent skills for the tools installed at build time.
#
# This is `when: start`, not `when: build`, and that is not a preference.
# ~/.claude and ~/.agents are symlinks into the devcontainer-shared-config
# volume, which only exists once a container is running. A skill written during
# the build lands in the image's own directory, which the volume then shadows —
# so it would be invisible in every container that mounts the volume, and stale
# in every one that does not. The entrypoint runs this once per container, after
# the shared-config block has wired those symlinks up.
#
# Consequence worth knowing: the skill is installed into the *shared* volume, so
# it becomes visible to every container using it, not only to scraper projects.
# That is inherent to where skills are stored here, not something this script
# chooses.
set -euo pipefail

# Skills to install globally, as the Skills CLI addresses them.
SKILLS=(
    firecrawl/cli
)

log() { echo "==> $*"; }

if ! command -v npx >/dev/null 2>&1; then
    # Nothing to retry: this profile ships nodejs, so npx is missing only if the
    # profile was copied and stripped down. Say so and succeed, or the entrypoint
    # would run this again on every start forever.
    echo "WARNING: npx is not installed, skipping agent skills (${SKILLS[*]})" >&2
    exit 0
fi

# The skills land in the shared volume, so a second container would reinstall
# what the first already put there.
skill_installed() {
    local name="${1##*/}"
    [ -e "$HOME/.agents/skills/$name" ] || [ -e "$HOME/.claude/skills/$name" ]
}

failed=0
for skill in "${SKILLS[@]}"; do
    if skill_installed "$skill"; then
        log "Skill $skill already present, skipping"
        continue
    fi
    log "Installing skill $skill"
    # A failure here is usually the network, which is worth retrying: exiting
    # non-zero leaves the entrypoint's .done sentinel unwritten, so the next
    # container start tries again.
    if ! npx --yes skills add -g "$skill"; then
        echo "ERROR: failed to install skill $skill" >&2
        failed=1
    fi
done

exit "$failed"
