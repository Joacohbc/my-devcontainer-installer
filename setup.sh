#!/bin/bash

apt-get update
apt-get install -y git curl wget apt-transport-https gnupg

# INSTALL JAVA
JAVA_VERSIONS="temurin-17-jdk temurin-11-jdk"
wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add -
echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" | tee /etc/apt/sources.list.d/adoptium.list
apt-get update
apt-get install -y $JAVA_VERSIONS maven

# NVM
NVM_LATEST_RELEASE=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | grep tag_name | cut -d '"' -f 4)
curl -L -o- https://raw.githubusercontent.com/nvm-sh/nvm/$NVM_LATEST_RELEASE/install.sh | bash

# PYTHON
apt-get install -y python3
apt-get install -y python3-pip

# GOLANG
source /workspace/golang_utils.sh
install_golang