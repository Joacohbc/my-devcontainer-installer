#!/bin/bash
# Put ECC (Everything Claude Code) on PATH. Nothing else.
#
# This installs the CLIs and deliberately stops there: it does NOT run
# `ecc --target <t>`. Every ECC install target writes into the project
# (`./.claude/`, `./.agent/`) or into the home, i.e. into files the user owns
# and usually tracks in git — and the workspace is a bind mount, so those writes
# land in their real checkout on the host. Doing that unattended on first boot
# means a container start silently rewrites the user's repository with a module
# bundle nobody chose. Once the tools are here the install is one command away,
# and choosing to run it is theirs.
#
# From the workspace root:
#
#   ecc --target claude-project --profile developer   # ./.claude/
#   ecc --target antigravity    --profile developer   # ./.agent/
#
# `--dry-run` shows the plan first. Prefer a project target: the home ones
# (`claude`, `codex`, ...) write to ~/.claude / ~/.codex, which are symlinks
# into the shared config volume, so they would leak one project's ECC into every
# other container mounting it.
set -e

echo "==> Installing ECC CLIs"

export PATH="$HOME/.local/bin:$PATH"

# The installer runtime lives inside the ecc-universal package, so this global
# install is what provides the content too, not merely the `ecc` command. No
# `|| true`: a silent no-op here is what made ECC look installed when it was not.
if command -v pnpm >/dev/null 2>&1; then
  pnpm add -g ecc-universal ecc-agentshield
else
  npm install -g ecc-universal ecc-agentshield
fi

echo "==> ECC installed. Commands: 'ecc' (installer + subcommands) and 'agentshield'."
echo "    Nothing was written into the workspace. To lay ECC down in this project:"
echo "      ecc --target claude-project --profile developer"
echo "    Add --dry-run first to see the plan."
