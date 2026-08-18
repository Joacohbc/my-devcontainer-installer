#!/bin/bash
# Install ECC (Everything Claude Code) project-scoped into the workspace.
#
# ECC is not a skills pack. The repository ships agents, rules, commands, hooks
# and MCP configs alongside the skills, and its own installer is the ONLY
# channel that lays all of them down: `npx skills add affaan-m/ECC` copies the
# skills and nothing else, and `claude plugin install ecc@ecc` cannot ship the
# rules at all (upstream: "Claude Code plugins cannot distribute rules") while
# writing into ~/.claude, which in this image is a symlink into the shared
# config volume — so it would leak one project's ECC into every other container.
#
# Everything below therefore goes through `ecc --target <t>` with a project
# target, and every target used writes inside the workspace mount.
set -e

echo "==> Installing ECC for workspace"

export PATH="$HOME/.local/bin:$PATH"

# Which module bundle ECC installs. Declared by the module as an image ENV, see
# dockerfile.EccModule; 'developer' is upstream's default engineering profile.
PROFILE="${DEVCONTAINER_ECC_PROFILE:-developer}"

# The installer runtime lives inside the ecc-universal package, so this global
# install is what provides the content, not merely the `ecc` command. No
# `|| true`: without it every step below is a no-op, and a silent no-op here is
# what made ECC look installed when it was not.
if command -v pnpm >/dev/null 2>&1; then
  pnpm add -g ecc-universal ecc-agentshield
else
  npm install -g ecc-universal ecc-agentshield
fi

workspace_dir() {
  for candidate in /workspaces/*/ /workspace/*/ /workspace/; do
    [ -d "$candidate" ] && { echo "${candidate%/}"; return 0; }
  done
  return 1
}

if ! target="$(workspace_dir)"; then
  echo "ERROR: no workspace mount found; cannot install workspace-scoped ECC" >&2
  exit 1
fi

cd "$target"
echo "==> Installing ECC inside workspace: $target (profile: $PROFILE)"

run_ecc() {
  echo "==> ECC -> $2 (ecc --target $1 --profile $PROFILE)"
  ecc --target "$1" --profile "$PROFILE" || echo "   (skipped: ecc --target $1 failed)"
}

# Claude Code — ./.claude/ with agents, rules, commands, hooks, skills, scripts.
# This is the default target and the one ECC is built around, so it runs even
# when the Claude CLI itself is not in the image: the project config is what a
# later `claude` invocation (or a mounted host CLI) reads.
run_ecc claude-project "Claude Code"

# Antigravity — ./.agent/ with rules, workflows, skills and agents.
if command -v agy >/dev/null 2>&1; then
  run_ecc antigravity "Antigravity"
fi

# OpenCode — ~/.opencode with commands, hooks and config. That is the home, not
# the workspace, but ~/.opencode is NOT one of types.SharedConfigEntries, so it
# stays inside this container and is rebuilt on the next one. Upstream ships a
# dedicated 'opencode' profile for this target.
if command -v opencode >/dev/null 2>&1; then
  echo "==> ECC -> OpenCode (ecc --target opencode --profile opencode)"
  ecc --target opencode --profile opencode || echo "   (skipped: ecc --target opencode failed)"
fi

# Codex is deliberately NOT wired: its only ECC target writes to ~/.codex, which
# IS a shared-config entry — a symlink into the shared volume — so it would
# install one project's ECC into every container mounting it. Run it by hand if
# that is what you want. There is no GitHub Copilot target upstream at all.
echo "==> Codex: skipped on purpose (its ECC target writes to the shared ~/.codex volume)."
echo "    Run 'ecc --target codex --profile $PROFILE' yourself if you want it anyway."
