#!/bin/bash
# Install Caveman for devuser and wire its output-compression hooks/skills into
# every AI agent present in this container (Claude Code, Codex, Antigravity,
# GitHub Copilot). Node >= 18 is required (the per-platform installers are
# npx-based). Each platform is only wired if its CLI / config is present;
# failures never abort the rest.
set -e

echo "==> Installing Caveman"

export PATH="$HOME/.local/bin:$PATH"

REPO="JuliusBrussee/caveman"
wired=0

# Claude Code — native plugin marketplace.
if command -v claude >/dev/null 2>&1; then
  echo "==> Caveman -> Claude Code (plugin marketplace)"
  { claude plugin marketplace add "$REPO" && claude plugin install caveman@caveman; } \
    || echo "   (skipped: claude plugin install caveman@caveman failed)"
  wired=1
fi

# Codex — npx skills profile. Codex leaves no binary; ~/.codex is the marker.
# The trailing --yes is the `skills` CLI's own flag (distinct from npx's
# leading --yes, which only confirms downloading the `skills` package itself):
# without it, a repo with no --skill filter drops into an interactive
# multi-select picker, which hangs forever with no TTY attached — exactly
# where every start.d script runs.
if command -v codex >/dev/null 2>&1 || [ -d "$HOME/.codex" ]; then
  echo "==> Caveman -> Codex (npx skills --only codex)"
  npx --yes skills add "$REPO" --only codex --yes || echo "   (skipped: codex wiring failed)"
  wired=1
fi

# Antigravity — npx skills profile. Its binary on PATH is `agy`, not `antigravity`.
if command -v agy >/dev/null 2>&1; then
  echo "==> Caveman -> Antigravity (npx skills --only antigravity)"
  npx --yes skills add "$REPO" --only antigravity --yes || echo "   (skipped: antigravity wiring failed)"
  wired=1
fi

# GitHub Copilot — skills profile with always-on rule files.
if command -v copilot >/dev/null 2>&1; then
  echo "==> Caveman -> GitHub Copilot (npx skills --only copilot --with-init)"
  npx --yes skills add "$REPO" --only copilot --with-init --yes || echo "   (skipped: copilot wiring failed)"
  wired=1
fi

# Fallback: no known agent detected — run the generic auto-detecting installer.
if [ "$wired" -eq 0 ]; then
  echo "==> No known agent detected; running generic Caveman installer (auto-detect)"
  curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/install.sh | bash
fi
