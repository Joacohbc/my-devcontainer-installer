# Container lifecycle

What happens at each step, and what survives it. Knowing this is what tells
you whether a change needs `start`, `up`, a rebuild, or nothing.

## The steps

1. **Generate** (`agent create`, `workspace auto-init`): writes
   `devcontainer.config.json` and the generated stack in `.dc_<workspace>/`
   (Dockerfile, compose file, staged scripts). Never edit `.dc_<workspace>/`:
   re-run `agent create` with other flags instead.
2. **Build** (part of `agent create`, or `workspace up --build`,
   `image update`): `docker build` runs the modules, then the `build` scripts as
   `devuser`. No project, no volumes, no services exist yet.
3. **Create + start** (`workspace up`): compose creates the containers. The dev
   container mounts the project at `/workspaces/<workspace>` (alias
   `/workspace/<workspace>`) and the shared-config volume, then runs the
   entrypoint:
   1. users and passwords;
   2. resolve the project mount and its alias;
   3. remap `devuser`'s UID/GID to the project's owner (no host permission changes);
   4. link tool config (`~/.claude`, `~/.codex`, `~/.config/gh`, `~/.agents`, …)
      into the shared-config volume;
   5. aliases and the context skill;
   6. `start` scripts (`~/post-script/start.d/`, in name order, once each), in background;
   7. `service` scripts (`~/post-script/services.d/`), in background;
   8. `sshd` as PID 1 — the container lives as long as it does.

   Steps 6–7 run in the background, so the container is usable before they
   finish. Their output is in `~/.post-script-state/<name>.log`.
4. **Stop / start / restart** (`workspace stop|start|restart`): the same
   container. The entrypoint runs again, `service` scripts start again, `start`
   scripts are skipped (their `.done` sentinel is in the container).
5. **Down** (`workspace down`): removes containers and network. Named volumes
   stay unless `-v --yes`. The next `up` creates a **new** container.
6. **Destroy** (`agent destroy --yes`): down with volumes, removes
   `.dc_<workspace>/`, `devcontainer.config.json`, the SSH host block and the
   project's built image.

## What survives what

| | stop/start | `up` (recreate) / rebuild | `down` | `down -v` / `destroy` |
|---|---|---|---|---|
| Project files (host bind mount) | ✔ | ✔ | ✔ | ✔ (destroy keeps your code) |
| Shared tool config (logins, global skills) | ✔ | ✔ | ✔ | ✔ (shared by every project) |
| Service data (postgres, redis, mongo volumes) | ✔ | ✔ | ✔ | ✘ |
| Extra named volumes (`--volumes`) | ✔ | ✔ | ✔ | ✘ |
| Anything else in `$HOME` or the system | ✔ | ✘ | ✘ | ✘ |
| `start` script sentinels (`.done`) | ✔ — skipped | ✘ — they run again | ✘ | ✘ |
| What `build` scripts installed | ✔ | ✔ (rebuilt if inputs changed) | ✔ | image removed by destroy |

Consequences:

- A tool installed by hand (`agent exec -- uv tool install x`) is gone on the
  next recreate. If it must stay, make it a module or a `build` script.
- A `start` script re-runs against the same project on every recreate: it must
  be idempotent.
- To pick up a changed profile, image or script: `agent create` (regenerate) or
  `workspace up --build`; a plain `start` uses the old image.

## Recreate vs rebuild

`workspace up` recreates the container only when its compose definition changed
and rebuilds only with `--build` (or `image update`). The image is tagged by a
fingerprint of everything that goes into it (modules, options, scripts, parent
image), so an unchanged project reuses its image.
