# This Dockerfile sets up a development environment based on Ubuntu 22.04.
# It installs OpenSSH server, sudo, pwgen, Docker CLI, Java, Python, and Node.js development tools.
# SSH is configured to allow root login with passwords.
# The entrypoint script handles user creation and SSH service startup.

# Use an Ubuntu base image
FROM ubuntu:22.04

# Update packages and install basic utilities
RUN apt-get update && apt-get install -y \
    openssh-server \
    nano \
    sudo \
    pwgen \
    git \
    curl \
    wget \
    apt-transport-https \
    gnupg \
    lsb-release \
    ca-certificates \
    python3 \
    python3-pip \
    maven \
    zsh \
    acl \
    && apt-get clean

# Install Java (Temurin 17 and 11)
RUN wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add - && \
    echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" | tee /etc/apt/sources.list.d/adoptium.list && \
    apt-get update && \
    apt-get install -y temurin-17-jdk temurin-11-jdk && \
    apt-get clean

# Install GitHub CLI
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    wget -nv -O /tmp/githubcli-archive-keyring.gpg https://cli.github.com/packages/githubcli-archive-keyring.gpg && \
    cat /tmp/githubcli-archive-keyring.gpg | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null && \
    chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get update && \
    apt-get install -y jq gh && \
    apt-get clean

# Add Docker's official GPG key
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Set up the repository
RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker CLI
RUN apt-get update && apt-get install -y docker-ce-cli && apt-get clean

# Create devuser with sudo privileges
RUN useradd -m -s /bin/zsh devuser && \
    usermod -aG sudo devuser && \
    echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Configure Docker socket permissions for devuser
RUN groupadd docker
RUN usermod -aG docker devuser

# Copy setup scripts
COPY setup/ /tmp/setup/
RUN chmod +x /tmp/setup/*.sh

# Install Oh My Zsh and configure it
RUN su - devuser -c "/tmp/setup/oh-my-zsh-installer.sh"

# Install NVM and Node
RUN su - devuser -c 'curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash && \
    export NVM_DIR="$HOME/.nvm" && \
    source "$NVM_DIR/nvm.sh" && \
    nvm install --lts'

# Install npm-based CLIs as devuser
RUN su - devuser -c 'export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh" && \
    /tmp/setup/vercel-installer.sh && \
    /tmp/setup/cloudflare-installer.sh && \
    /tmp/setup/gemini-installer.sh && \
    /tmp/setup/jules-installer.sh'

# Append nvm initialization to .zshrc
RUN su - devuser -c 'echo "export NVM_DIR="$([ -z "${XDG_CONFIG_HOME-}" ] && printf %s "${HOME}/.nvm" || printf %s "${XDG_CONFIG_HOME}/nvm")"\n[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"" >> /home/devuser/.zshrc'
RUN su - devuser -c 'echo "nvm use --lts >> /dev/null" >> /home/devuser/.zshrc'

# Install other CLIs
RUN /tmp/setup/gcloud-installer.sh
RUN /tmp/setup/firebase-installer.sh

# Clean up setup scripts and cache
RUN rm -rf /tmp/setup/ && \
    apt-get autoremove -y && \
    apt-get autoclean && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /tmp/* && \
    rm -rf /var/tmp/*

# Configure the SSH service
RUN mkdir /var/run/sshd && \
    chmod 755 /var/run/sshd

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
