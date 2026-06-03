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

# Grant devuser full ACL access to /workspace so it owns the directory by default
if [ -d /workspace ]; then
    setfacl -R -m u:devuser:rwx /workspace 2>/dev/null || true
    setfacl -d -m u:devuser:rwx /workspace 2>/dev/null || true
fi

if [ -S /var/run/docker.sock ]; then
    setfacl -m u:devuser:rw /var/run/docker.sock 2>/dev/null || true
fi

# Start the SSH service
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0