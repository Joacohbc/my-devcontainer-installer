#!/usr/bin/env bash
# An image's scripts live next to image.yml and follow the same `when` rules as
# a profile's. Prefer `when: build` here: whatever an image bakes in is shared by
# every project that starts from it.
set -euo pipefail

uv tool install ruff
uv tool install pre-commit
