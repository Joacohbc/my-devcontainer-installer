#!/usr/bin/env bash
# Media-editor profile — the tools that are not catalog modules.
#
# This runs as devuser in a build layer, after every module, so pnpm and the
# declared environment (PATH) are already in place: the block is emitted
# after renderEnvironmentBlocks and executed through a login shell, which
# sources ~/.dc-env.sh.
#
# Two install targets:
#   apt   → command-line image/PDF/doc tools, which need root (devuser has
#           passwordless sudo, set up in the base layer).
#   pnpm  → the Remotion CLI, a Node toolchain. No second browser is
#           downloaded for it: the chrome module already installed a system
#           Chromium at /usr/bin/chromium, which Remotion's renderer can be
#           pointed at (see the remotion skill / ~/CONTEXT.md).
set -euo pipefail

log() { echo "==> $*"; }

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: $1 is required by the media-editor profile but is not installed" >&2
        exit 1
    fi
}

require pnpm

log "Installing image/PDF/doc CLI tools (apt)"
# imagemagick   -> create/convert/edit images (the `magick`/`convert` commands)
# poppler-utils -> pdftoppm/pdftotext/pdfinfo, read/rasterize PDFs
# ghostscript   -> gs, PDF processing/merging/compression
# qpdf          -> structural PDF transforms (split, merge, decrypt)
# pandoc        -> convert between markdown/html/docx and friends
sudo apt-get update
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    imagemagick poppler-utils ghostscript qpdf pandoc yt-dlp
sudo rm -rf /var/lib/apt/lists/*

log "Installing the Remotion CLI (pnpm, global)"
pnpm add -g @remotion/cli

log "Media toolchain installed:"
for tool in magick convert pdftoppm pdftotext gs qpdf pandoc ffmpeg remotion; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "    $tool -> $(command -v "$tool")"
    else
        echo "    WARNING: $tool is not on PATH after install" >&2
    fi
done

echo "==> Scaffold a Remotion project with: pnpm create video"
echo "==> Point Remotion's Chromium at the system browser: /usr/bin/chromium (see the remotion skill)"
