#!/usr/bin/env bash
# when: build — runs as devuser inside `docker build`. Network and the modules
# above are available; the project, volumes and secrets are NOT. Write only to
# $HOME or system paths, never to /workspaces.
set -euo pipefail

uv tool install ruff
