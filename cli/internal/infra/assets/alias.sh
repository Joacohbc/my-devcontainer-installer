#!/bin/sh
# devcontainer-cli — default shell aliases and helper functions.
#
# Installed at build time to ~/.devcontainer_aliases.sh and sourced from
# ~/.zshrc, ~/.bashrc and ~/.profile. DO NOT EDIT INSIDE THE CONTAINER: this
# file is baked into the image and any change is lost when the container is
# recreated.
#
# For your OWN aliases use ~/.alias.sh instead — it lives in the shared-config
# volume, persists across every container, and is sourced AFTER this file so it
# always wins. It is generated from the CLI config: manage it on the host with
# `devcontainer-cli config alias set/unset`, then `devcontainer-cli config alias
# sync` to push it into every container.
#
# Every block below is guarded by `command -v`, so this single file is valid in
# every image variant regardless of which language modules were selected. Each
# guard is a full `if` block (never `cmd && alias …`) so a missing tool cannot
# leave the sourcing rc file with a non-zero exit status.

# ── kill_port <port> ─────────────────────────────────────────────────────────
# Kill whatever is listening on a TCP port. lsof ships in the base image.
kill_port() {
    if [ -z "$1" ]; then
        echo "usage: kill_port <port>" >&2
        return 2
    fi
    _kp_pids=$(lsof -t -i:"$1" 2>/dev/null)
    if [ -z "$_kp_pids" ]; then
        echo "kill_port: nothing is listening on port $1" >&2
        unset _kp_pids
        return 1
    fi
    echo "kill_port: killing $(echo "$_kp_pids" | tr '\n' ' ')on port $1"
    # shellcheck disable=SC2086 # intentional word splitting: one PID per line
    kill -9 $_kp_pids
    unset _kp_pids
}

# ── JavaScript / TypeScript: pnpm instead of npm ─────────────────────────────
if command -v pnpm >/dev/null 2>&1; then
    alias npm='pnpm'
    alias npx='pnpm dlx'
fi

# ── Python: uv instead of pip ────────────────────────────────────────────────
if command -v uv >/dev/null 2>&1; then
    # Let `uv pip` operate on the container's system Python without requiring a
    # virtualenv or an explicit --system on every call. The container IS the
    # isolation boundary, so a venv buys nothing here.
    export UV_SYSTEM_PYTHON=1
    pip() { uv pip "$@"; }
    pip3() { uv pip "$@"; }
fi

# ── AI agents: skip the permission prompts ───────────────────────────────────
# You are inside a disposable, isolated container, so the interactive approval
# prompts only add friction. Each alias is set only when its CLI is installed
# (command -v guard), so this one file is valid in every image variant.
#
# Escape hatch, always available even with the aliases active:
#     command claude ...     (or)     \claude ...
if command -v claude >/dev/null 2>&1; then
    alias claude='claude --dangerously-skip-permissions'
fi
if command -v codex >/dev/null 2>&1; then
    alias codex='codex --dangerously-bypass-approvals-and-sandbox'
fi
if command -v copilot >/dev/null 2>&1; then
    alias copilot='copilot --allow-all-tools'
fi
if command -v agy >/dev/null 2>&1; then
    alias agy='agy --dangerously-skip-permissions'
fi
