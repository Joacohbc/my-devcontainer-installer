#!/bin/bash
# Install Browser Harness (browser-use/browser-harness) for devuser,
# enable recording traces, and register its skills and interaction-skills
# at the project workspace level.
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

workspace_dir() {
  for candidate in /workspaces/*/ /workspace/*/ /workspace/; do
    [ -d "$candidate" ] && { echo "${candidate%/}"; return 0; }
  done
  return 1
}

# Install skills and interaction recipes into project workspace
if target="$(workspace_dir)"; then
  echo "==> Installing browser-harness skills into project workspace: $target"

  # Standard project-scoped agent skills (.agents/skills/browser-harness)
  mkdir -p "$target/.agents/skills/browser-harness"
  if command -v browser-harness >/dev/null 2>&1; then
    browser-harness skill > "$target/.agents/skills/browser-harness/SKILL.md" 2>/dev/null || true
  fi

  # Project-scoped Claude Code and Codex skills
  mkdir -p "$target/.claude/skills/browser-harness"
  mkdir -p "$target/.codex/skills/browser-harness"
  if [ -f "$target/.agents/skills/browser-harness/SKILL.md" ]; then
    cp "$target/.agents/skills/browser-harness/SKILL.md" "$target/.claude/skills/browser-harness/SKILL.md" 2>/dev/null || true
    cp "$target/.agents/skills/browser-harness/SKILL.md" "$target/.codex/skills/browser-harness/SKILL.md" 2>/dev/null || true
  fi

  # Fetch interaction-skills and agent-workspace from upstream GitHub repo
  echo "==> Fetching interaction-skills and agent-workspace from GitHub"
  TMP_DIR=$(mktemp -d /tmp/bh-fetch-XXXXXX)
  if curl -fsSL https://github.com/browser-use/browser-harness/archive/refs/heads/main.tar.gz | tar -xz -C "$TMP_DIR" 2>/dev/null; then
    SRC_DIR="$TMP_DIR/browser-harness-main"

    if [ -d "$SRC_DIR/interaction-skills" ]; then
      cp -r "$SRC_DIR/interaction-skills" "$target/.agents/skills/browser-harness/"
      cp -r "$SRC_DIR/interaction-skills" "$target/.claude/skills/browser-harness/" 2>/dev/null || true
      cp -r "$SRC_DIR/interaction-skills" "$target/.codex/skills/browser-harness/" 2>/dev/null || true
    fi

    # Place agent-workspace in project root if not already existing
    if [ -d "$SRC_DIR/agent-workspace" ] && [ ! -d "$target/agent-workspace" ]; then
      cp -r "$SRC_DIR/agent-workspace" "$target/"
    fi

    rm -rf "$TMP_DIR"
  fi
else
  echo "==> No workspace mount found; skipping project-level skills install"
fi
