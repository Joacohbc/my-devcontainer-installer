# Use an Ubuntu base image
FROM ubuntu:22.04

# Update packages and install SSH, sudo, and other utilities
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
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

# Create devuser with sudo privileges
RUN useradd -m -s /bin/zsh devuser && \
    usermod -aG sudo devuser && \
    echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

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
    unzip \
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
