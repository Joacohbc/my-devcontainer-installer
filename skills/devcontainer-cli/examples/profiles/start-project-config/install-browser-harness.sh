#!/usr/bin/env bash
# when: build — the slow, project-independent part: install once into the image.
set -euo pipefail

uv tool install --python 3.12 --upgrade "browser-harness[mcp]"
browser-harness telemetry disable 2>/dev/null || true
