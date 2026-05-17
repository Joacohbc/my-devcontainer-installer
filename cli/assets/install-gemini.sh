#!/bin/bash
# Install Gemini CLI globally for devuser.
set -e

if [ -d "$HOME/.local/share/pnpm" ]; then
  export PNPM_HOME="$HOME/.local/share/pnpm"
  export PATH="$PNPM_HOME:$PATH"
fi
command -v pnpm >/dev/null 2>&1 || { echo "pnpm not found"; exit 1; }

echo "==> Installing @google/gemini-cli"
pnpm install -g @google/gemini-cli
gemini --version || true
