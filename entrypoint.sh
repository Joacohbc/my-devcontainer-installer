#!/bin/bash
# This script serves as the entrypoint for the Docker container.
# It ensures the SSH service is configured, creates a 'devuser' with a random password
# if it doesn't exist, sets a random password for the 'root' user if not already set,
# and then starts the SSH daemon.
# It also handles Docker-outside-Docker (DooD) setup by adding 'devuser' to the 'docker' group.

# Ensure the SSH service is configured correctly
if [ ! -d "/var/run/sshd" ]; then
    mkdir /var/run/sshd
fi

# Check if the devuser already exists
if [ $(id -u devuser 2>/dev/null || echo -1) -ge 0 ]; then
    echo "devuser password: $(cat /home/devuser/initial_password.txt)"
else
    # Create the devuser with a random password and sudo privileges
    useradd -m -s /bin/bash devuser
    DEV_PASSWORD=$(pwgen -s 32) # Generate a 32-character random password
    echo "devuser:$DEV_PASSWORD" | chpasswd
    usermod -aG sudo devuser
    
    echo "$DEV_PASSWORD" > /home/devuser/initial_password.txt
    chown devuser:devuser /home/devuser/initial_password.txt # Ensure devuser owns the file
    chmod 600 /home/devuser/initial_password.txt # Set appropriate permissions
    echo "devuser password: $(cat /home/devuser/initial_password.txt)"
fi

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

# Start the SSH service
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0