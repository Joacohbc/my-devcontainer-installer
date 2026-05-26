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

# Fix Docker socket permissions at runtime
if [ -S /var/run/docker.sock ]; then
    setfacl -m "g:docker:rw" /var/run/docker.sock
    echo "Docker socket permissions updated."
fi

# Propagate DOCKER_HOST to SSH sessions. Compose `environment:` vars only reach
# PID 1 (this script); SSH login shells start with a fresh environment, so the
# docker CLI would otherwise fall back to the missing /var/run/docker.sock.
# pam_env reads /etc/environment for every SSH session, shell-agnostic.
if [ -n "$DOCKER_HOST" ]; then
    sed -i '/^DOCKER_HOST=/d' /etc/environment 2>/dev/null || true
    echo "DOCKER_HOST=$DOCKER_HOST" >> /etc/environment
    echo "DOCKER_HOST propagated to SSH sessions: $DOCKER_HOST"
fi

# Start the SSH service
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0