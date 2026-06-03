#!/bin/bash
# Install Graphify (graphifyy) for devuser. Builds knowledge graphs from a
# codebase and exposes the `graphify` CLI / `/graphify` agent command.
# Requires Python 3.10+. Prefers uv, falls back to pipx, then pip --user.
set -e

echo "==> Installing Graphify (graphifyy)"

export PATH="$HOME/.local/bin:$PATH"

if command -v uv >/dev/null 2>&1; then
  uv tool install graphifyy
elif command -v pipx >/dev/null 2>&1; then
  pipx install graphifyy
else
  pip install --user graphifyy
fi

# Wire the agent integration (Claude Code, Codex, OpenCode, ...).
graphify install || true
graphify --version || true
