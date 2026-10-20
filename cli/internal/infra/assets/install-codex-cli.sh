#!/bin/bash
# Put the Codex CLI on PATH.
#
# This used to run `npx @openai/codex` from the workspace and install nothing,
# which left `codex` missing from PATH: the `codex_yolo` alias (guarded by
# `command -v codex`) never existed, `get-devcontainer-context` reported Codex as
# absent, and every launch re-resolved the package from the registry. Installing
# it once, the same way install-ecc.sh does, fixes all three. Starting it is the
# user's call — run `codex` from the project directory.
set -e

echo "==> Installing Codex CLI (@openai/codex)"
export PATH="$HOME/.local/bin:$PATH"
if command -v pnpm >/dev/null 2>&1; then
  pnpm add -g @openai/codex
else
  npm install -g @openai/codex
fi
codex --version || true
