#!/usr/bin/env bash
# when: manual — copied to ~/post-script/custom-reset-db.sh and never run on its own.
# It runs in the user's shell, so it may prompt; the DC_* variables are only
# set for build/start/service scripts, so it works from the current directory.
set -euo pipefail

db="${1:-$PWD/dev.db}"
read -r -p "Delete and recreate $db? [y/N] " answer
[ "$answer" = "y" ] || exit 0
rm -f "$db"
sqlite3 "$db" 'CREATE TABLE IF NOT EXISTS migrations (id TEXT PRIMARY KEY);'
echo "recreated $db"
