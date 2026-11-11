#!/usr/bin/env bash
# when: start — runs once per container, after the project is mounted. The
# entrypoint exports DC_WORKSPACE_DIR (the project dir, /workspaces/<name>).
# Images built before that contract existed do not set it, hence the fallback.
# Keep it idempotent: a recreated container runs it again.
set -euo pipefail

project_dir="${DC_WORKSPACE_DIR:-}"
if [ -z "$project_dir" ]; then
  for dir in /workspaces/*/; do
    [ -d "$dir" ] && project_dir="${dir%/}" && break
  done
fi
if [ -z "$project_dir" ] || [ ! -w "$project_dir" ]; then
  echo "setup-browser-harness-mcp: no writable project dir" >&2
  exit 1
fi

write_config() {
  mkdir -p "$(dirname "$1")"
  [ -e "$1" ] && return 0
  cat > "$1"
}

stdio_server='{
  "mcpServers": {
    "browser-harness": {
      "command": "uvx",
      "args": ["--from", "browser-harness[mcp]", "browser-harness-mcp"]
    }
  }
}'

printf '%s\n' "$stdio_server" | write_config "$project_dir/.mcp.json"
printf '%s\n' "$stdio_server" | write_config "$project_dir/.agent/mcp.json"
write_config "$project_dir/.cursor/mcp.json" <<'JSON'
{
  "mcpServers": {
    "browser-harness": {
      "type": "stdio",
      "command": "uvx",
      "args": ["--from", "browser-harness[mcp]", "browser-harness-mcp"]
    }
  }
}
JSON
