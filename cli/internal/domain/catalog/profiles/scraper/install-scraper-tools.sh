#!/usr/bin/env bash
# Scraper profile — the tools that are not catalog modules.
#
# This runs as devuser in a build layer, after every module, so uv, pnpm and the
# declared environment (PATH, UV_SYSTEM_PYTHON) are already in place: the block
# is emitted after renderEnvironmentBlocks and executed through a login shell,
# which sources ~/.dc-env.sh.
#
# Two install targets, on purpose:
#   uv tool  → a CLI in its own environment, shimmed into ~/.local/bin (owned by
#              devuser, already on PATH). Nothing to import, nothing to break.
#   uv pip   → a library a project imports, so it has to live in the system
#              interpreter. That needs root, hence sudo (passwordless, set up in
#              the base layer).
#
# No browser is downloaded: the chrome module already installed a system
# Chromium at /usr/bin/chromium, and both Playwright and Crawl4AI are pointed at
# it explicitly rather than fetching a second copy.
set -euo pipefail

CHROMIUM_BIN=/usr/bin/chromium

log() { echo "==> $*"; }

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: $1 is required by the scraper profile but is not installed" >&2
        exit 1
    fi
}

require uv
require pnpm

UV_BIN="$(command -v uv)"

# uv gained --break-system-packages after Ubuntu started marking its interpreter
# externally managed (PEP 668). Fall back so an older uv still installs.
uv_pip_system() {
    sudo "$UV_BIN" pip install --system --break-system-packages "$@" \
        || sudo "$UV_BIN" pip install --system "$@"
}

log "Installing Node scraping CLIs (pnpm, global)"
# agent-browser: browser automation CLI aimed at AI agents (needs node >= 24,
# which the nodejs module's default LTS satisfies).
# firecrawl-cli: the `firecrawl` command. The plain `firecrawl` npm package is
# the SDK and ships no binary, so it would install nothing usable here.
pnpm add -g agent-browser firecrawl-cli

log "Installing Python scraping CLIs (uv tool)"
# markitdown converts documents (pdf, docx, xlsx, …) to markdown; the [all]
# extra pulls the optional format backends.
uv tool install 'markitdown[all]'
# trafilatura strips a web page down to its main text, which is the step
# markitdown does not do.
uv tool install trafilatura
# yt-dlp pairs with the ffmpeg module for media downloads.
uv tool install yt-dlp

log "Installing Python scraping libraries (system interpreter)"
# These are imported from project code, so a tool-style isolated environment
# would make them invisible to it.
uv_pip_system crawl4ai playwright

log "Pointing Playwright and Crawl4AI at the system Chromium"
if [ ! -x "$CHROMIUM_BIN" ]; then
    echo "ERROR: expected the chrome module to provide $CHROMIUM_BIN" >&2
    exit 1
fi
# Read by Crawl4AI and by anything else that honours it; Playwright's Python API
# takes the path per-launch (executable_path=), which ~/CONTEXT.md documents.
sudo tee /etc/profile.d/scraper-chromium.sh >/dev/null <<EOF
export CHROME_BIN="$CHROMIUM_BIN"
export CRAWL4AI_BROWSER_PATH="$CHROMIUM_BIN"
EOF

log "Scraper toolchain installed:"
for tool in agent-browser firecrawl markitdown trafilatura yt-dlp; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "    $tool -> $(command -v "$tool")"
    else
        echo "    WARNING: $tool is not on PATH after install" >&2
    fi
done
python3 -c 'import crawl4ai, playwright; print("    crawl4ai + playwright importable from system python")'
