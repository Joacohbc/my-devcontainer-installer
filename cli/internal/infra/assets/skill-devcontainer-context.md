---
name: devcontainer-context
description: Orientation for working inside a devcontainer-cli Docker container. Use at the start of a session in this environment, and whenever you are unsure where the project lives, which tools or versions are installed, which ports and services are reachable, or how to install something. Reads ~/CONTEXT.md and runs get-devcontainer-context.
---

# Devcontainer context

You are **not** on the user's machine. This session runs inside a Docker
container built and managed by `devcontainer-cli`. The conventions here are not
the ones you would assume on a normal host, so get the facts before running
commands.

## Do this first

1. Read the container's own description:

       cat ~/CONTEXT.md

   It is generated at build time from the modules and services this image was
   built with, so it only ever describes what is actually installed: the
   toolchains, the workspace path, the published ports, the databases and their
   credentials.

2. Get the live inventory:

       get-devcontainer-context           # human/markdown output
       get-devcontainer-context --tools   # installed tools and versions only
       get-devcontainer-context --json    # structured, easier to parse

   This is the runtime half: what is installed *right now*, with versions, plus
   which sibling services actually answer.

Do both once per session, before planning work. `~/CONTEXT.md` answers "what is
this container", the command answers "what is true at this moment".

## What to keep in mind

- **Edit code only in the workspace mount** (`/workspaces/<project>`, aliased as
  `/workspace/<project>`). It is a bind mount of real files on the user's
  machine: deleting there deletes on the host. There is no bare `/workspace`
  directory to work in — every project has its own path, so that whatever you
  key to the working directory stays that project's.
- **Everything outside the workspace and `/home/devuser` is discarded** when the
  container is recreated. Never leave work there.
- You are `devuser` with passwordless `sudo`. Installing packages with
  `sudo apt-get install` is fine and expected — the container is disposable and
  is itself the sandbox.
- **Check the inventory before installing anything.** A toolchain is often
  already present through a version manager (fnm/nvm for Node, uv for Python,
  the Go tarball) rather than through apt, and a second copy from apt will
  shadow it.
- There is no systemd and no init system. `systemctl` and `service` do not work;
  start processes directly.
- **Only the ports the project published are reachable from the host.** Binding
  another port inside the container does not expose it — that needs the project
  to be regenerated, or `devcontainer-cli port-forward`, from the host.
- **A service running on the host is not reachable from here either**, unless
  the host opened a reverse tunnel for it
  (`devcontainer-cli port-forward reverse:<port>`), which makes it answer on
  this container's own `127.0.0.1:<port>`. Ask for one instead of assuming the
  host's `localhost` is yours.
- Sibling services (postgres, redis, mongo, …) are reached by their **compose
  service name** as hostname, not `localhost`. Names, ports and credentials are
  in `~/CONTEXT.md`.
- **A change to the image itself does not belong inside the container.** Adding
  a toolchain permanently, editing `~/.devcontainer_aliases.sh`, changing
  published ports or services — all of that is host-side configuration
  (`devcontainer-cli` + `devcontainer.config.json`) and needs a rebuild. Say so
  instead of making a change that is lost on the next recreate.
- Parts of the home (`~/.claude`, `~/.codex`, `~/.agents`, `~/.config/gh`, …)
  may be symlinks into a volume **shared with every other devcontainer**.
  Treat global tool config as shared state; keep project-specific settings in
  the workspace.

## If `~/CONTEXT.md` is missing

The image predates it. Fall back to `get-devcontainer-context`, and if that is
missing too, the user can copy it in from the host with:

    devcontainer-cli copy --asset get-devcontainer-context

Report what you could not determine instead of guessing at the environment.
