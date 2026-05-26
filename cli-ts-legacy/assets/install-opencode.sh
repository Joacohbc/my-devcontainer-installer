#!/bin/bash
# Install OpenCode via official installer.
set -e

echo "==> Installing OpenCode (opencode.ai)"
curl -fsSL https://opencode.ai/install | bash

if ! grep -q 'opencode/bin' "$HOME/.profile" 2>/dev/null; then
  echo 'export PATH="$HOME/.opencode/bin:$PATH"' >> "$HOME/.profile"
fi

export PATH="$HOME/.opencode/bin:$PATH"
opencode --version || true
