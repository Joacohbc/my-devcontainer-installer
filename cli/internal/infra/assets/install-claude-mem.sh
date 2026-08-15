#!/bin/bash
# Install claude-mem for devuser and wire it into every AI agent CLI present
# in this container. Node-based (npx). `npx claude-mem install` (or the
# platform-specific --ide flag) is the only supported path: a plain
# `npm install -g claude-mem` only installs the SDK library, with no hooks or
# worker service registered. Each platform is only wired if its CLI / config
# is present; failures never abort the rest.
set -e

echo "==> Installing claude-mem"

export PATH="$HOME/.local/bin:$PATH"

wired=0

# Claude Code — native plugin marketplace (the richest integration: lifecycle
# hooks + local worker service + automatic context injection on restart).
if command -v claude >/dev/null 2>&1; then
  echo "==> claude-mem -> Claude Code (plugin marketplace)"
  { claude plugin marketplace add thedotmack/claude-mem && claude plugin install claude-mem; } \
    || echo "   (skipped: claude plugin install claude-mem failed)"
  wired=1
fi

# OpenCode — its own --ide installer.
if command -v opencode >/dev/null 2>&1; then
  echo "==> claude-mem -> OpenCode"
  npx --yes claude-mem install --ide opencode || echo "   (skipped: opencode wiring failed)"
  wired=1
fi

# Antigravity — its own --ide installer. Its binary on PATH is `agy`, not
# `antigravity`.
if command -v agy >/dev/null 2>&1; then
  echo "==> claude-mem -> Antigravity"
  npx --yes claude-mem install --ide antigravity || echo "   (skipped: antigravity wiring failed)"
  wired=1
fi

# Fallback: if no known agent was detected, let claude-mem auto-detect.
if [ "$wired" -eq 0 ]; then
  echo "==> No known agent detected; running generic claude-mem installer (auto-detect)"
  npx --yes claude-mem install || true
fi
