#!/bin/bash
# Codex CLI — no global install. Runs in project dir via npx.
# This script triggers it inside the project mount.
set -e

# Resolve the project mount the way the entrypoint does. There is no bare
# /workspace to fall back on in current images: each project gets its own
# /workspaces/<name> (aliased /workspace/<name>), so that a path-keyed agent
# history never merges two projects. Older images did mount at /workspace.
target="$1"
if [ -z "$target" ]; then
    target=/workspace
    for _ws in /workspaces/*; do
        [ -d "$_ws" ] || continue
        target="$_ws"
        break
    done
fi

cd "$target"
echo "==> Running: npx @openai/codex (cwd=$(pwd))"
npx @openai/codex
