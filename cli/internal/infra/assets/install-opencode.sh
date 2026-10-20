#!/bin/bash
# Install OpenCode via official installer.
set -e

echo "==> Installing OpenCode (opencode.ai)"
curl -fsSL https://opencode.ai/install | bash

# ~/.opencode/bin is declared as image environment by the opencode module
# (ProvidesEnv), so every process sees it — not only shells that read
# ~/.profile. The export below only covers the version check that follows.
export PATH="$HOME/.opencode/bin:$PATH"
opencode --version || true
