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
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    acl

##
## DOCKER-OUTSIDE-DOCKER SETUP
## 

# Add Docker's official GPG key
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Set up the repository
RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker CLI
RUN apt-get update && apt-get install -y docker-ce-cli

# Create devuser with sudo privileges
RUN useradd -m -s /bin/zsh devuser && \
    usermod -aG sudo devuser && \
    echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Configure Docker socket permissions for devuser
RUN groupadd docker
RUN usermod -aG docker devuser

##
## ZSH TOOLS SETUP
## 

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

# Install GitHub CLI
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    wget -nv -O /tmp/githubcli-archive-keyring.gpg https://cli.github.com/packages/githubcli-archive-keyring.gpg && \
    cat /tmp/githubcli-archive-keyring.gpg | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null && \
    chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get update && \
    apt-get install -y jq gh && \
    apt-get clean

##
## Java, Python, SQLite, Go Setup
##

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

##
## DATABASE CLIENT TOOLS (MySQL, Redis, Postgres, Mongo 8.0)
##

# 1. Setup MongoDB 8.0 Repository for 'mongosh'
RUN curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | gpg --dearmor -o /etc/apt/keyrings/mongodb-server-8.0.gpg && \
    echo "deb [ arch=amd64,arm64 signed-by=/etc/apt/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/8.0 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-8.0.list

# 2. Install all DB clients
RUN apt-get update && apt-get install -y \
    postgresql-client \
    default-mysql-client \
    redis-tools \
    mongodb-mongosh

##
## NVM SETUP (Node Version Manager)
##

# Install NVM for devuser (last step to avoid issues with other installations)
RUN NVM_VERSION=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | jq -r .tag_name) && \
    su - devuser -c "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/${NVM_VERSION}/install.sh | bash"
RUN su - devuser -c 'echo "export NVM_DIR="$([ -z "${XDG_CONFIG_HOME-}" ] && printf %s "${HOME}/.nvm" || printf %s "${XDG_CONFIG_HOME}/nvm")"\n[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"" >> /home/devuser/.profile'
RUN su - devuser -c 'export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh" && nvm install --lts'
RUN su - devuser -c 'echo "nvm use --lts >> /dev/null" >> /home/devuser/.profile'

##
## CLEANUP & ENTRYPOINT
##

# Clean up
RUN apt-get autoremove -y && \
    apt-get autoclean && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /tmp/* && \
    rm -rf /var/tmp/*

# Configure the SSH service
RUN mkdir /var/run/sshd && \
    chmod 755 /var/run/sshd

# Expose port 22 for SSH
EXPOSE 22

# Copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set the entrypoint script
ENTRYPOINT ["/entrypoint.sh"]