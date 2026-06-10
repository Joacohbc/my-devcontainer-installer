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

# Start the SSH service
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0