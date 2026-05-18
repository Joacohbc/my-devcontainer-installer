#!/usr/bin/env sh
# devcontainer-cli installer (Linux / macOS)
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/install.sh | sh
#   curl -fsSL .../install.sh | VERSION=v1.0.0 INSTALL_DIR=$HOME/.local/share/devcontainer-cli sh
set -eu

REPO="${REPO:-Joacohbc/my-devcontainer-installer}"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/share/devcontainer-cli}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

err() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '==> %s\n' "$*"; }

uname_s=$(uname -s)
uname_m=$(uname -m)

case "$uname_s" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *) err "unsupported OS: $uname_s" ;;
esac

case "$uname_m" in
  x86_64|amd64) arch=x64 ;;
  arm64|aarch64)
    if [ "$os" = darwin ]; then
      arch=x64
      info "Apple Silicon detected — using darwin-x64 build (Rosetta 2 required)"
    else
      arch=arm64
    fi
    ;;
  *) err "unsupported arch: $uname_m" ;;
esac

target="${os}-${arch}"
asset="devcontainer-cli-${target}.tar.gz"

if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

command -v curl >/dev/null 2>&1 || err "curl required"
command -v tar >/dev/null 2>&1 || err "tar required"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "Downloading $asset ($VERSION)"
curl -fSL --progress-bar -o "$tmp/$asset" "$url" || err "download failed: $url"

info "Extracting"
tar -xzf "$tmp/$asset" -C "$tmp"

mkdir -p "$INSTALL_DIR" "$BIN_DIR"
rm -rf "$INSTALL_DIR"/assets "$INSTALL_DIR"/devcontainer-cli
cp -R "$tmp/devcontainer-cli-${target}/." "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/devcontainer-cli"

if [ "$os" = darwin ]; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/devcontainer-cli" >/dev/null 2>&1 || true
fi

ln -sf "$INSTALL_DIR/devcontainer-cli" "$BIN_DIR/devcontainer-cli"

info "Installed:"
printf '  binary : %s\n' "$BIN_DIR/devcontainer-cli"
printf '  assets : %s/assets\n' "$INSTALL_DIR"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) printf '\nAdd to PATH:\n  export PATH="%s:$PATH"\n' "$BIN_DIR" ;;
esac

info "Verify: devcontainer-cli --help"
