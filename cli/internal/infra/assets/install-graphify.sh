#!/bin/bash
# Install Graphify (graphifyy) for devuser and wire it into every AI agent
# present in this container (Claude Code, Codex, Antigravity, GitHub Copilot).
# Requires Python 3.10+. Prefers uv, falls back to pipx, then pip --user.
# Wiring is done in global/user scope (the project-scoped variant is
# intentionally avoided): it lands in devuser's home, persists, and never
# mutates the user's workspace repo.
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

graphify --version || true

# Wire the agent integration per platform (global scope). Each platform is only
# wired if its CLI / config is present in this container; failures never abort
# the rest.
wire() { # $1 = human label, $2 = graphify platform id
  echo "==> graphify install --platform $2 ($1)"
  graphify install --platform "$2" || echo "   (skipped: graphify install --platform $2 failed)"
}

wired=0
if command -v claude >/dev/null 2>&1; then wire "Claude Code" claude; wired=1; fi
# Codex is npx-based and leaves no binary; ~/.codex is the best-effort marker.
if command -v codex >/dev/null 2>&1 || [ -d "$HOME/.codex" ]; then wire "Codex" codex; wired=1; fi
if command -v antigravity >/dev/null 2>&1; then wire "Antigravity" antigravity; wired=1; fi
if command -v copilot >/dev/null 2>&1; then wire "GitHub Copilot" copilot; wired=1; fi

# Fallback: if no known agent was detected, let graphify auto-detect.
if [ "$wired" -eq 0 ]; then
  echo "==> No known agent detected; running graphify install (auto-detect)"
  graphify install || true
fi
