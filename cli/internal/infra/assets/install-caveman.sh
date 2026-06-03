#!/bin/bash
# Install Caveman for devuser. Auto-detects supported AI agents and wires the
# output-compression hooks, statusline badge and caveman-shrink MCP middleware.
# The installer is Node-based (bin/install.js) and needs Node >= 18.
set -e

echo "==> Installing Caveman"

curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/install.sh | bash
