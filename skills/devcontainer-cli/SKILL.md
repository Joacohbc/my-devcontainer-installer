---
name: devcontainer-cli
description: Drive devcontainer-cli from the host to give a project a disposable Docker dev environment. Use when asked to create, start, stop, rebuild or destroy a devcontainer, to run builds/tests/commands inside one instead of on this machine, to reach a service running in one (published ports, port-forward, SSH), or to inspect what a container has installed. Also use before installing a toolchain or database on the host, when a container is the better place for it.
---

# devcontainer-cli from the host

`devcontainer-cli` builds and manages disposable Docker development
environments ("devcontainers") for a project directory. You are running **on the
host**: you drive the CLI, and the work happens inside the container.

Use it whenever a task wants a toolchain, a database or a risky command that
should not touch this machine. Anything installed inside is thrown away with the
container; the project directory itself is a bind mount, so edits there are real
edits on the host.

## The `agent` command group is your interface

`devcontainer-cli agent <verb>` is the command set meant for you. Every verb in
it is non-interactive by default, so none of them can open a wizard and leave
you waiting on a prompt you cannot answer.

| Command | What it does |
|---|---|
| `agent cli-info` | Print the catalogue: modules, services, profiles, skills, scripts, paths |
| `agent create` | Generate + build + start an environment for the current directory |
| `agent run --profile <id>` | Run a standalone or temporal throwaway container from a profile |
| `agent exec [-w] -- <cmd>` | Run a command inside the container (use `-w` to run from the workspace directory) |
| `agent ssh [--ephemeral]` | Open an SSH session into the container (ephemeral by default, bypassing ~/.ssh/config) |
| `agent forward <ports>` | Tunnel a container port to 127.0.0.1 (or a leading bind IP, e.g. `100.x.y.z:8000:8000`) via direct SSH tunnel (ephemeral by default); `reverse:<port>` tunnels the other way, exposing a host port inside the container |
| `agent list [path]` | List a directory inside the container (--json and -a supported) |
| `agent destroy [--yes]` | Tear down the project stack, remove volumes and configuration, and remove built images |

Use these first. Commands like file copying (`access copy`), context inspection
(`inspect context`), skill installation (`config skill install`), or container lifecycle
(`workspace up`, `workspace down`, `workspace destroy`, `config clean`) live in their
canonical 6 domains and are documented below.

## 🚨 STRICT RULE: NEVER TOUCH DOCKER DIRECTLY

You must **NEVER** use `docker`, `docker compose`, `docker-compose`, or `docker exec` directly to interact with a devcontainer environment, not even for simple commands like `curl` or `wget`.
You must **NEVER** manually edit the generated `docker-compose.yml` or `Dockerfile` located in `.dc_<workspace>`.

Instead, you must always use the CLI commands provided:
- To run commands inside: `devcontainer-cli agent exec -w -- <cmd>`
- To port-forward: `devcontainer-cli agent forward <ports>`
- To SSH: `devcontainer-cli agent ssh -- <cmd>`
- To run temporal tasks: `devcontainer-cli agent run --profile <id>`
- To tear down a workspace: `devcontainer-cli agent destroy --yes`

## Orient before acting

    devcontainer-cli --version
    devcontainer-cli agent cli-info       # what this CLI can build; --json to parse
    devcontainer-cli inspect status       # this project's containers
    devcontainer-cli inspect status --all # every managed container on the machine

Note: release builds validate CLI version and installed host skill freshness once every 24h on startup (printing notices to `stderr`); run `devcontainer-cli config upgrade-cli --check` to verify both immediately.

If the binary is missing, say so instead of installing it silently — the
installer is the user's call (`cli/install.sh` in the project repo).

A directory already has a devcontainer when it contains
`devcontainer.config.json` (the project's answers) and `.dc_<workspace>/`
(generated `Dockerfile`, `docker-compose.yml`, `.env` under
`.dc_<workspace>/build/`). Both are safe to read and belong to the CLI — edit
them by re-running generation, not by hand.

`<workspace>` defaults to the sanitized directory name, and container names are
`<workspace>-<service>`: the dev container itself is
`<workspace>-devcontainer-ssh`, databases are `<workspace>-postgres`,
`<workspace>-redis`, `<workspace>-mongo`.

### Several agents or git worktrees at once

Each checkout gets its own devcontainer with no extra flags: the workspace
comes from the directory name, so sibling worktrees (`repo-feature-a/`,
`repo-feature-b/`) never share a container. `devcontainer.config.json` and
`.dc_*/` are gitignored, so a fresh worktree starts with no devcontainer even
when the main checkout has one; run `agent create` from the worktree root and
never commit those files.

- **Pick the workspace name on the first `agent create`.** That run chooses a
  free compose subnet and saves it in `devcontainer.config.json`. Re-running
  `agent create --workspace <other>` keeps the saved subnet while the first
  workspace's network still holds it, and fails with
  `subnet X overlaps with existing Docker network X`. To rename, run
  `devcontainer-cli agent destroy --yes` first. Pass `--workspace` only when two checkouts share a
  directory name (e.g. `~/a/app` and `~/b/app`).
- **Run git on the host, not inside the container.** A worktree's `.git` is a
  file pointing at the main checkout's host path, which the container does not
  mount, so `git` fails there (`fatal: not a git repository`). Go builds stamp
  VCS info by default, so run them with `GOFLAGS=-buildvcs=false`.
- **Target a container explicitly** with `agent exec -c <workspace>-devcontainer-ssh`
  when the directory's config no longer matches the container you want, and
  check `devcontainer-cli inspect status --all` before creating anything.

## Pre-flight: Prompt Intent & Skill Assessment

Before composing or executing `agent create`, review the user's task, prompt,
and instructions. Map their requirements to matching toolchains, services, and
agent skills. Do not blindly spin up an environment with generic defaults.

### 1. Explore categories and match prompt requirements to workspace agent skills

Inspect the full catalogue with `devcontainer-cli agent cli-info --json` or
`config skill list`. The catalogue contains 13 built-in skills organized
into 3 distinct categories (`core-agents`, `frontend-ux-design`, `docs-content`, plus
user-defined skills).

> [!IMPORTANT]
> **Do NOT always pick the same few skills.** The table below is **only an illustrative sample
> of recommendations** across common task types. Always query the live catalogue
> (`agent cli-info --json`) to find the exact skill(s) best suited for the prompt
> (e.g., `agent-browser` for browser automation, `impeccable` for design tokens, `theme-factory`
> for color palettes, `canvas-design` for visual art, `pdf`/`docx` for office docs, etc.):

| User Task / Prompt Intent | Sample Suggested Skills (`--skills`) | Primary Category |
|---|---|---|
| Frontend UI, styling, design tokens | `frontend-design`, `ui-ux-pro-max`, `impeccable`, `theme-factory`, `web-artifacts-builder`, `algorithmic-art` | `frontend-ux-design` |
| Discovery & browser automation | `find-skills`, `agent-browser` | `core-agents` |
| Documentation, canvas, office formats | `canvas-design`, `docx`, `pdf`, `pptx`, `xlsx` | `docs-content` |

*(Note: `--skills` installs skills into the project's `.agents/skills` via the Skills CLI and automatically adds the `skills` module, which requires `nodejs`).*

### 2. Match prompt requirements to profiles, modules, and services

- **Prebuilt profile first**: If the stack matches a prebuilt profile (`nodejs`,
  `bun`, `python`, `go`, `node-python`, `node-go`, etc.), prefer `--mode profiles --profile <id>`
  for fast remote pull instead of a slow local build.
- **Databases**: Add `--services postgres`, `redis`, or `mongo` as requested in the prompt.
- **Client tools**: Add `--modules postgres-client` if the user will need CLI tools (`psql`).

### 3. Apply skills atomically or incrementally

- **When creating a new environment**: Pass `--skills` directly to `agent create`:
  ```bash
  devcontainer-cli agent create --mode profiles --profile nodejs --services postgres --skills webapp-testing,impeccable,tdd
  ```
- **When the container is ALREADY running**: Do not rebuild or recreate the container
  just to add an agent skill. Install it directly into the workspace with the ephemeral runner:
  ```bash
  devcontainer-cli config skill install webapp-testing impeccable
  ```

## Create an environment

Run it from the project directory. It generates the files, builds the image and
leaves the stack running, so the next `agent exec` works immediately:

    cd /path/to/project
    devcontainer-cli agent create --modules nodejs,pnpm,github-cli

Re-run it with different flags to change an existing project; it overwrites the
generated files without asking.

| Flag | Effect |
|---|---|
| `--modules a,b,c` | Dockerfile modules (toolchains/tools) to install |
| `--profile <id>` | With mode=custom (default): start from a profile — a module bundle plus its custom scripts. With mode=profiles: the `[remote]`-tagged id to pull instead of building |
| `--script <path>[:build\|start\|service\|manual]` | Add a script of your own (repeatable): baked into the image, run once per container start, run as background service on every start, or only copied to `~/post-script/` |
| `--skills find-skills,agent-browser,impeccable` | Agent skills installed **into the project workspace** via the Skills CLI. Adds the internal `skills` module, which requires `nodejs` |
| `--skills-mode manual\|auto` | `manual` (default) leaves the `install-skills` command to the user; `auto` installs on every container start, writing into the workspace unprompted |
| `--services mongo,postgres,redis` | Add database services to the compose stack |
| `--ports 3000:3000,8080:80` | Publish container ports (bound to 127.0.0.1 unless an IP is given). **Avoid this by default** — prefer `agent forward`/`access route add` instead (see below) |
| `--forward-ports 3000,reverse:5432` | Tunnels `access forward` opens for this project when called with no argument. Nothing is published; a `reverse:` spec brings a host port into the container instead |
| `--volumes myvol:/data,./cache:/cache` | Extra mounts on the dev container |
| `--workspace <name>` | Override the workspace name (default: directory name) |
| `--mode profiles --profile nodejs` | Skip the local build, pull a prebuilt ghcr.io image for a `[remote]`-tagged profile |

Module ids for `--modules`: `github-cli`, `nodejs`, `pnpm`, `yarn`, `bun`,
`python`, `go`, `rust`, `php`, `c-cpp`, `java-temurin`, `java-openjdk`,
`sqlite`, `postgres-client`, `redis-client`, `mongo-client`, `claude-code`,
`codex-cli`, `copilot-cli`, `opencode`, `antigravity-cli`, `chrome`, `ffmpeg`,
`dod` (Docker-out-of-Docker), `cloudflared`. Pick only one of the two
`java-*` modules.

`base`, `aliases`, `zellij` and `cleanup` are always applied, and the `skills`
module is added for you by `--skills` — none of them go in `--modules`.
**`agent cli-info` is authoritative for all of this**, including any profile or
skill the user defined themselves; the list above is a summary that can lag.

Databases are compose **services**, not modules: use `--services postgres`, and
`--modules postgres-client` only if you also want `psql` in the dev container.

Prebuilt images exist for these profile ids, and pulling one with
`--mode profiles --profile <id>` is much faster than a local build: `base`,
`nodejs`, `bun`, `python`, `go`, `java-temurin`, `node-go`, `node-python`,
`node-java-temurin`, `bun-go`, `bun-python`, `bun-java-temurin`. A `[local]` profile (only user-created ones) has nothing to
pull — build it with `agent create --profile <id>` on the default mode.

### Writing a profile, an image or a script

Read the matching file **before** writing one; they are not loaded otherwise.

| You are about to… | Read |
|---|---|
| write a script for `--script` or a profile/image (`when: build\|start\|service\|manual`) | `references/script-contract.md` |
| create or edit a profile (`profile.yml`) or an image (`image.yml`, `image create`, `from:`) | `references/profiles-and-images.md` |
| reason about what a stop, recreate, rebuild, `down` or `destroy` keeps | `references/lifecycle.md` |
| copy a working starting point | `examples/` (one profile per `when`, a base and a derived image) |

The one mistake to never make: a `build` script cannot touch the project —
`/workspaces` does not exist during `docker build`. Anything that writes into
the project is `when: start` and reads the path from `$DC_WORKSPACE_DIR`.

### Then read the container, before running anything in it

`agent create` tells you the image was built. It does not tell you what ended up
in it — a profile carries modules you did not name, a module pins a version, a
service publishes a port. Read that once, up front:

    devcontainer-cli inspect context          # or --json to parse

It streams the container's own `~/CONTEXT.md`, generated at build time from the
modules and services this project selected, plus a live inventory of the tools
with their versions and the reachable services. That is where the answers to
"which Python is in here", "how do I reach postgres", "is pnpm installed" are,
so you stop guessing and stop probing with throwaway `agent exec` calls.

Read it again after any `agent create` re-run or `image update --rebuild`: both can
change what is inside.

## Run commands inside

    devcontainer-cli agent exec -- go test ./...        # exit code propagated
    devcontainer-cli agent exec -- uv pip install requests
    devcontainer-cli agent exec -w -- pnpm install
    devcontainer-cli agent exec -c myapp-postgres --user postgres -T -- pg_dump devdb > dump.sql

`agent exec` is a `docker exec` wrapper and **always the first choice when the
container is local**: it avoids SSH overhead, runs immediately without `sshd`,
and enforces `devuser`. Only reach for `agent ssh` on a local container when the
task specifically requires SSH (`ssh-agent` forwarding, a real SSH PTY, or a tool
that shells out to `ssh`). When the container lives on a **remote Docker host**,
flip that priority and use `agent ssh --via <user@host> -c <container> -- <cmd>`
first. `agent exec` requires a command — there is no interactive mode to fall into.

It runs as **devuser**, who owns the workspace files. That matters more than it
looks: the container itself runs as root, so a command run as root that writes
into the project creates root-owned files on the **host**, in the user's real
checkout, which they then cannot edit without sudo. Do not pass `--user root`
unless the command genuinely needs it (`apt-get`, writing outside the home).

**Wrap the command in `bash -lc '…'` only when it needs a shell** — which is
narrower than it looks. `docker exec` runs your argv directly, so the wrapper is
what buys you pipes, `&&`, globs, `$VAR` expansion and `cd`, plus the container's
`pip`→`uv pip` and `npm`→`pnpm` indirections, which are shell functions and
aliases that no process inherits. Use `bash`, not `zsh`: a non-interactive
`zsh -lc` skips `~/.zshrc`.

**Finding a binary is not one of those reasons.** The image declares its
toolchain PATH in the image environment, so `node`, `uv`, `pnpm`, `cargo`,
`bun`, `go` and anything in `~/.local/bin` are found by a bare
`agent exec -- <cmd>`. Wrapping a plain command adds a login shell for nothing:
`agent exec -- uv pip install requests` is right, `bash -lc 'uv pip install
requests'` just does the same thing slower. Node resolves to the version the
image was built with; a shell still picks whatever `fnm`/`nvm` selects for that
session, so switching versions works as usual.

The exception is **an image built before this was in place**, which carries the
PATH only in its rc files. If a tool you know is installed comes back as
`executable file not found in $PATH`, re-run it wrapped in `bash -lc` and
rebuild with `devcontainer-cli image update --rebuild`.

Other notes that matter:

- Redirections you write on the host line (`> dump.sql`) are handled by *your*
  shell, so they need no `bash -lc` — but pass `-T` for them, so no TTY mangles
  the stream.
- `-c/--container <name>` targets another container in the stack. Database
  containers are plain images with **no devuser** and often no bash, so the
  devuser default does not apply to them: name the user they do have and run the
  command bare (`-c <ws>-postgres --user postgres -- psql -l`), not through
  `bash -lc`. Without `--user` they fail with `unable to find user devuser`.

The project is mounted at `/workspaces/<workspace>` inside the container (short
alias `/workspace/<workspace>`). Never assume a bare `/workspace`.

**You do not start there.** Nothing in the image declares a `WORKDIR`, so a
command lands in `/` and an interactive shell in `/home/devuser`. Pass `-w` to
run in the project instead — that is what a build or a test almost always means,
and it is why most commands do not need a `bash -lc 'cd … && …'` wrapper:

    devcontainer-cli agent exec -w -- go test ./...
    devcontainer-cli agent exec -w -- bash -lc 'pnpm install && pnpm build'

`-w` targets the project's own mount, so leave it off for a database container,
which has no such directory.

### Long-lived interactive or TUI processes in the background (Zellij)

Interactive TUI tools require a real PTY and exit immediately if launched with
`nohup ... < /dev/null &`, or die when you `exit` your interactive shell.
Because `zellij` is preinstalled in every image, use a detached Zellij session
to give the process a persistent PTY independent of your shell session and PID 1
(`sshd`):

    # Create a detached background session and run the command inside it
    devcontainer-cli agent exec -w -- zellij attach --create-background <session>
    devcontainer-cli agent exec -w -- zellij --session <session> run -- <command>

    # Check active sessions or reattach later (detach with Ctrl+O, then D)
    devcontainer-cli agent exec -- zellij list-sessions
    devcontainer-cli access shell -- zellij attach <session>

### Package managers are not the ones you expect

- **Python: `uv`, never `pip`.** `uv add X` for a project dependency, `uv tool
  install X` for a CLI, `uv pip install X` for a library importable anywhere —
  no venv, no `--break-system-packages`.
- **Node: call `pnpm` directly.** `npm`/`npx` are aliases a non-interactive
  shell does not expand.

Why, and the exceptions: `references/package-managers.md`.

## Lifecycle

`agent create` already starts the stack. To drive it afterwards:

    devcontainer-cli workspace init              # interactive wizard (prompts: never from an agent)
    devcontainer-cli workspace auto-init         # detect the stack, build and start without prompts
    devcontainer-cli workspace up                # create/recreate and start (detached)
    devcontainer-cli workspace up --build        # rebuild the image first
    devcontainer-cli workspace start | stop | restart
    devcontainer-cli workspace down              # remove containers + network, keep volumes
    devcontainer-cli workspace down -v --yes     # also delete the named volumes (data loss)

    devcontainer-cli image update                # rebuild (mode=custom) or pull (mode=profiles)
    devcontainer-cli image update --rebuild      # force a rebuild

`start`/`stop`/`restart` accept `-c <container>` to act on one container.

What each step runs, and what survives a stop, a recreate, a `down` or a
`destroy` (your files, volumes, `$HOME`, `start` script sentinels):
`references/lifecycle.md`.

## Reach services across the boundary

**Prefer a tunnel or a route over publishing ports with `--ports`**: a tunnel is
on demand and cannot collide with another project; a published port changes the
project for good.

    devcontainer-cli agent forward 3000                 # 127.0.0.1:3000 -> container:3000 (foreground)
    devcontainer-cli agent forward 5432 --service postgres  # a sibling compose service
    devcontainer-cli agent forward reverse:5432         # container:5432 -> a service on THIS machine
    devcontainer-cli access route add 3000              # http://<workspace>-3000.devcli.localhost (foreground; Ctrl+C removes)
    devcontainer-cli access network connect other-app --alias billing  # attach another stack container
    devcontainer-cli agent ssh -- go version            # only locally if SSH semantics are needed

Inside the container, sibling services are reached by compose service name
(`postgres`, `redis`, `mongo`), never `localhost`. Both `agent forward` and
`access route add` bind strictly to `127.0.0.1` by default and stay in the
foreground. Bind addresses, reverse-tunnel limits, permanent `forward_ports`,
routes (`rm`/`stop`) and `network disconnect`: `references/networking.md`.

## Inspect

    devcontainer-cli inspect context       # what is installed inside; --json for parsing
    devcontainer-cli inspect status        # compact table (state, ports, image)
    devcontainer-cli inspect info          # ports, mounts, IPs, timestamps
    devcontainer-cli inspect logs -f
    devcontainer-cli inspect logs postgres --tail 100
    devcontainer-cli agent list /home/devuser --json

`inspect context` is the one to read before planning work inside a container: it prints
the container's own `~/CONTEXT.md` plus the live tool inventory with versions,
so you never guess whether a toolchain is there. `agent cli-info` describes the
CLI; `inspect context` describes one running container.

Move files with `access copy` (`:` marks the container side):

    devcontainer-cli access copy ./seed.sql :/home/devuser/seed.sql
    devcontainer-cli access copy :/home/devuser/out.log ./out.log
    devcontainer-cli access copy --asset install-claude-code

## Clean up

    devcontainer-cli agent destroy --dry-run    # what would go
    devcontainer-cli agent destroy --yes        # this project only

`agent destroy` is irreversible: it brings the stack down with its volumes (data
in them is deleted), removes `.dc_<workspace>/` and `devcontainer.config.json`,
prunes the project's SSH host block, and removes the image built for it. An
image pulled from a registry is left alone — other projects share it.

Confirm with the user before running it, and **always** before
`agent destroy --all --yes`, which additionally sweeps every other managed container,
image, network and volume on the machine.

## Beyond the agent group

These have no `agent` spelling; use them when the task calls for it.

Any compose verb the wrappers do not cover, scoped to this project:

    devcontainer-cli workspace compose ps
    devcontainer-cli workspace compose exec -T postgres psql -U devuser -c '\l'

A throwaway container with no project to configure:

    devcontainer-cli agent run --profile nodejs --name scratch
    devcontainer-cli agent destroy --container scratch --yes

`agent run --profile` only accepts a `[remote]`-tagged id (`base` for no
language), since it always pulls.

A container on a **different Docker host**, through an existing SSH connection
to it — **prefer `agent ssh --via`** (ephemeral, runs as `devuser`, never prompts):

    devcontainer-cli agent ssh --via me@docker-host -c dc-base -- uname -a
    devcontainer-cli access shell --via me@docker-host -c dc-base --user devuser -- uname -a  # fallback if target has no sshd
    devcontainer-cli config ssh-key --export me@docker-host  # only if the user asks to configure cross-host SSH client access

Global config, profiles and shared logins:

    devcontainer-cli config                       # current defaults
    devcontainer-cli profile list                 # bundles for --profile; [remote]/[local] tag
    devcontainer-cli profile info <id>            # one profile's full resolved definition
    devcontainer-cli config skill list            # agent skills for --skills; built-in + user-defined
    devcontainer-cli config skill install [ids...] # install skills at project level in an ephemeral container
    devcontainer-cli config skill add <id> --ref owner/repo   # define your own
    devcontainer-cli config alias set ll "ls -la" # aliases for every container
    devcontainer-cli config shared sync alias.sh  # push them into running containers

`config shared sync` seeds a shared Docker volume from this machine's tool
configs (`~/.claude`, `~/.config/gh`, …) so logins persist across every
container. It copies credentials into a volume — only run it when the user asks
for it, and never as a side effect of another task.

Targeted cleanup across the machine, one category at a time:

    devcontainer-cli config clean --dry-run           # preview across all categories
    devcontainer-cli config clean containers          # stopped managed containers
    devcontainer-cli config clean images
    devcontainer-cli config clean all --yes           # sweep everything managed

Every `config clean` subcommand touches only resources this CLI created (they carry a
managed label). Use `--dry-run` first and show the user what would go.

## Rules

- **Prefer `agent <verb>`.** Those never prompt. If you drop to a plain command
  that can prompt (generation, `workspace init`, `run`, `workspace down`, `workspace destroy`, `config clean`,
  `access forward`), pass `--no-interactive` or it opens a wizard and hangs. Add
  `-y/--yes` for destructive commands — and get the user's agreement first for
  `agent destroy`, `workspace destroy`, `workspace down -v` and `config clean all`.
- **Reach a service with `agent forward` or `access route add`, not `--ports`.**
  Publishing a port is a permanent project change; a tunnel or a route is
  on-demand and leaves nothing behind. Only publish when the user explicitly
  needs the port open outside a tunnel/route.
- **A host service needs `agent forward reverse:<port>` to exist inside.**
  The container's `localhost` is its own; nothing on this machine answers there
  by default. Open the reverse tunnel instead of editing the project to publish
  a port, and never assume a host database is reachable because it runs.
- **Assess skills and intent before running `agent create`.** Analyze the user's prompt
  and explore the full catalogue (`agent cli-info --json` or `config skill list`) across its
  categories (`core-agents`, `frontend-ux-design`, `docs-content`, plus user-defined skills).
  Do not default to the same subset every time; select the specific skill that best solves
  the prompt. If the container already exists, use `config skill install <id...>` instead of recreating it.
- **Read `agent cli-info` before composing a create.** It is generated from the
  live catalogue, so it never names a module this binary does not have — and it
  includes the user's own profiles and skills, which no document can.
- **Read `inspect context` after every create, before working inside.**
  `cli-info` says what this CLI could build; `inspect context` says what the
  container actually has. Skipping it is how you end up running the wrong
  Python, guessing a database host, or installing something already there.
- **`bash -lc '…'` only for shell syntax** (`&&`, `|`, `*`, `cd`, `$VAR`) or the
  container's `pip`/`npm` indirections — not to find a binary, which the image
  PATH already handles. `command not found` from `agent exec` on an image built
  before that was in place means the PATH lives only in the rc files: re-run it
  wrapped, check with `inspect context`, and rebuild with `image update --rebuild`.
- **Install packages with the container's package manager**, and with the right
  one for the job: `uv add` for a project dependency, `uv tool install` for a
  CLI, `uv pip install` for something importable container-wide, `pnpm` for
  Node. An error telling you to create a venv, pass `--break-system-packages` or
  `apt install python3-xyz` means you used the wrong tool — switch, don't force
  it; the image already sets what `uv pip` needs.
- **Nothing outside the workspace mount and `/home/devuser` survives** a
  recreate. Install-and-forget inside the container is fine for a one-off; a
  toolchain that must persist belongs in `--modules` and a re-run of `agent create`.
- **Changing the image is a host-side action.** Adding a module, a service or a
  published port means re-running `agent create` with the new flags — not
  `apt-get install` inside the container.
- Run every command from the project directory; the CLI resolves the workspace
  from the current directory unless `--workspace` says otherwise.
- **Local = `agent exec` first; Remote (`--via`) = `agent ssh --via` first.** Use
  `agent exec` on local containers unless SSH-specific features are needed. On
  remote Docker hosts (`--via`), prioritize `agent ssh --via <user@host> -c <ctr> -- <cmd>`
  (ephemeral and non-interactive), falling back to `access shell --via ... --user devuser`
  only if the container runs no `sshd`.
- Long-running commands (`inspect logs -f`, `agent forward`, `access route add`,
  `agent ssh` without a command) do not return. Give them a command, a `--tail`,
  or run them in the background.
- `--help` on any command is authoritative and current; check it before
  inventing a flag.

<!-- devcontainer-cli:managed skill=devcontainer-cli — installed via 'npx skills add Joacohbc/my-devcontainer-installer@devcontainer-cli -g'; local edits are replaced on reinstall. -->

