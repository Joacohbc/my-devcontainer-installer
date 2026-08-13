#!/usr/bin/env bash
# The entrypoint's hook into the project-skills installer.
#
# It exists so the installer can tell an automatic run from one the user asked
# for: the entrypoint invokes its start.d scripts with no arguments, so the
# distinction has to come from the caller rather than from the installer
# inspecting how it was launched. In manual mode this run is a no-op.
set -euo pipefail

exec "$HOME/.local/bin/install-skills" --auto
