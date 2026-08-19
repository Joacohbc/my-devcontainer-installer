#!/bin/bash
# Install Browser Harness (browser-use/browser-harness) for devuser,
# enable recording traces, and register its agent skill into every AI agent
# present in this container (Claude Code, Codex, Antigravity, GitHub Copilot, OpenCode).
# Requires Python 3.12+ (prefers uv tool, falls back to pipx, then pip --user).
set -e

echo "==> Installing Browser Harness"

export PATH="$HOME/.local/bin:$PATH"

if command -v uv >/dev/null 2>&1; then
  uv tool install --python 3.12 --upgrade --force browser-harness
elif command -v pipx >/dev/null 2>&1; then
  pipx install browser-harness
else
  pip install --user browser-harness
fi

browser-harness --version || true

# Enable session recording traces for video generation with ffmpeg.
echo "==> Enabling browser-harness recordings"
browser-harness recordings enable || true

# Wire the skill into detected agents.
if command -v browser-harness >/dev/null 2>&1; then
  # Claude Code
  if command -v claude >/dev/null 2>&1 || [ -d "$HOME/.claude" ]; then
    echo "==> browser-harness -> Claude Code (skill)"
    mkdir -p "$HOME/.claude/skills/browser-harness"
    browser-harness skill > "$HOME/.claude/skills/browser-harness/SKILL.md" 2>/dev/null || true
  fi

  # Codex
  if command -v codex >/dev/null 2>&1 || [ -d "$HOME/.codex" ]; then
    echo "==> browser-harness -> Codex (skill)"
    codex_home="${CODEX_HOME:-$HOME/.codex}"
    mkdir -p "$codex_home/skills/browser-harness"
    browser-harness skill > "$codex_home/skills/browser-harness/SKILL.md" 2>/dev/null || true
  fi

  # Antigravity
  if command -v agy >/dev/null 2>&1 || [ -d "$HOME/.gemini/antigravity" ]; then
    echo "==> browser-harness -> Antigravity (skill)"
    mkdir -p "$HOME/.gemini/antigravity/skills/browser-harness"
    browser-harness skill > "$HOME/.gemini/antigravity/skills/browser-harness/SKILL.md" 2>/dev/null || true
  fi

  # GitHub Copilot CLI
  if command -v copilot >/dev/null 2>&1 || [ -d "$HOME/.copilot" ]; then
    echo "==> browser-harness -> GitHub Copilot (skill)"
    mkdir -p "$HOME/.copilot/skills/browser-harness"
    browser-harness skill > "$HOME/.copilot/skills/browser-harness/SKILL.md" 2>/dev/null || true
  fi

  # OpenCode
  if command -v opencode >/dev/null 2>&1 || [ -d "$HOME/.config/opencode" ]; then
    echo "==> browser-harness -> OpenCode (skill)"
    mkdir -p "$HOME/.config/opencode/skills/browser-harness"
    browser-harness skill > "$HOME/.config/opencode/skills/browser-harness/SKILL.md" 2>/dev/null || true
  fi
fi
