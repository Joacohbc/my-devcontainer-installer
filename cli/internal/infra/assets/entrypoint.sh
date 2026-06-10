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

# Align devuser's UID/GID with the owner of the mounted /workspace so the
# container can read/write the bind mount WITHOUT ever modifying the host's
# original permissions (no chown/setfacl on /workspace).
if [ -d /workspace ]; then
    WS_UID=$(stat -c %u /workspace)
    WS_GID=$(stat -c %g /workspace)
    CUR_UID=$(id -u devuser)
    CUR_GID=$(id -g devuser)
    if [ "$WS_UID" != "0" ] && { [ "$WS_UID" != "$CUR_UID" ] || [ "$WS_GID" != "$CUR_GID" ]; }; then
        # Ubuntu >= 23.10 ships a stock "ubuntu" user that already holds
        # UID/GID 1000, which makes the usermod below fail silently; remove any
        # account squatting on the target UID before remapping.
        CONFLICT_USER=$(getent passwd "$WS_UID" | cut -d: -f1)
        if [ -n "$CONFLICT_USER" ] && [ "$CONFLICT_USER" != "devuser" ]; then
            userdel -r "$CONFLICT_USER" 2>/dev/null || userdel "$CONFLICT_USER" 2>/dev/null || true
        fi
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
# foreign UID after a failed one (which broke every shell rc on SSH login).
# The home must never be chowned to a UID devuser does not really have, and
# /workspace is never touched.
DEV_UID=$(id -u devuser)
DEV_GID=$(id -g devuser)
if [ "$(stat -c %u /home/devuser)" != "$DEV_UID" ] || [ "$(stat -c %g /home/devuser)" != "$DEV_GID" ]; then
    chown -R "$DEV_UID:$DEV_GID" /home/devuser 2>/dev/null || true
fi

if [ -S /var/run/docker.sock ]; then
    setfacl -m u:devuser:rw /var/run/docker.sock 2>/dev/null || true
fi

# ── Shared persistent tool config ──────────────────────────────────────────
# One global volume (mounted at /mnt/shared-config) holds the config/sessions
# for AI/dev tools so logins persist across every container. Each entry below is
# materialized inside the volume and symlinked into devuser's home. The whole
# block no-ops when the mount is absent (opt-out, or an image built before the
# mount existed), which keeps it safe for quick-run on older images.
# Each row: "<volume-subpath> <dir|file> <home-relative-target>".
# Keep in sync with types.SharedConfigEntries (internal/domain/types/sharedconfig.go).
SHARED_CONFIG_DIR="/mnt/shared-config"
if [ -d "$SHARED_CONFIG_DIR" ]; then
    # Recursive chown only when the volume root drifted from devuser's real UID
    # (cheap top-level chown happens per-entry below); mirrors the /home/devuser
    # drift-repair above.
    if [ "$(stat -c %u "$SHARED_CONFIG_DIR")" != "$DEV_UID" ]; then
        chown -R "$DEV_UID:$DEV_GID" "$SHARED_CONFIG_DIR" 2>/dev/null || true
    fi
    while read -r entry_id entry_kind entry_target; do
        [ -z "$entry_id" ] && continue
        src="$SHARED_CONFIG_DIR/$entry_id"
        dest="/home/devuser/$entry_target"

        # 1. Materialize the entry inside the volume.
        if [ "$entry_kind" = "dir" ]; then
            mkdir -p "$src"
        elif [ ! -e "$src" ]; then
            touch "$src"
        fi
        chown "$DEV_UID:$DEV_GID" "$src" 2>/dev/null || true

        # 2. Ensure the target's parent exists (e.g. ~/.config for gh).
        destparent=$(dirname "$dest")
        if [ ! -d "$destparent" ]; then
            mkdir -p "$destparent"
            chown "$DEV_UID:$DEV_GID" "$destparent" 2>/dev/null || true
        fi

        # 3. Link, adopting pre-existing real content when the volume side is empty.
        if [ -L "$dest" ]; then
            ln -sfn "$src" "$dest"                                 # repair/repoint
        elif [ -d "$dest" ]; then
            if [ -n "$(ls -A "$src" 2>/dev/null)" ]; then
                if [ -z "$(ls -A "$dest" 2>/dev/null)" ]; then
                    rmdir "$dest" && ln -s "$src" "$dest"          # replace empty real dir
                else
                    echo "shared-config: keeping non-empty $dest (volume already has data)" >&2
                fi
            else
                cp -a "$dest/." "$src/" 2>/dev/null || true        # adopt into empty volume
                chown -R "$DEV_UID:$DEV_GID" "$src" 2>/dev/null || true
                rm -rf "$dest" && ln -s "$src" "$dest"
            fi
        elif [ -f "$dest" ]; then
            if [ ! -s "$src" ] && [ -s "$dest" ]; then
                cat "$dest" > "$src" 2>/dev/null || true           # adopt file content
                chown "$DEV_UID:$DEV_GID" "$src" 2>/dev/null || true
            fi
            if [ ! -s "$dest" ] || [ ! -s "$src" ] || cmp -s "$dest" "$src"; then
                rm -f "$dest" && ln -s "$src" "$dest"
            else
                echo "shared-config: keeping $dest (both file and volume copy non-empty)" >&2
            fi
        else
            ln -s "$src" "$dest"
        fi
        [ -L "$dest" ] && chown -h "$DEV_UID:$DEV_GID" "$dest" 2>/dev/null || true
    done <<'SHARED_CONFIG_ENTRIES'
claude dir .claude
claude.json file .claude.json
antigravity dir .antigravity
antigravity-config dir .config/antigravity
gemini dir .gemini
codex dir .codex
gh dir .config/gh
SHARED_CONFIG_ENTRIES
fi

# Start the SSH service
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0