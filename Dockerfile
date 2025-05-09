# This Dockerfile sets up a development environment based on Ubuntu 22.04.
# It installs OpenSSH server, sudo, pwgen, and Docker CLI for Docker-outside-Docker (DooD).
# SSH is configured to allow root login with passwords.
# The entrypoint script handles user creation and SSH service startup.

# Use an Ubuntu base image
FROM ubuntu:22.04

# Update packages and install SSH, sudo, and other utilities
RUN apt-get update && apt-get install -y \
    openssh-server \
    nano \
    sudo \
    pwgen \
    # To disable Docker-outside-Docker, comment out ca-certificates, curl, gnupg, lsb-release if they are not needed by other packages
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    && apt-get clean

# To disable Docker-outside-Docker, comment out the following lines for Docker GPG key and repository setup
# Add Docker's official GPG key
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Set up the repository
RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# To disable Docker-outside-Docker, comment out the next line
# Install Docker CLI
RUN apt-get update && apt-get install -y docker-ce-cli && apt-get clean

# Configure the SSH service
RUN mkdir /var/run/sshd && \
    chmod 755 /var/run/sshd # Ensure correct permissions

# Configure SSH to allow root login and password authentication
RUN sed -i 's/^#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
RUN sed -i 's/^#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config

# Expose port 22 for SSH
EXPOSE 22

# Copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set the entrypoint script
ENTRYPOINT ["/entrypoint.sh"]
