#!/bin/bash
# This script provides utility functions for installing and updating Go (Golang).
# It determines the system architecture, fetches the latest Go version (or a default),
# downloads and installs it, and configures the PATH.
# It also includes a function to update an existing Go installation by backing it up,
# removing it, and then performing a fresh installation.

# Architecture to download
OS_ARCH=$(uname -m)
case $OS_ARCH in
    x86_64)
        ARCH="linux-amd64"
        ;;
    aarch64)
        ARCH="linux-arm64"
        ;;
    armv6l|armv7l)
        ARCH="linux-armv6l"
        ;;
    i386|i686)
        ARCH="linux-386"
        ;;
    *)
        echo "Unsupported architecture: $OS_ARCH"
        exit 1
        ;;
esac

apt-get install curl -y

install_golang() {
    TEMP_DIR=$(mktemp -d)
    if [[ ! -d "$TEMP_DIR" ]]; then
        echo "Error creating temporary directory."
        return 1
    fi
    
    echo "Temporary directory: $TEMP_DIR"

    # Get the HTML of the downloads page
    GO_RELEASE_PAGE=$(curl -sS https://go.dev/dl/ || { echo "Error fetching downloads page."; exit 1; })

    # Extract the filename of the latest version
    GO_LATEST_VERSION=$(echo "$GO_RELEASE_PAGE" | grep -oE "go[0-9.]+\.[a-z0-9-]+\.tar\.gz" | grep $ARCH | head -n 1)

    # Check if a link was found
    DEFAULT_GO_VERSION="go1.20.$ARCH.tar.gz" #Default version
    if [[ -z "$GO_LATEST_VERSION" ]]; then
        echo "Download link for $ARCH not found. Installing default version."
        GO_LATEST_VERSION=$DEFAULT_GO_VERSION
    fi

    # Build the full URL
    GO_DOWNLOAD_URL="https://go.dev/dl/$GO_LATEST_VERSION"
    echo "Download URL: $GO_DOWNLOAD_URL"

    # Download the file to the temporary folder
    wget -qO "$TEMP_DIR/$GO_LATEST_VERSION" "$GO_DOWNLOAD_URL" #Download the file to the temporary directory

    if [[ $? -ne 0 ]]; then
        echo "Error downloading Golang binaries. Exiting..."
        rm -rf "$TEMP_DIR" #Delete the temporary directory in case of error
        return 1
    fi

    # Extract the file from the temporary folder
    tar -C /usr/local -xzf "$TEMP_DIR/$GO_LATEST_VERSION" #Extract the file from the temporary directory

    # Update the PATH variable
    touch /etc/profile
    echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile

    # If the user uses Zsh, also update the configuration file
    mkdir -p /etc/zsh
    touch /etc/zsh/zprofile
    echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/zsh/zprofile

    # Clean up the temporary folder
    rm -rf "$TEMP_DIR" #Delete the folder and all its contents.

    echo "Golang installed successfully."
    return 0
}

update_golang() {
    # Check if Go is installed
    if [[ ! -d "/usr/local/go" ]]; then
        echo "Go is not installed. Running clean installation..."
        install_golang
        return
    fi

    # Create backup
    BACKUP_DIR="/tmp/go_backup_$(date +%Y%m%d_%H%M%S)"
    echo "Creating backup in: $BACKUP_DIR"
    if ! cp -r /usr/local/go "$BACKUP_DIR"; then
        echo "Error creating backup. Aborting update..."
        return 1
    fi

    # Remove current installation
    echo "Removing current installation..."
    if ! rm -rf /usr/local/go; then
        echo "Error removing current installation. Restoring backup..."
        cp -r "$BACKUP_DIR" /usr/local/go
        rm -rf "$BACKUP_DIR"
        return 1
    fi

    # Install new version
    echo "Installing new version..."
    if ! install_golang; then
        echo "Error during installation. Restoring backup..."
        cp -r "$BACKUP_DIR" /usr/local/go
        rm -rf "$BACKUP_DIR"
        return 1
    fi

    # Validate installation
    if ! command -v /usr/local/go/bin/go >/dev/null; then
        echo "Go command not found after installation. Restoring backup..."
        cp -r "$BACKUP_DIR" /usr/local/go
        rm -rf "$BACKUP_DIR"
        return 1
    fi

    # Clean up backup on success
    echo "Update successful. Deleting backup..."
    rm -rf "$BACKUP_DIR"
    echo "Go updated successfully to: $(/usr/local/go/bin/go version)"
    return 0
}