#!/usr/bin/env bash
# when: service — started in the background on EVERY container start (no .done
# sentinel). Stay in the foreground (`exec`): the entrypoint already detaches
# it, and its output goes to ~/.post-script-state/<name>.log.
set -euo pipefail

cd "${DC_WORKSPACE_DIR:?run by the entrypoint, which sets DC_WORKSPACE_DIR}"
[ -f package.json ] || { echo "no package.json yet, nothing to serve"; exit 0; }
pnpm install --frozen-lockfile
exec pnpm dev --host 0.0.0.0
