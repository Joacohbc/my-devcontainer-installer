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
    zsh \
    fontconfig \
    # To disable Docker-outside-Docker, comment out ca-certificates, curl, gnupg, lsb-release if they are not needed by other packages
    ca-certificates \
    curl \
    gnupg \
    lsb-release

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
RUN apt-get update && apt-get install -y docker-ce-cli

# Create devuser with sudo privileges
RUN useradd -m -s /bin/zsh devuser && \
    usermod -aG sudo devuser && \
    echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Configure Docker socket permissions for devuser
RUN groupadd docker
RUN usermod -aG docker devuser

# Install Zsh configuration and plugins
COPY zsh-installer.sh /tmp/zsh-installer.sh
RUN chmod +x /tmp/zsh-installer.sh
RUN su - devuser -c "/tmp/zsh-installer.sh"
RUN rm /tmp/zsh-installer.sh

# Install prerequisites for development tools
RUN apt-get update && apt-get install -y \
    git \
    wget \
    apt-transport-https

# Install Java (Temurin JDK 11 & 17) & Maven
RUN wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add - && \
    echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" | tee /etc/apt/sources.list.d/adoptium.list && \
    apt-get update && \
    apt-get install -y temurin-17-jdk temurin-11-jdk maven

# Install Python
RUN apt-get update && apt-get install -y python3 python3-pip

# Install SQLite
RUN apt-get update && apt-get install -y sqlite3

# Install Go
COPY golang_utils.sh /tmp/golang_utils.sh
RUN bash -c "source /tmp/golang_utils.sh && install_golang" && rm /tmp/golang_utils.sh

# Clean up
RUN apt-get autoremove -y && \
    apt-get autoclean && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /tmp/* && \
    rm -rf /var/tmp/*

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
