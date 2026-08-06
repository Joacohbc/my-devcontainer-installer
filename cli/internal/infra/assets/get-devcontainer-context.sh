#!/bin/bash
# get-devcontainer-context — print everything an AI agent needs to know about
# THIS container: the static conventions from ~/CONTEXT.md plus a live inventory
# of the runtime, the installed tools and the reachable services.
#
# Installed at build time to ~/.local/bin/get-devcontainer-context (already on
# PATH via the base module's .local_bin_init.sh). Also copyable into older
# containers with `devcontainer-cli copy --asset get-devcontainer-context`.
#
#   get-devcontainer-context           human/markdown output (default)
#   get-devcontainer-context --json    structured output, easier to parse
#   get-devcontainer-context --tools   only the installed-tool inventory

set -u

JSON=0
TOOLS_ONLY=0
for arg in "$@"; do
    case "$arg" in
        --json) JSON=1 ;;
        --tools) TOOLS_ONLY=1 ;;
        -h|--help)
            sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "get-devcontainer-context: unknown option '$arg'" >&2
            exit 2
            ;;
    esac
done

# ── Runtime facts ────────────────────────────────────────────────────────────

# Resolve the workspace the same way the entrypoint does: a per-project mount
# under /workspaces/<name>, with /workspace as a stable alias for older layouts.
WORKSPACE_DIR=/workspace
for _ws in /workspaces/*; do
    [ -d "$_ws" ] || continue
    WORKSPACE_DIR="$_ws"
    break
done

DOCKER_SOCK=no
[ -S /var/run/docker.sock ] && DOCKER_SOCK=yes

SHARED_CONFIG=no
[ -d /mnt/shared-config ] && SHARED_CONFIG=yes

# ── Tool inventory ───────────────────────────────────────────────────────────
# "<command>|<version flag>|<label>". Only tools actually present are reported:
# a list of absent tools is noise for an agent deciding what to run.
TOOLS='
node|--version|Node.js
pnpm|--version|pnpm
npm|--version|npm
yarn|--version|Yarn
bun|--version|Bun
python3|--version|Python
uv|--version|uv
pip|--version|pip
go|version|Go
java|-version|Java
php|--version|PHP
rustc|--version|Rust
cargo|--version|Cargo
gcc|--version|GCC
sqlite3|--version|SQLite
psql|--version|PostgreSQL client
redis-cli|--version|Redis client
mongosh|--version|MongoDB shell
git|--version|Git
gh|--version|GitHub CLI
docker|--version|Docker CLI
zellij|--version|Zellij
micro|--version|micro editor
jq|--version|jq
lsof|-v|lsof
ffmpeg|-version|ffmpeg
chromium|--version|Chromium
ngrok|--version|ngrok
cloudflared|--version|cloudflared
claude|--version|Claude Code
codex|--version|Codex CLI
copilot|--version|GitHub Copilot CLI
agy|--version|Antigravity CLI
opencode|--version|OpenCode
graphify|--version|Graphify
caveman|--version|Caveman
'

# first_line trims a version banner down to one usable line, skipping the JVM's
# "Picked up JAVA_TOOL_OPTIONS/_JAVA_OPTIONS" notices, which java prints before
# its own version.
first_line() {
    grep -v '^Picked up ' | head -n 1 | tr -d '\r' | cut -c1-120
}

# lsof -v prints a multi-line banner on stderr whose first line is only a header
# ("lsof version information:") — the number lives on the "revision:" line, so
# first_line alone would report nothing usable. Fall back to the raw banner if a
# future lsof drops that line.
lsof_version() {
    local banner revision
    banner=$(lsof -v 2>&1 </dev/null)
    revision=$(printf '%s\n' "$banner" | sed -n 's/^[[:space:]]*revision:[[:space:]]*//p' | first_line)
    if [ -n "$revision" ]; then
        printf '%s\n' "$revision"
    else
        printf '%s\n' "$banner" | first_line
    fi
}

# Every probe reads stdin from /dev/null: a wrapper that asks something ("Install
# the CLI? [y/N]") would otherwise hang the whole inventory waiting for an answer.
tool_version() { # $1 = command, $2 = version flag
    case "$1" in
        lsof) lsof_version ;;
        # java prints its banner on stderr, not stdout.
        java) "$1" "$2" 2>&1 </dev/null | first_line ;;
        *) "$1" "$2" 2>/dev/null </dev/null | first_line ;;
    esac
}

# ── Reachable sibling services ───────────────────────────────────────────────
# "<hostname>|<port>|<label>"
SERVICES='
postgres|5432|PostgreSQL
redis|6379|Redis
mongo|27017|MongoDB
'

service_reachable() { getent hosts "$1" >/dev/null 2>&1; }

# ── Output ───────────────────────────────────────────────────────────────────

json_escape() { sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'; }

emit_json() {
    printf '{\n'
    printf '  "workspace": "%s",\n' "$WORKSPACE_DIR"
    printf '  "hostname": "%s",\n' "$(hostname)"
    printf '  "user": "%s",\n' "$(id -un)"
    printf '  "uid": %s,\n' "$(id -u)"
    printf '  "gid": %s,\n' "$(id -g)"
    printf '  "dockerSocket": %s,\n' "$([ "$DOCKER_SOCK" = yes ] && echo true || echo false)"
    printf '  "sharedConfig": %s,\n' "$([ "$SHARED_CONFIG" = yes ] && echo true || echo false)"
    printf '  "contextFile": "%s",\n' "$HOME/CONTEXT.md"

    printf '  "tools": {'
    sep=''
    while IFS='|' read -r cmd flag label; do
        [ -z "$cmd" ] && continue
        command -v "$cmd" >/dev/null 2>&1 || continue
        ver=$(tool_version "$cmd" "$flag" | json_escape)
        printf '%s\n    "%s": {"label": "%s", "version": "%s"}' "$sep" "$cmd" "$label" "$ver"
        sep=','
    done <<< "$TOOLS"
    printf '\n  },\n'

    printf '  "services": {'
    sep=''
    while IFS='|' read -r host port label; do
        [ -z "$host" ] && continue
        service_reachable "$host" || continue
        printf '%s\n    "%s": {"label": "%s", "port": %s}' "$sep" "$host" "$label" "$port"
        sep=','
    done <<< "$SERVICES"
    printf '\n  }\n'
    printf '}\n'
}

emit_static_context() {
    if [ -r "$HOME/CONTEXT.md" ]; then
        cat "$HOME/CONTEXT.md"
    else
        echo "# Container context"
        echo
        echo "(~/CONTEXT.md is missing — this image predates it.)"
    fi
    echo
    echo "---"
    echo
    echo "# This container, right now"
    echo
    echo "- Hostname: $(hostname)"
    echo "- Workspace: $WORKSPACE_DIR"
    echo "- User: $(id -un) (uid $(id -u), gid $(id -g))"
    echo "- Docker socket mounted: $DOCKER_SOCK"
    echo "- Shared config volume mounted: $SHARED_CONFIG"
    echo
}

emit_tools() {
    local found=0
    echo "## Installed tools"
    echo
    while IFS='|' read -r cmd flag label; do
        [ -z "$cmd" ] && continue
        command -v "$cmd" >/dev/null 2>&1 || continue
        printf -- '- %-22s %s\n' "$cmd" "$(tool_version "$cmd" "$flag")"
        found=1
    done <<< "$TOOLS"
    [ "$found" -eq 0 ] && echo "(none detected)"
    echo
}

emit_services() {
    local any=0
    echo "## Reachable services"
    echo
    while IFS='|' read -r host port label; do
        [ -z "$host" ] && continue
        service_reachable "$host" || continue
        printf -- '- %-10s %s:%s\n' "$label" "$host" "$port"
        any=1
    done <<< "$SERVICES"
    [ "$any" -eq 0 ] && echo "(none — this project has no database services)"
    echo
}

emit_text() {
    if [ "$TOOLS_ONLY" -eq 1 ]; then
        emit_tools
        return
    fi
    emit_static_context
    emit_tools
    emit_services
}

if [ "$JSON" -eq 1 ]; then
    emit_json
else
    emit_text
fi
