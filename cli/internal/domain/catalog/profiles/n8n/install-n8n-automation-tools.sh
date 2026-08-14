#!/usr/bin/env bash
# n8n profile — the tools that are not catalog modules.
#
# This runs as devuser in a build layer, after every module, so pnpm and the
# declared environment (PATH) are already in place: the block is emitted
# after renderEnvironmentBlocks and executed through a login shell, which
# sources ~/.dc-env.sh.
#
# n8n is installed inside this devcontainer (not as a separate compose
# service): `n8n start` runs it directly, with the editor UI on :5678
# (published above). No browser is downloaded for Playwright: the chrome
# module already installed a system Chromium at /usr/bin/chromium, which
# Playwright's Node API is pointed at per-launch (executablePath).
set -euo pipefail

log() { echo "==> $*"; }

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: $1 is required by the n8n profile but is not installed" >&2
        exit 1
    fi
}

require pnpm

log "Installing n8n and Playwright (pnpm, global)"
pnpm add -g n8n playwright

log "n8n / automation toolchain installed:"
for tool in n8n playwright; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "    $tool -> $(command -v "$tool")"
    else
        echo "    WARNING: $tool is not on PATH after install" >&2
    fi
done

echo "==> Start n8n with: n8n start (editor UI published on :5678)"
echo "==> By default n8n workflows live in ~/.n8n, which does not survive a rebuild;"
echo "    set N8N_USER_FOLDER into the bind-mounted workspace to persist them."
echo "==> Point Playwright at the system Chromium: executablePath: '/usr/bin/chromium'"
