#!/bin/bash
# Install Claude Code globally for devuser using native installer.
set -e

CLAUDE_JSON="$HOME/.claude.json"

claude_missing() {
    [ ! -x "$HOME/.local/bin/claude" ] && ! command -v claude >/dev/null 2>&1
}

# The shared-config volume may have created ~/.claude.json as an empty file;
# seed it with an empty JSON object so Claude Code can parse it.
if [ -e "$CLAUDE_JSON" ] && [ ! -s "$CLAUDE_JSON" ]; then
    echo '{}' > "$CLAUDE_JSON"
fi

# Workaround for anthropics/claude-code#20345: a ~/.claude.json carrying
# "installMethod": "native" from another container (persisted by the shared
# config volume) makes the installer skip copying the binary while still
# reporting success. Drop the stale marker so the installer runs fresh.
if claude_missing && [ -s "$CLAUDE_JSON" ] && jq -e '.installMethod' "$CLAUDE_JSON" >/dev/null 2>&1; then
    echo "==> Removing stale installMethod from ~/.claude.json (binary not present in this container)"
    tmp=$(mktemp)
    jq 'del(.installMethod)' "$CLAUDE_JSON" > "$tmp"
    # Write through the path: ~/.claude.json may be a symlink into the shared
    # config volume and must stay one (mv would replace the link itself).
    cat "$tmp" > "$CLAUDE_JSON"
    rm -f "$tmp"
fi

echo "==> Installing Claude Code (native installer)"
curl -fsSL https://claude.ai/install.sh | bash

if claude_missing; then
    echo "ERROR: Claude Code binary not found after install (anthropics/claude-code#20345)" >&2
    exit 1
fi
echo "==> Claude Code installed: $("$HOME/.local/bin/claude" --version 2>/dev/null || claude --version)"
