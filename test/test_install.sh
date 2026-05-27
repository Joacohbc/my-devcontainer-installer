#!/usr/bin/env sh
# devcontainer-cli local source installer (Linux / macOS)
# Builds the CLI from source and installs it locally.
# Usage:
#   ./install.sh
#   INSTALL_DIR=$HOME/.local/share/devcontainer-cli ./install.sh
set -eu

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/share/devcontainer-cli}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

err() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '==> %s\n' "$*"; }

# 1. Verify Go is installed
command -v go >/dev/null 2>&1 || err "Go is required to build from source. Please install Go (https://go.dev) and try again."

# Find script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Verify cli directory exists
if [ ! -d "$SCRIPT_DIR/cli" ]; then
  err "cli directory not found at $SCRIPT_DIR/cli"
fi

uname_s=$(uname -s)

# Create install and bin directories
mkdir -p "$INSTALL_DIR" "$BIN_DIR"
binary_path="$INSTALL_DIR/devcontainer-cli"

# 2. Build devcontainer-cli from source
info "Building devcontainer-cli from source..."
cd "$SCRIPT_DIR/cli"

# Get current version from git, default to dev
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Compile binary and place it in the target directory
go build -ldflags "-X main.version=$VERSION" -o "$binary_path" ./cmd/devcontainer-cli || err "go build failed"

chmod +x "$binary_path"

if [ "$uname_s" = "Darwin" ]; then
  xattr -d com.apple.quarantine "$binary_path" >/dev/null 2>&1 || true
fi

# Symlink to bin directory
ln -sf "$binary_path" "$BIN_DIR/devcontainer-cli"

info "Installed (built from source):"
printf '  binary : %s\n' "$BIN_DIR/devcontainer-cli"
printf '  version: %s\n' "$VERSION"

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

  # Determine proper bash rc file early for accurate messaging
  bashrc="$HOME/.bashrc"
  if [ "$uname_s" = "Darwin" ]; then
    if [ -f "$HOME/.bash_profile" ]; then
      bashrc="$HOME/.bash_profile"
    elif [ -f "$HOME/.profile" ]; then
      bashrc="$HOME/.profile"
    else
      bashrc="$HOME/.bash_profile"
    fi
  fi

  # zsh
  if command -v zsh >/dev/null 2>&1 || [ -f "${ZDOTDIR:-$HOME}/.zshrc" ]; then
    if "$binary_path" completion zsh > "$COMPLETION_DIR/_devcontainer-cli" 2>/dev/null; then
      update_rc "${ZDOTDIR:-$HOME}/.zshrc" <<EOF
export PATH="$BIN_DIR:\$PATH"
if [ -d "$COMPLETION_DIR" ]; then
  fpath=("$COMPLETION_DIR" \$fpath)
  if dummy=\$(type compdef) 2>/dev/null; then
    autoload -Uz _devcontainer-cli
    compdef _devcontainer-cli devcontainer-cli
  else
    autoload -Uz compinit && compinit
  fi
fi
EOF
      info "zsh completion: ${ZDOTDIR:-$HOME}/.zshrc"
    fi
  fi

  # bash
  if command -v bash >/dev/null 2>&1 || [ -f "$HOME/.bashrc" ] || [ -f "$HOME/.bash_profile" ]; then
    if "$binary_path" completion bash > "$COMPLETION_DIR/devcontainer-cli.bash" 2>/dev/null; then
      update_rc "$bashrc" <<EOF
export PATH="$BIN_DIR:\$PATH"
[ -f "$COMPLETION_DIR/devcontainer-cli.bash" ] && source "$COMPLETION_DIR/devcontainer-cli.bash"
EOF
      info "bash completion: $bashrc"
    fi
  fi

  # fish
  if command -v fish >/dev/null 2>&1 || [ -d "${XDG_CONFIG_HOME:-$HOME/.config}/fish" ]; then
    FISH_COMP_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"
    mkdir -p "$FISH_COMP_DIR"
    if "$binary_path" completion fish > "$FISH_COMP_DIR/devcontainer-cli.fish" 2>/dev/null; then
      update_rc "${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" <<EOF
if not contains "$BIN_DIR" \$PATH
  set -gx PATH "$BIN_DIR" \$PATH
fi
EOF
      info "fish completion: $FISH_COMP_DIR/devcontainer-cli.fish"
    fi
  fi

  # Tailor exit instructions to user's active shell ($SHELL)
  active_shell=""
  shell_env="${SHELL:-}"
  case "$shell_env" in
    *zsh*)  active_shell="zsh" ;;
    *bash*) active_shell="bash" ;;
    *fish*) active_shell="fish" ;;
  esac

  if [ "$active_shell" = "zsh" ]; then
    info "Restart your shell or run 'source ${ZDOTDIR:-$HOME}/.zshrc' to enable PATH + completion."
  elif [ "$active_shell" = "bash" ]; then
    info "Restart your shell or run 'source $bashrc' to enable PATH + completion."
  elif [ "$active_shell" = "fish" ]; then
    info "Restart your shell or run 'source ${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish' to enable PATH + completion."
  else
    info "Restart your shell (or 'exec \$SHELL') to enable PATH + completion."
  fi
else
  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) printf '\nAdd to PATH:\n  export PATH="%s:$PATH"\n' "$BIN_DIR" ;;
  esac
  printf '\nEnable completion manually:\n'
  printf '  devcontainer-cli completion zsh  > %s/_devcontainer-cli\n' "$COMPLETION_DIR"
  printf '  devcontainer-cli completion bash > %s/devcontainer-cli.bash\n' "$COMPLETION_DIR"
  printf '  devcontainer-cli completion fish > %s/devcontainer-cli.fish\n' "${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"
fi

info "Verify: devcontainer-cli --help"
