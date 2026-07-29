#!/bin/bash
# Writes ~/CONTEXT.md — the orientation document an AI coding agent (or a human)
# should read first inside this container. Run as devuser at build time from the
# aliases module so every container ships it.
#
# Deliberately STATIC and toolset-agnostic: it describes the conventions that
# hold in every image variant. Anything variant-specific (which languages and
# CLIs are actually installed, which database services are reachable) is
# reported at runtime by `get-devcontainer-context`.
set -e

cat > "$HOME/CONTEXT.md" <<'CONTEXT'
# Container context

You are running **inside a Docker container** managed by `devcontainer-cli`.
Read this before running commands; the conventions here are not the same as on
a normal host.

Run `get-devcontainer-context` for the live inventory of this specific
container: installed tools with versions, the resolved workspace, reachable
database services and mounted volumes.

## Where you are

- The project is bind-mounted at `/workspace` (a stable alias of
  `/workspaces/<project-name>`). **Edit code only there.**
- You are the `devuser` user, with passwordless `sudo`. Installing system
  packages with `sudo apt-get install` is fine and expected.
- `/home/devuser` persists for the life of the container; parts of it
  (`~/.claude`, `~/.codex`, `~/.config/gh`, `~/.agents`, …) are symlinks into a
  Docker volume shared by *every* container, so tool logins and agent skills
  survive rebuilds.
- **Everything outside `/workspace` and `/home/devuser` is thrown away** when
  the container is recreated. Never leave work there.
- There is no systemd and no init system. Do not use `systemctl` or `service`.

## Networking

- Database services (postgres, redis, mongo) are **sibling containers**, not
  local processes. Reach them by their service name — `postgres:5432`,
  `redis:6379`, `mongo:27017` — **not** `localhost`.
- A port you bind inside the container is only reachable from the host if it was
  published when the container was created.
- `kill_port <port>` frees a port that is already taken.

## Python — use `uv`, not `pip`

`uv` is the package manager here.

| Instead of | Use |
|---|---|
| `pip install X` | `uv pip install X` |
| `python script.py` | `uv run script.py` |
| `pipx install X` | `uv tool install X` |
| creating a venv | nothing — `UV_SYSTEM_PYTHON=1` is already exported |

`pip` and `pip3` are already shell functions that forward to `uv pip`, so the
old commands keep working. **No virtualenv is needed**: the container is the
isolation boundary, and `UV_SYSTEM_PYTHON=1` makes `uv pip` target the system
interpreter directly.

## JavaScript / TypeScript — use `pnpm`, not `npm`

| Instead of | Use |
|---|---|
| `npm install` | `pnpm install` |
| `npm install X` | `pnpm add X` |
| `npm run X` | `pnpm run X` |
| `npx X` | `pnpm dlx X` |

`npm` and `npx` are already aliased to `pnpm` and `pnpm dlx`. Respect a
lockfile that is already in the repo: if the project has a `package-lock.json`
and no `pnpm-lock.yaml`, use `command npm` rather than silently switching the
project's package manager.

## AI agents

The agent CLIs (`claude`, `codex`, `copilot`) are aliased to skip their
interactive permission prompts — the container is the sandbox. Use
`command claude …` (or `\claude …`) to get the unaliased command back.

## Shell customisation

- `~/.devcontainer_aliases.sh` — the defaults above. Baked into the image;
  editing it here is pointless, the change is lost on recreate.
- `~/.alias.sh` — **your** aliases. Persists across every container. Sourced
  after the defaults, so it overrides them.
CONTEXT

echo "Wrote container context to $HOME/CONTEXT.md"
