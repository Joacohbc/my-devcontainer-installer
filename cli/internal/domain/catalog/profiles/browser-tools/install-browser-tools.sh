#!/usr/bin/env bash
# Browser & Tools profile — installs agent-browser and automation dependencies.
#
# This runs as devuser in a build layer, after every module, so pnpm, uv, and
# Chromium are already in place: the block is emitted after renderEnvironmentBlocks
# and executed through a login shell, which sources ~/.dc-env.sh.
set -euo pipefail

CHROMIUM_BIN=/usr/bin/chromium

log() { echo "==> $*"; }

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: $1 is required by the browser-tools profile but is not installed" >&2
        exit 1
    fi
}

require pnpm
require uv

UV_BIN="$(command -v uv)"

uv_pip_system() {
    sudo "$UV_BIN" pip install --system --break-system-packages "$@" \
        || sudo "$UV_BIN" pip install --system "$@"
}

log "Installing agent-browser CLI and Node Playwright (pnpm, global)"
pnpm add -g agent-browser playwright

log "Installing Python Playwright bindings (system interpreter)"
uv_pip_system playwright

log "Configuring system Chromium environment variables"
if [ ! -x "$CHROMIUM_BIN" ]; then
    echo "ERROR: expected the chrome module to provide $CHROMIUM_BIN" >&2
    exit 1
fi

sudo tee /etc/profile.d/browser-tools.sh >/dev/null <<EOF
export CHROME_BIN="$CHROMIUM_BIN"
export CRAWL4AI_BROWSER_PATH="$CHROMIUM_BIN"
EOF

log "Browser & Tools toolchain installed:"
for tool in agent-browser playwright; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "    $tool -> $(command -v "$tool")"
    else
        echo "    WARNING: $tool is not on PATH after install" >&2
    fi
done
python3 -c 'import playwright; print("    playwright importable from system python")'
