# Custom script contract

A profile or image carries shell scripts next to its manifest. Each declares
`when` it runs, and that one word decides what exists while it runs. Most
broken scripts are a correct script declared in the wrong phase.

```yaml
scripts:
  - file: install-tool.sh     # [A-Za-z0-9._-]+.sh, a file name next to the manifest
    when: build               # build (default) | start | service | manual
```

## What each phase gets

| | `build` | `start` | `service` | `manual` |
|---|---|---|---|---|
| Runs | a `RUN` layer of `docker build` | entrypoint, once per container | entrypoint, on **every** container start | never on its own |
| As | `devuser` (`su -`) | `devuser`, `bash -l`, in background | `devuser`, `nohup bash -l`, in background | whoever runs it |
| Project at `/workspaces/<name>` | **no** | yes | yes | yes |
| Shared-config volume, compose services | **no** | yes | yes | yes |
| Network | yes | yes | yes | yes |
| Modules' toolchains on `PATH` | yes | yes | yes | yes (login shell) |
| `DC_SCRIPT_PHASE` | `build` | `start` | `service` | — |
| `DC_WORKSPACE_DIR`, `DC_WORKSPACE` | — | set | set | — (use `$PWD`) |
| Output | the build log | `~/.post-script-state/<name>.log` | `~/.post-script-state/<name>.log` | your terminal |
| Lands at | deleted after its layer | `~/post-script/start.d/90-custom-<file>` | `~/post-script/services.d/custom-<file>` | `~/post-script/custom-<file>` |

`DC_WORKSPACE_DIR` is the project directory (`/workspaces/<workspace>`) and
`DC_WORKSPACE` its name; both are empty if no project is mounted. `su -` starts from an empty environment, so only these
variables and what the login shell sets are guaranteed; do not rely on the
compose `environment:` reaching a start or service script.

## Pick the phase

- **Installs something project-independent** (a CLI, a language tool, a system
  package) → `build`. It is cached in the image and costs nothing per container.
- **Touches the project** (writes `.mcp.json`, `.env`, editor config, seeds a
  database) → `start`. The project does not exist during `build`.
- **Must keep running** (dev server, watcher, tunnel) → `service`.
- **Prompts, logs in, or is destructive** → `manual`. Nothing interactive may run
  unattended.

The CLI warns before building when a `build` script mentions `/workspaces` or
`/workspace` (not under `$HOME`), `$WORKSPACE` or `$DC_WORKSPACE*` outside a comment: that build
would fail at its own layer with `Permission denied`.

## Rules

- Start every script with `#!/usr/bin/env bash` and `set -euo pipefail`.
- **`build`**: write only to `$HOME` and system paths (via the modules' tools —
  there is no root). Never put a secret in a build script: it stays in a layer.
- **`start`**: make it idempotent. A `.done` sentinel skips it on `workspace
  start`, but a recreated container (`workspace up` after a change, `image
  update`, `down` + `up`) runs it again against the same project. Do not
  overwrite files the user may have edited: check before writing.
- **`start`**: read the project path from `$DC_WORKSPACE_DIR`. Images built before
  the contract existed do not set it; fall back to the first `/workspaces/*/`.
- **`service`**: stay in the foreground (`exec your-server`). The entrypoint
  already backgrounds it; a daemon that forks away loses its log.
- **`manual`**: it runs in the user's shell, so it may prompt.
- A failing `start` or `service` script does not stop the container: read its
  log. A failing `build` script fails the build.

## Debugging

    devcontainer-cli agent exec -- ls ~/.post-script-state/
    devcontainer-cli agent exec -- cat ~/.post-script-state/90-custom-setup.sh.log
    devcontainer-cli agent exec -- bash -l ~/post-script/start.d/90-custom-setup.sh   # re-run by hand

To force a `start` script to run again on the next start, delete its `.done`
file in `~/.post-script-state/`.

Worked examples, one per phase: `../examples/` (see `../examples/README.md`).
