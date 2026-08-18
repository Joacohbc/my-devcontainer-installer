#!/bin/bash
#
# The container entrypoint.
#
# It is a DRIVER, not a program: every step below is a named stage function, and
# `main` at the bottom is the whole order in one readable list. Stages share
# state through globals on purpose — stage_identity computes DEV_UID/DEV_GID
# that three later stages need — so they are functions in one shell, not
# separate processes.
#
# Why functions in one file and not an /etc/entrypoint.d/ directory of numbered
# scripts: a stage directory is a seam, and nothing varies across it. No module
# and no user contributes an entrypoint stage. (Modules DO contribute installer
# scripts, and that directory-drop seam already exists — ~/post-script/start.d,
# run by the last stage below.) Functions give the same readability and the same
# test seam — a test extracts one and runs it against a temp tree — without
# inventing an extension point that would have exactly one adapter.
#
# ORDER IS LOAD-BEARING. Four edges, three of them pinned by tests:
#
#   stage_workspace     -> stage_identity        WORKSPACE_DIR decides the remap
#   stage_identity      -> everything after it   DEV_UID/DEV_GID, SC_OWNER
#   stage_shared_config -> stage_user_aliases    ~/.alias.sh must not win the
#                                                race against the volume symlink
#   stage_shared_config -> stage_context_skill   ~/.claude must already BE the
#                                                volume symlink, or `mkdir -p`
#                                                makes it a real directory and
#                                                the volume is never wired up

# ── The shared-config module ────────────────────────────────────────────────
# Its implementation is one library shared with the host-side `config shared
# sync`, so the volume layout has a single definition instead of one copy per
# caller. The catalogue it reads is rendered from types.SharedConfigEntries at
# generate time, so Go is the only place an entry is declared.
# DEV_HOME is where every stage below writes. It is a variable rather than ten
# literals so the stages can be run against a temp tree by a test — the same
# reason the workspace roots above it are.
DEV_HOME="/home/devuser"

SHARED_CONFIG_LIB="/usr/local/lib/devcontainer/shared-config.sh"
SHARED_CONFIG_TABLE="/usr/local/lib/devcontainer/shared-config-entries"
SHARED_CONFIG_DIR="/mnt/shared-config"

if [ -r "$SHARED_CONFIG_LIB" ]; then
    . "$SHARED_CONFIG_LIB"
else
    echo "entrypoint: $SHARED_CONFIG_LIB is missing; shared config and the global agent skill will be skipped" >&2
fi

# ── Stages ──────────────────────────────────────────────────────────────────

# sshd's runtime dir, the account the whole image is built around, and one
# random password each for devuser and root, generated once and kept.
stage_users() {
    if [ ! -d "/var/run/sshd" ]; then
        mkdir /var/run/sshd
    fi

    if ! id "devuser" &>/dev/null; then
        echo "Error: User 'devuser' does not exist. Please ensure it is created in the Dockerfile."
        exit 1
    fi

    if [ ! -f "$DEV_HOME/initial_password.txt" ]; then
        DEV_PASSWORD=$(pwgen -s 32)
        echo "devuser:$DEV_PASSWORD" | chpasswd
        echo "$DEV_PASSWORD" > "$DEV_HOME/initial_password.txt"
        chown devuser:devuser "$DEV_HOME/initial_password.txt"
        chmod 600 "$DEV_HOME/initial_password.txt"
    fi
    echo "devuser password: $(cat "$DEV_HOME/initial_password.txt")"

    if [ -f /root/initial_password.txt ]; then
        echo "initial root password: $(cat /root/initial_password.txt)"
    else
        INITIAL_PASSWORD=$(pwgen -s 32)
        echo $INITIAL_PASSWORD > /root/initial_password.txt
        echo "root:$INITIAL_PASSWORD" | chpasswd
        echo "initial root password: $INITIAL_PASSWORD"
    fi
}

# Resolve the project mount. New layouts mount it at /workspaces/<name> — a
# unique path per project so the path-keyed history of Claude Code/Antigravity
# never collides in the shared config volume. Older images mounted directly at
# /workspace still work (the glob finds nothing and the legacy mount is used).
#
# The short alias is /workspace/<name>, NOT a bare /workspace: agents key their
# session history by the directory they were started in, so a single path shared
# by every project merges all of their chats into one history. A bare alias
# silently undoes the whole point of the per-project mount, since that is the
# path people actually cd into.
WORKSPACE_MOUNT_ROOT=/workspaces
WORKSPACE_ALIAS_ROOT=/workspace
LEGACY_WORKSPACE_MOUNT=/workspace

alias_project_mount() {
    local project_mount="$1"
    # A container started by an older image carries a bare alias LINK in its
    # writable layer, which has to go before the alias root can be a directory.
    [ -L "$WORKSPACE_ALIAS_ROOT" ] && rm -f "$WORKSPACE_ALIAS_ROOT"
    mkdir -p "$WORKSPACE_ALIAS_ROOT"
    ln -sfn "$project_mount" "$WORKSPACE_ALIAS_ROOT/$(basename "$project_mount")"
}

stage_workspace() {
    WORKSPACE_DIR=$LEGACY_WORKSPACE_MOUNT
    for _project_mount in "$WORKSPACE_MOUNT_ROOT"/*; do
        [ -d "$_project_mount" ] || continue
        WORKSPACE_DIR="$_project_mount"
        alias_project_mount "$_project_mount"
        break
    done
}

# Align devuser's UID/GID with the owner of the mounted project dir so the
# container can read/write the bind mount WITHOUT ever modifying the host's
# original permissions (no chown/setfacl on the workspace). A local-cached image
# already bakes the host UID/GID at build time, so this is a no-op there; it only
# fires for prebuilt 'remote' images run on a host whose UID is not the baked
# default (the image has no other account squatting on the target id).
#
# It ends by publishing DEV_UID/DEV_GID and SC_OWNER, which every later stage
# reads — SC_OWNER is the shared-config library's one configuration knob.
stage_identity() {
    if [ -d "$WORKSPACE_DIR" ]; then
        WS_UID=$(stat -c %u "$WORKSPACE_DIR")
        WS_GID=$(stat -c %g "$WORKSPACE_DIR")
        CUR_UID=$(id -u devuser)
        CUR_GID=$(id -g devuser)
        if [ "$WS_UID" != "0" ] && { [ "$WS_UID" != "$CUR_UID" ] || [ "$WS_GID" != "$CUR_GID" ]; }; then
            # Renumber devuser's own group only when the target GID is free;
            # otherwise adopt the existing group as primary via usermod -g.
            if ! getent group "$WS_GID" >/dev/null; then
                groupmod -g "$WS_GID" devuser 2>/dev/null || true
            fi
            usermod -u "$WS_UID" -g "$WS_GID" devuser 2>/dev/null || usermod -u "$WS_UID" devuser 2>/dev/null || true
        fi
    fi

    DEV_UID=$(id -u devuser)
    DEV_GID=$(id -g devuser)
    SC_OWNER="$DEV_UID:$DEV_GID"

    # Re-own the persisted home to devuser's ACTUAL UID/GID, and only when it
    # drifted: this finishes a successful remap and repairs homes left owned by a
    # foreign UID after a failed one. A silent failure here breaks every shell rc
    # on SSH login, so surface it instead of swallowing the error. The home must
    # never be chowned to a UID devuser does not really have, and the workspace is
    # never touched.
    if [ "$(stat -c %u "$DEV_HOME")" != "$DEV_UID" ] || [ "$(stat -c %g "$DEV_HOME")" != "$DEV_GID" ]; then
        chown -R "$DEV_UID:$DEV_GID" "$DEV_HOME" || echo "warning: could not fully chown $DEV_HOME to $DEV_UID:$DEV_GID; shell rc files may not load" >&2
    fi

    if [ -S /var/run/docker.sock ]; then
        setfacl -m u:devuser:rw /var/run/docker.sock 2>/dev/null || true
    fi
}

# One global volume (mounted at SHARED_CONFIG_DIR) holds the config/sessions for
# AI/dev tools so logins persist across every container. The whole stage no-ops
# when the mount is absent (opt-out, or an image built before the mount existed),
# which keeps it safe for quick-run on older images.
stage_shared_config() {
    command -v shared_config_apply >/dev/null || return 0
    [ -d "$SHARED_CONFIG_DIR" ] || return 0
    [ -r "$SHARED_CONFIG_TABLE" ] || {
        echo "shared-config: $SHARED_CONFIG_TABLE is missing; skipping" >&2
        return 0
    }
    SC_ENTRIES="$(cat "$SHARED_CONFIG_TABLE")"

    # Re-own the volume to devuser's real UID only when it drifted (mirrors the
    # $DEV_HOME repair above); also fixes entries seeded by config shared sync.
    if [ "$(stat -c %u "$SHARED_CONFIG_DIR")" != "$DEV_UID" ]; then
        chown -R "$DEV_UID:$DEV_GID" "$SHARED_CONFIG_DIR" 2>/dev/null || true
    fi

    shared_config_apply "$SHARED_CONFIG_DIR" "$DEV_HOME"

    # Antigravity CLI 2.0 reads skills from ~/.gemini/antigravity-cli/skills but
    # does not understand ~/.agents/skills, where the shared agent skills live.
    # Bridge them with a symlink so the (persisted, shared) .agents skills are
    # visible to Antigravity. Both .agents and .gemini are symlinks into the
    # shared volume set up above, so this link persists across containers.
    local ag_skills_link="$DEV_HOME/.gemini/antigravity-cli/skills"
    local ag_shared_skills="$DEV_HOME/.agents/skills"
    # The bridge's target has to exist for Antigravity to read through it; the
    # link's own parent is created by link_or_keep.
    mkdir -p "$ag_shared_skills"
    own "$ag_shared_skills"
    link_or_keep "$ag_skills_link" "$ag_shared_skills"
}

# ~/.alias.sh is sourced by every shell after the image's baked defaults, so the
# user can redefine anything at any time. Normally it is a symlink into the
# shared-config volume (created by the stage above), which is what makes an edit
# apply to every container and survive a rebuild. When that volume is opted out
# of there is nothing to link, so create a plain local file instead — the file
# must always exist and be writable by devuser, or "edit your aliases" has no
# answer.
stage_user_aliases() {
    [ -e "$DEV_HOME/.alias.sh" ] && return 0
    su - devuser -c 'cat > "$HOME/.alias.sh"' <<'USER_ALIASES'
# Your own shell aliases and functions.
#
# Sourced by every shell AFTER the CLI's baked defaults
# (~/.devcontainer_aliases.sh), so anything defined here wins. Changes take
# effect in the next shell — no rebuild, no restart.
#
# NOTE: the shared-config volume is not mounted in this container, so this file
# is local to it and is lost when the container is recreated. Configure aliases
# on the host with `devcontainer-cli config alias set` (and `config alias sync`)
# to have them persist across every container.
USER_ALIASES
    chown "$DEV_UID:$DEV_GID" "$DEV_HOME/.alias.sh" 2>/dev/null || true
}

# Every image bakes ~/.devcontainer-skills/devcontainer-context/SKILL.md (see the
# aliases Dockerfile module): a small always-installed skill telling an agent it
# is inside a devcontainer and to read ~/CONTEXT.md / run
# get-devcontainer-context before assuming anything about the environment.
#
# Agents look for global skills inside their own config dir, so link it into the
# canonical ~/.agents/skills store (which the Antigravity bridge above already
# follows) and into ~/.claude/skills. A SYMLINK is deliberate: those directories
# usually live in the shared volume, so the link always resolves to THIS image's
# copy instead of persisting a stale one for every other container. link_or_keep
# never clobbers a real directory, so a skill the user installed under that name
# is kept.
BAKED_SKILLS_DIR="$DEV_HOME/.devcontainer-skills"

stage_context_skill() {
    command -v link_or_keep >/dev/null || return 0
    [ -d "$BAKED_SKILLS_DIR/devcontainer-context" ] || return 0
    local skills_dir skill_link
    for skills_dir in "$DEV_HOME/.agents/skills" "$DEV_HOME/.claude/skills"; do
        skill_link="$skills_dir/devcontainer-context"
        link_or_keep "$skill_link" "$BAKED_SKILLS_DIR/devcontainer-context" \
            "skills: keeping existing $skill_link (not a symlink)"
    done
}

# Scripts under post-script/start.d/ are the non-interactive installers
# (Claude Code, Antigravity, Copilot, OpenCode, then the agent-wiring tools
# Graphify/Caveman) the generator marked as auto-start. The ENTIRE block runs as
# devuser (never root): a single 'su - devuser' login shell owns the loop, the
# sentinels and the logs, so nothing under the home is left root-owned. It runs
# in the BACKGROUND so SSH comes up immediately, and only once per container — a
# per-script ".done" sentinel under ~/.post-script-state skips already-installed
# tools across stop/start (a fresh container has no sentinel and reinstalls).
# The numeric "NN-" filename prefix drives run order via the sorted glob. Each
# script runs through a login shell (bash -l) so node/python/.local/bin from the
# shell-init files are on PATH. The whole stage no-ops when the dir is absent.
stage_post_scripts() {
    [ -d "$DEV_HOME/post-script/start.d" ] || return 0
    su - devuser -s /bin/bash -c '
        start_dir="$HOME/post-script/start.d"
        state_dir="$HOME/.post-script-state"
        mkdir -p "$state_dir"
        for script in "$start_dir"/*.sh; do
            [ -e "$script" ] || continue
            name="$(basename "$script")"
            done_marker="$state_dir/$name.done"
            [ -f "$done_marker" ] && continue
            log="$state_dir/$name.log"
            echo "post-script: running $name as $(id -un) (log: $log)"
            if bash -l "$script" > "$log" 2>&1; then
                : > "$done_marker"
                echo "post-script: $name completed"
            else
                echo "post-script: $name FAILED (see $log)" >&2
            fi
        done
    ' &
}

# ── The order ───────────────────────────────────────────────────────────────

main() {
    stage_users
    stage_workspace
    stage_identity
    stage_shared_config
    stage_user_aliases
    stage_context_skill
    stage_post_scripts

    # sshd is exec'd, not backgrounded: it must be PID 1 so a `docker stop`
    # reaches it and the container's lifetime is its lifetime.
    exec /usr/sbin/sshd -D -o ListenAddress=0.0.0.0
}

main "$@"
