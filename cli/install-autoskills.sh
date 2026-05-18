#!/bin/bash
# Autoskills — no global install. Runs in project dir via npx.
# This script triggers it inside /workspace.
set -e

cd "${1:-/workspace}"
echo "==> Running: npx autoskills (cwd=$(pwd))"
npx autoskills
