#!/bin/bash

sudo apt-get update
sudo apt-get install -y git curl wget apt-transport-https gnupg

# INSTALL JAVA
JAVA_VERSIONS="temurin-17-jdk temurin-11-jdk"
wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | sudo apt-key add -
echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" | sudo tee /etc/apt/sources.list.d/adoptium.list
sudo apt-get update
sudo apt-get install -y $JAVA_VERSIONS

# GOLANG

# https://golang.org/dl/go1.$GO_VERSION
GO_VERSION="go1.23.0.linux-arm64.tar.gz"

wget "https://golang.org/dl/$GO_VERSION"
sudo tar -C /usr/local -xzf $GO_VERSION
sudo echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/profile
sudo echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/zsh/zprofile

# NVM
NVM_LATEST_RELEASE=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | grep tag_name | cut -d '"' -f 4)
curl -L -o- https://raw.githubusercontent.com/nvm-sh/nvm/$NVM_LATEST_RELEASE/install.sh | bash

# PYTHON
sudo apt-get install -y python3
sudo apt-get install -y python3-pip