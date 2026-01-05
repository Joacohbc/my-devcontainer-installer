#!/bin/bash

# Ensure the SSH service is configured correctly
if [ ! -d "/var/run/sshd" ]; then
    mkdir /var/run/sshd
fi

# Check if the devuser already exists and set/update password
if [ -f /home/devuser/initial_password.txt ]; then
    echo "devuser password: $(cat /home/devuser/initial_password.txt)"
else
    # Create the devuser with a random password and sudo privileges (fallback)
    useradd -m -s /bin/bash devuser
    DEV_PASSWORD=$(pwgen -s 12) # Generate a 12-character random password
    echo "devuser:$DEV_PASSWORD" | chpasswd
    usermod -aG sudo devuser
    
    echo "$DEV_PASSWORD" > /home/devuser/initial_password.txt
    chown devuser:devuser /home/devuser/initial_password.txt
    chmod 600 /home/devuser/initial_password.txt
    echo "devuser password: $DEV_PASSWORD"
fi

# Check if root already has a password file
if [ -f /root/initial_password.txt ]; then
    echo "initial root password: $(cat /root/initial_password.txt)"
else
    # Set a password for the root user
    INITIAL_PASSWORD=$(pwgen -s 12)
    echo $INITIAL_PASSWORD > /root/initial_password.txt
    echo "root:$INITIAL_PASSWORD" | chpasswd
    echo "initial root password: $INITIAL_PASSWORD"
fi

# Fix Docker socket permissions at runtime
if [ -S /var/run/docker.sock ]; then
    setfacl -m "g:docker:rw" /var/run/docker.sock
    echo "Docker socket permissions updated."
fi

# Start the SSH service
/usr/sbin/sshd -D -o ListenAddress=0.0.0.0