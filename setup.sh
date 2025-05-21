#!/bin/bash
# This script provides an interactive setup for installing common development tools:
# Java (Temurin JDK 11 & 17), NVM (Node Version Manager), Python, and Go.
# It prompts the user for each tool and installs it if confirmed.

apt-get update
apt-get install -y git curl wget apt-transport-https gnupg

# INSTALL JAVA
read -p "Do you want to install Java? (yes/no): " install_java
if [[ "$install_java" == "yes" ]]; then
    JAVA_VERSIONS="temurin-17-jdk temurin-11-jdk"
    wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add -
    echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" | tee /etc/apt/sources.list.d/adoptium.list
    apt-get update
    apt-get install -y $JAVA_VERSIONS maven
    echo "Java installation complete."
else
    echo "Skipping Java installation."
fi

# NVM
read -p "Do you want to install NVM (Node.js)? (yes/no): " install_nvm
if [[ "$install_nvm" == "yes" ]]; then
    NVM_LATEST_RELEASE=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | grep tag_name | cut -d '"' -f 4)
    curl -L -o- https://raw.githubusercontent.com/nvm-sh/nvm/$NVM_LATEST_RELEASE/install.sh | bash
    echo "NVM (Node.js) installation complete. Please source your .bashrc or .zshrc file, or open a new terminal."
else
    echo "Skipping NVM (Node.js) installation."
fi

# PYTHON
read -p "Do you want to install Python? (yes/no): " install_python
if [[ "$install_python" == "yes" ]]; then
    apt-get install -y python3
    apt-get install -y python3-pip
    echo "Python installation complete."
else
    echo "Skipping Python installation."
fi

# SQLITE
read -p "Do you want to install SQLite? (yes/no): " install_sqlite
if [[ "$install_sqlite" == "yes" ]]; then
    apt-get install -y sqlite3
    echo "SQLite installation complete."
else
    echo "Skipping SQLite installation."
fi

# GOLANG
read -p "Do you want to install Go? (yes/no): " install_golang
if [[ "$install_golang" == "yes" ]]; then
    source /workspace/golang_utils.sh
    install_golang
    echo "Go installation complete."
else
    echo "Skipping Go installation."
fi