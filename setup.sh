#!/bin/bash
# This script provides an interactive setup for installing common development tools:
# Java (Temurin JDK 11 & 17), NVM (Node Version Manager), Python, and Go.
# It prompts the user for each tool and installs it if confirmed.

# Check if running as root or with sudo
if [[ $EUID -ne 0 ]]; then
    echo "Error: This script must be run as root or with sudo privileges."
    echo "Please run: sudo $0"
    exit 1
fi

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