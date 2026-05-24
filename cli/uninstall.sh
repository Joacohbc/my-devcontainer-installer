#!/usr/bin/env sh
# devcontainer-cli uninstaller (Linux / macOS)
# Removes the binary, symlink, completion files, and the managed rc block that
# install.sh added. Honors the same INSTALL_DIR / BIN_DIR overrides.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/uninstall.sh | sh
#   sh uninstall.sh                       # local
#   KEEP_CONFIG=1 sh uninstall.sh         # leave rc blocks untouched
set -eu

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/share/devcontainer-cli}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
KEEP_CONFIG="${KEEP_CONFIG:-0}"

MARK_START="# >>> devcontainer-cli >>>"
MARK_END="# <<< devcontainer-cli <<<"

info() { printf '==> %s\n' "$*"; }

# Strip the marker-delimited managed block from $1 (no-op if absent).
strip_rc() {
  rc="$1"
  [ -f "$rc" ] || return 0
  grep -qF "$MARK_START" "$rc" 2>/dev/null || return 0
  tmp="${rc}.dcbak.$$"
  awk -v s="$MARK_START" -v e="$MARK_END" '
    $0==s {skip=1; next}
    $0==e {skip=0; next}
    skip!=1 {print}
  ' "$rc" > "$tmp"
  mv -f "$tmp" "$rc"
  info "Removed managed block: $rc"
}

# Symlink
link="$BIN_DIR/devcontainer-cli"
if [ -L "$link" ] || [ -e "$link" ]; then
  rm -f "$link"
  info "Removed symlink: $link"
fi

# Install dir (binary + completions)
if [ -d "$INSTALL_DIR" ]; then
  rm -rf "$INSTALL_DIR"
  info "Removed: $INSTALL_DIR"
fi

# rc blocks
if [ "$KEEP_CONFIG" = "1" ]; then
  info "KEEP_CONFIG=1 — leaving rc files untouched."
else
  strip_rc "${ZDOTDIR:-$HOME}/.zshrc"
  strip_rc "$HOME/.bashrc"
  strip_rc "$HOME/.bash_profile"
fi

info "Uninstalled devcontainer-cli."
info "Project files (.dc_<workspace>/, devcontainer.config.json) are left intact."
