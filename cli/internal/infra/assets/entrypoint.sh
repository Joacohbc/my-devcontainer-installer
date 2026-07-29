#!/bin/bash

# Ensure the SSH service is configured correctly
if [ ! -d "/var/run/sshd" ]; then
    mkdir /var/run/sshd
fi

# Check if the devuser exists (it should be created in the Dockerfile)
if ! id "devuser" &>/dev/null; then
    echo "Error: User 'devuser' does not exist. Please ensure it is created in the Dockerfile."
    exit 1
fi

# Ensure devuser has a password
if [ ! -f /home/devuser/initial_password.txt ]; then
    DEV_PASSWORD=$(pwgen -s 32) # Generate a 32-character random password
    echo "devuser:$DEV_PASSWORD" | chpasswd
    
    echo "$DEV_PASSWORD" > /home/devuser/initial_password.txt
    chown devuser:devuser /home/devuser/initial_password.txt # Ensure devuser owns the file
    chmod 600 /home/devuser/initial_password.txt # Set appropriate permissions
fi

echo "devuser password: $(cat /home/devuser/initial_password.txt)"

# Check if root already has a password file
if [ -f /root/initial_password.txt ]; then
    echo "initial root password: $(cat /root/initial_password.txt)"
else
    # Set a password for the root user
    INITIAL_PASSWORD=$(pwgen -s 32)
    echo $INITIAL_PASSWORD > /root/initial_password.txt
    echo "root:$INITIAL_PASSWORD" | chpasswd
    echo "initial root password: $INITIAL_PASSWORD"
fi

# Resolve the project mount. New layouts mount it at /workspaces/<name> — a
# unique path per project so the path-keyed history of Claude Code/Antigravity
# never collides in the shared config volume — and keep /workspace as a stable
# alias. Older images mounted directly at /workspace still work (the glob finds
# nothing and WORKSPACE_DIR stays /workspace).
WORKSPACE_DIR=/workspace
for _ws in /workspaces/*; do
    [ -d "$_ws" ] || continue
    WORKSPACE_DIR="$_ws"
    [ -e /workspace ] || ln -s "$_ws" /workspace
    break
done

# Align devuser's UID/GID with the owner of the mounted project dir so the
# container can read/write the bind mount WITHOUT ever modifying the host's
# original permissions (no chown/setfacl on the workspace). A local-cached image
# already bakes the host UID/GID at build time, so this is a no-op there; it only
# fires for prebuilt 'remote' images run on a host whose UID is not the baked
# default (the image has no other account squatting on the target id).
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

# Re-own the persisted home to devuser's ACTUAL UID/GID, and only when it
# drifted: this finishes a successful remap and repairs homes left owned by a
# foreign UID after a failed one. A silent failure here breaks every shell rc on
# SSH login, so surface it instead of swallowing the error. The home must never
# be chowned to a UID devuser does not really have, and the workspace is never
# touched.
DEV_UID=$(id -u devuser)
DEV_GID=$(id -g devuser)
if [ "$(stat -c %u /home/devuser)" != "$DEV_UID" ] || [ "$(stat -c %g /home/devuser)" != "$DEV_GID" ]; then
    chown -R "$DEV_UID:$DEV_GID" /home/devuser || echo "warning: could not fully chown /home/devuser to $DEV_UID:$DEV_GID; shell rc files may not load" >&2
fi

if [ -S /var/run/docker.sock ]; then
    setfacl -m u:devuser:rw /var/run/docker.sock 2>/dev/null || true
fi

# ── Shared persistent tool config ──────────────────────────────────────────
# One global volume (mounted at /mnt/shared-config) holds the config/sessions
# for AI/dev tools so logins persist across every container. Each entry below is
# materialized inside the volume and symlinked into devuser's home. The whole
# block no-ops when the mount is absent (opt-out, or an image built before the
# mount existed), which keeps it safe for quick-run on older images. Pre-existing
# real config in the home is never destroyed; seed the volume from the host with
# 'devcontainer-cli config shared sync'.
# Each row: "<volume-subpath> <dir|file> <home-relative-target>".
# Keep in sync with types.SharedConfigEntries (internal/domain/types/sharedconfig.go).
SHARED_CONFIG_DIR="/mnt/shared-config"
if [ -d "$SHARED_CONFIG_DIR" ]; then
    # Re-own the volume to devuser's real UID only when it drifted (mirrors the
    # /home/devuser repair above); also fixes entries seeded by config shared sync.
    if [ "$(stat -c %u "$SHARED_CONFIG_DIR")" != "$DEV_UID" ]; then
        chown -R "$DEV_UID:$DEV_GID" "$SHARED_CONFIG_DIR" 2>/dev/null || true
    fi
    while read -r entry_id entry_kind entry_target; do
        [ -z "$entry_id" ] && continue
        src="$SHARED_CONFIG_DIR/$entry_id"
        dest="/home/devuser/$entry_target"

        # Materialize the entry inside the volume (dir, or empty file).
        if [ "$entry_kind" = "dir" ]; then
            mkdir -p "$src"
        elif [ ! -e "$src" ]; then
            touch "$src"
        fi
        chown "$DEV_UID:$DEV_GID" "$src" 2>/dev/null || true

        # Link into the home, never clobbering pre-existing real (non-symlink)
        # config. ln -sfn both creates a missing link and repoints a stale one.
        if [ -e "$dest" ] && [ ! -L "$dest" ]; then
            echo "shared-config: keeping existing $dest (not a symlink); run 'devcontainer-cli config shared sync' to seed the volume" >&2
            continue
        fi
        destparent="$(dirname "$dest")"
        [ -d "$destparent" ] || { mkdir -p "$destparent" && chown "$DEV_UID:$DEV_GID" "$destparent" 2>/dev/null; }
        ln -sfn "$src" "$dest"
        chown -h "$DEV_UID:$DEV_GID" "$dest" 2>/dev/null || true
    done <<'SHARED_CONFIG_ENTRIES'
claude dir .claude
claude.json file .claude.json
antigravity dir .antigravity
antigravity-config dir .config/antigravity
gemini dir .gemini
agents dir .agents
codex dir .codex
gh dir .config/gh
alias.sh file .alias.sh
SHARED_CONFIG_ENTRIES

    # Antigravity CLI 2.0 reads skills from ~/.gemini/antigravity-cli/skills but
    # does not understand ~/.agents/skills, where the shared agent skills live.
    # Bridge them with a symlink so the (persisted, shared) .agents skills are
    # visible to Antigravity. Both .agents and .gemini are symlinks into the
    # shared volume set up above, so this link persists across containers. Never
    # clobber a real (non-symlink) skills dir.
    ag_skills_parent="/home/devuser/.gemini/antigravity-cli"
    ag_skills_link="$ag_skills_parent/skills"
    if [ ! -e "$ag_skills_link" ] || [ -L "$ag_skills_link" ]; then
        mkdir -p "$ag_skills_parent" "/home/devuser/.agents/skills"
        ln -sfn "/home/devuser/.agents/skills" "$ag_skills_link"
        chown -h "$DEV_UID:$DEV_GID" "$ag_skills_link" 2>/dev/null || true
        chown "$DEV_UID:$DEV_GID" "$ag_skills_parent" "/home/devuser/.agents/skills" 2>/dev/null || true
    fi
fi

# ── The user's own alias file ───────────────────────────────────────────────
# ~/.alias.sh is sourced by every shell after the image's baked defaults, so the
# user can redefine anything at any time. Normally it is a symlink into the
# shared-config volume (created above), which is what makes an edit apply to
# every container and survive a rebuild. When that volume is opted out of there
# is nothing to link, so create a plain local file instead — the file must always
# exist and be writable by devuser, or "edit your aliases" has no answer.
if [ ! -e /home/devuser/.alias.sh ]; then
    su - devuser -c 'cat > "$HOME/.alias.sh"' <<'USER_ALIASES'
# Your own shell aliases and functions.
#
# Sourced by every shell AFTER the CLI's baked defaults
# (~/.devcontainer_aliases.sh), so anything defined here wins. Changes take
# effect in the next shell — no rebuild, no restart.
#
# NOTE: the shared-config volume is not mounted in this container, so this file
# is local to it and is lost when the container is recreated. Edit it on the host
# with `devcontainer-cli config alias edit` to have it persist everywhere.
USER_ALIASES
    chown "$DEV_UID:$DEV_GID" /home/devuser/.alias.sh 2>/dev/null || true
fi

# ── Auto-run non-interactive installer post-scripts ─────────────────────────
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
# shell-init files are on PATH. The whole block no-ops when the dir is absent.
if [ -d /home/devuser/post-script/start.d ]; then
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
fi

# Start the SSH service
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0