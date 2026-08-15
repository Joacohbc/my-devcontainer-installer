#!/bin/bash
# Install context-mode for devuser and wire it into whatever AI agent CLIs are
# present in this container. Node-based (npm -g); the CLI itself (context-mode
# / ctx) works on any Node >= 22.5 or falls back gracefully otherwise.
#
# Only Claude Code and GitHub Copilot CLI get scripted wiring here: both ship
# a real plugin-manager subcommand this script can call unattended. Every
# other supported platform (Gemini CLI, VS Code/JetBrains Copilot, OpenCode,
# Cursor, ...) needs a hand-edited MCP/hook config file per its own docs —
# not something to script blindly against a config this installer does not
# own. context-mode/ctx are still on PATH afterwards for that manual setup.
set -e

echo "==> Installing context-mode"

export PATH="$HOME/.local/bin:$PATH"

npm install -g context-mode
context-mode --version || true

wired=0

# Claude Code — native plugin marketplace (the officially recommended path).
if command -v claude >/dev/null 2>&1; then
  echo "==> context-mode -> Claude Code (plugin marketplace)"
  { claude plugin marketplace add mksglu/context-mode && claude plugin install context-mode@context-mode; } \
    || echo "   (skipped: claude plugin install context-mode failed)"
  wired=1
fi

# GitHub Copilot CLI — its own plugin manager.
if command -v copilot >/dev/null 2>&1; then
  echo "==> context-mode -> GitHub Copilot CLI (plugin manager)"
  copilot plugin install mksglu/context-mode:configs/copilot-cli \
    || echo "   (skipped: copilot plugin install failed)"
  wired=1
fi

if [ "$wired" -eq 0 ]; then
  echo "==> No agent with a scriptable plugin install detected; context-mode/ctx are on PATH for manual setup"
fi
