#!/bin/bash
# NVM (Node Version Manager) installation script
# This script installs the latest version of NVM

echo "Installing NVM (Node Version Manager)..."
NVM_LATEST_RELEASE=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | grep tag_name | cut -d '"' -f 4)
curl -L -o- https://raw.githubusercontent.com/nvm-sh/nvm/$NVM_LATEST_RELEASE/install.sh | bash
echo "NVM (Node.js) installation complete. Please source your .bashrc or .zshrc file, or open a new terminal."
