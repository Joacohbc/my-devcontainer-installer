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
asset="devcontainer-cli-${target}"

if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

command -v curl >/dev/null 2>&1 || err "curl required"

mkdir -p "$INSTALL_DIR" "$BIN_DIR"
binary_path="$INSTALL_DIR/devcontainer-cli"
tmp_path="${binary_path}.download"

info "Downloading $asset ($VERSION)"
curl -fSL --progress-bar -o "$tmp_path" "$url" || err "download failed: $url"

mv -f "$tmp_path" "$binary_path"
chmod +x "$binary_path"

if [ "$os" = darwin ]; then
  xattr -d com.apple.quarantine "$binary_path" >/dev/null 2>&1 || true
fi

ln -sf "$binary_path" "$BIN_DIR/devcontainer-cli"

info "Installed:"
printf '  binary : %s\n' "$BIN_DIR/devcontainer-cli"

# ---------------------------------------------------------------------------
# Shell completion + PATH setup
# ---------------------------------------------------------------------------
# Writes a marker-delimited managed block into the user's rc file(s). The same
# markers are used by uninstall.sh to remove it cleanly. Re-running the
# installer replaces the block instead of duplicating it. Opt out with
# SETUP_COMPLETION=0.
SETUP_COMPLETION="${SETUP_COMPLETION:-1}"
COMPLETION_DIR="$INSTALL_DIR/completions"
MARK_START="# >>> devcontainer-cli >>>"
MARK_END="# <<< devcontainer-cli <<<"

# Replace any existing managed block in $1 with the block read from stdin.
update_rc() {
  rc="$1"
  [ -f "$rc" ] || { mkdir -p "$(dirname "$rc")"; : > "$rc"; }
  block="$(cat)"
  tmp="${rc}.dcbak.$$"
  awk -v s="$MARK_START" -v e="$MARK_END" '
    $0==s {skip=1; next}
    $0==e {skip=0; next}
    skip!=1 {print}
  ' "$rc" > "$tmp"
  {
    printf '%s\n' "$MARK_START"
    printf '%s\n' "$block"
    printf '%s\n' "$MARK_END"
  } >> "$tmp"
  mv -f "$tmp" "$rc"
}

if [ "$SETUP_COMPLETION" = "1" ]; then
  mkdir -p "$COMPLETION_DIR"

  # zsh
  if command -v zsh >/dev/null 2>&1 || [ -f "${ZDOTDIR:-$HOME}/.zshrc" ]; then
    if "$binary_path" completion zsh > "$COMPLETION_DIR/_devcontainer-cli" 2>/dev/null; then
      update_rc "${ZDOTDIR:-$HOME}/.zshrc" <<EOF
export PATH="$BIN_DIR:\$PATH"
fpath=("$COMPLETION_DIR" \$fpath)
autoload -U compinit && compinit
EOF
      info "zsh completion: ${ZDOTDIR:-$HOME}/.zshrc"
    fi
  fi

  # bash
  if command -v bash >/dev/null 2>&1 || [ -f "$HOME/.bashrc" ]; then
    if "$binary_path" completion bash > "$COMPLETION_DIR/devcontainer-cli.bash" 2>/dev/null; then
      bashrc="$HOME/.bashrc"
      [ "$os" = darwin ] && [ -f "$HOME/.bash_profile" ] && bashrc="$HOME/.bash_profile"
      update_rc "$bashrc" <<EOF
export PATH="$BIN_DIR:\$PATH"
[ -f "$COMPLETION_DIR/devcontainer-cli.bash" ] && source "$COMPLETION_DIR/devcontainer-cli.bash"
EOF
      info "bash completion: $bashrc"
    fi
  fi

  info "Restart your shell (or 'exec \$SHELL') to enable PATH + completion."
else
  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) printf '\nAdd to PATH:\n  export PATH="%s:$PATH"\n' "$BIN_DIR" ;;
  esac
  printf '\nEnable completion manually:\n'
  printf '  devcontainer-cli completion zsh  > %s/_devcontainer-cli\n' "$COMPLETION_DIR"
  printf '  devcontainer-cli completion bash > %s/devcontainer-cli.bash\n' "$COMPLETION_DIR"
fi

info "Verify: devcontainer-cli --help"
info "Self-update: devcontainer-cli update"
