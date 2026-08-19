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
| `agent create [--temporal]` | Generate + build + start an environment for the current directory (or throwaway with `--temporal`) |
| `agent exec [-w] -- <cmd>` | Run a command inside the container (use `-w` to run from the workspace directory) |
| `agent ssh [--ephemeral]` | Open an SSH session into the container (ephemeral by default, bypassing ~/.ssh/config) |
| `agent forward <ports>` | Tunnel a container port to 127.0.0.1 via direct SSH tunnel (ephemeral by default) |
| `agent copy <src> <dest>` | Move files between host and container |
| `agent list [path]` | List a directory inside the container (--json and -a supported) |
| `agent context` | Print what the container IS and has installed: its `~/CONTEXT.md` plus a live tool inventory (`--json`) |
| `agent clean` | Destroy the project and remove what it left behind |

Use these first. The plain commands they wrap (`shell`, `ssh`, `port-forward`,
`copy`, `ls`, `destroy`, `clean`) still exist and are documented at the end —
reach for them only for the cases the group does not cover.

## 🚨 STRICT RULE: NEVER TOUCH DOCKER DIRECTLY

You must **NEVER** use `docker`, `docker compose`, `docker-compose`, or `docker exec` directly to interact with a devcontainer environment, not even for simple commands like `curl` or `wget`.
You must **NEVER** manually edit the generated `docker-compose.yml` or `Dockerfile` located in `.dc_<workspace>`.

Instead, you must always use the CLI commands provided:
- To run commands inside: `devcontainer-cli agent exec -w -- <cmd>`
- To port-forward: `devcontainer-cli agent forward <ports>`
- To SSH: `devcontainer-cli agent ssh`
- To run temporal tasks: `devcontainer-cli agent create --temporal --profile <id>`

## Orient before acting

    devcontainer-cli --version
    devcontainer-cli agent cli-info       # what this CLI can build; --json to parse
    devcontainer-cli status               # this project's containers
    devcontainer-cli status --all         # every managed container on the machine

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

## Create an environment

Run it from the project directory. It generates the files, builds the image and
leaves the stack running, so the next `agent exec` works immediately:

    cd /path/to/project
    devcontainer-cli agent create --with nodejs,pnpm,github-cli

Re-run it with different flags to change an existing project; it overwrites the
generated files without asking.

| Flag | Effect |
|---|---|
| `--with a,b,c` | Dockerfile modules (toolchains/tools) to install |
| `--profile <id>` | With mode=custom (default): start from a profile — a module bundle plus its custom scripts. With mode=profiles: the `[remote]`-tagged id to pull instead of building |
| `--script <path>[:build\|start\|manual]` | Add a script of your own (repeatable): baked into the image, run once per container start, or only copied to `~/post-script/` |
| `--skill firecrawl,agent-browser,webapp-testing` | Agent skills installed **into the project workspace** via the Skills CLI. Adds the internal `skills` module, which requires `nodejs` |
| `--skills-mode manual\|auto` | `manual` (default) leaves the `install-skills` command to the user; `auto` installs on every container start, writing into the workspace unprompted |
| `--service mongo,postgres,redis` | Add database services to the compose stack |
| `--ports 3000:3000,8080:80` | Publish container ports (bound to 127.0.0.1 unless an IP is given) |
| `--volumes myvol:/data,./cache:/cache` | Extra mounts on the dev container |
| `--workspace <name>` | Override the workspace name (default: directory name) |
| `--mode profiles --profile nodejs` | Skip the local build, pull a prebuilt ghcr.io image for a `[remote]`-tagged profile |
| `--no-up` | Generate and build, but leave the containers stopped |
| `--temporal` | Run an ephemeral container without project files |
| `--name <name>` | Container name for temporal run (default: `dc-<profile>`) |
| `--expose-all` | Publish ports on all interfaces for temporal run |
| `--copy-ai-scripts` | Copy AI scripts into the temporal container |

Module ids for `--with`: `github-cli`, `nodejs`, `pnpm`, `yarn`, `bun`,
`python`, `go`, `rust`, `php`, `c-cpp`, `java-temurin`, `java-openjdk`,
`sqlite`, `postgres-client`, `redis-client`, `mongo-client`, `claude-code`,
`codex-cli`, `copilot-cli`, `opencode`, `antigravity-cli`, `graphify`,
`caveman`, `claude-mem`, `context-mode`, `ecc`, `browser-harness`, `chrome`, `ffmpeg`,
`dod` (Docker-out-of-Docker), `ngrok`, `cloudflared`. Pick only one of the two
`java-*` modules.

`base`, `aliases`, `zellij` and `cleanup` are always applied, and the `skills`
module is added for you by `--skill` — none of them go in `--with`.
**`agent cli-info` is authoritative for all of this**, including any profile or
skill the user defined themselves; the list above is a summary that can lag.

Databases are compose **services**, not modules: use `--service postgres`, and
`--with postgres-client` only if you also want `psql` in the dev container.

Prebuilt images exist for these profile ids, and pulling one with
`--mode profiles --profile <id>` is much faster than a local build: `nodejs`,
`bun`, `python`, `go`, `java-temurin`, `node-go`, `node-python`,
`node-java-temurin`, `bun-go`, `bun-python`, `bun-java-temurin`. A `[local]`
profile (e.g. `scraper`, or a user-created one) has nothing to pull — build it
with `agent create --profile <id>` on the default mode.

### Then read the container, before running anything in it

`agent create` tells you the image was built. It does not tell you what ended up
in it — a profile carries modules you did not name, a module pins a version, a
service publishes a port. Read that once, up front:

    devcontainer-cli agent context            # or --json to parse

It streams the container's own `~/CONTEXT.md`, generated at build time from the
modules and services this project selected, plus a live inventory of the tools
with their versions and the reachable services. That is where the answers to
"which Python is in here", "how do I reach postgres", "is pnpm installed" are,
so you stop guessing and stop probing with throwaway `agent exec` calls.

Read it again after any `agent create` re-run or `update --rebuild`: both can
change what is inside.

## Run commands inside

    devcontainer-cli agent exec -- go test ./...        # exit code propagated
    devcontainer-cli agent exec -- uv pip install requests
    devcontainer-cli agent exec -w -- pnpm install
    devcontainer-cli agent exec -c myapp-postgres --user postgres -T -- pg_dump devdb > dump.sql

`agent exec` is a `docker exec` wrapper and the normal way to do work in the
container. It requires a command — there is no interactive mode to fall into.

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
rebuild with `devcontainer-cli update --rebuild`.

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

### Package managers are not the ones you expect

The image replaces the usual tools, and the replacements live in shell
functions and aliases — so the tool you reach for from `agent exec` is often the
wrong one. Run `devcontainer-cli context` to see which of these the image has.

- **Python: `uv`, never `pip`.** uv has one command per case, and they are not
  interchangeable:

  | What you want | Command |
  |---|---|
  | A dependency of **one project** | `uv add X`, then `uv run …` |
  | A **command-line tool** | `uv tool install X` |
  | A library importable **anywhere in the container** | `uv pip install X` |

  `uv add` is the one for project work; the `.venv/` it creates in the project
  directory is expected and you never activate it by hand. `uv pip install X`
  writes to the system interpreter and needs no venv and no sudo — the image
  sets `UV_SYSTEM_PYTHON=1` and `UV_BREAK_SYSTEM_PACKAGES=1` and gives devuser
  write access to the install directories, so **do not pass
  `--break-system-packages` yourself and do not build a venv to work around an
  install error**.

  Never `pip install` or `python3 -m pip install`: the base is Ubuntu 24.04,
  whose system Python is PEP 668 "externally managed", so plain pip refuses
  with `error: externally-managed-environment`. `pip`/`pip3` are shell
  *functions* forwarding to `uv pip`, so they work in a shell but not from a
  bare `agent exec` — call `uv pip` directly. If the image was built without uv
  (`--with python` sets the `uv` option, which can be turned off), you are on
  plain pip and a venv is then the honest answer.
- **Node: call `pnpm` directly.** `npm`/`npx` are *aliases*, and a
  non-interactive shell does not expand aliases — so `bash -lc 'npm install'`
  really runs npm, not pnpm.

      devcontainer-cli agent exec -- uv pip install requests
      devcontainer-cli agent exec -- pnpm add -D vitest
      devcontainer-cli agent exec -w -- uv add rich

## Lifecycle

`agent create` already starts the stack. To drive it afterwards:

    devcontainer-cli up                # create/recreate and start (detached)
    devcontainer-cli up --build        # rebuild the image first
    devcontainer-cli start | stop | restart
    devcontainer-cli down              # remove containers + network, keep volumes
    devcontainer-cli down -v --yes     # also delete the named volumes (data loss)

    devcontainer-cli update            # rebuild (mode=custom) or pull (mode=profiles)
    devcontainer-cli update --rebuild  # force a rebuild

`start`/`stop`/`restart` accept `-c <container>` to act on one container.

## Reach services in the container

Ports published at creation time (`--ports`) are reachable on `127.0.0.1`
directly. For a port that was not published, tunnel it instead of regenerating:

    devcontainer-cli agent forward 3000              # 127.0.0.1:3000 -> container:3000
    devcontainer-cli agent forward 8080:80           # 127.0.0.1:8080 -> container:80
    devcontainer-cli agent forward 5432:postgres:5432  # reach a sibling service

`agent forward` tunnels over SSH and stays in the **foreground** until
interrupted, so run it in the background (or in a separate terminal) if you need
to keep working.

Publishing a port permanently means regenerating with the port included:

    devcontainer-cli agent create --ports 3000:3000

For a real SSH session rather than a `docker exec`:

    devcontainer-cli agent ssh -- go version
    devcontainer-cli agent ssh --via me@docker-host -c mycontainer -- uname -a

Sibling services are reached **inside** the container by compose service name
(`postgres`, `redis`, `mongo`), never `localhost`.

To let an unrelated container talk to the project:

    devcontainer-cli network connect other-app

To route a local domain to a container service via the global Caddy reverse proxy:

    devcontainer-cli route add 3000                  # http://<workspace>-3000.devcli.localhost
    devcontainer-cli route add 8000 --domain api     # http://api.devcli.localhost
    devcontainer-cli route ls                        # list active routes
    devcontainer-cli route status                    # check global Caddy router status

## Inspect

    devcontainer-cli context           # what is installed inside; --json for parsing
    devcontainer-cli status            # compact table (state, ports, image)
    devcontainer-cli info              # ports, mounts, IPs, timestamps
    devcontainer-cli logs -f
    devcontainer-cli logs postgres --tail 100
    devcontainer-cli agent list /home/devuser --json

`context` is the one to read before planning work inside a container: it prints
the container's own `~/CONTEXT.md` plus the live tool inventory with versions,
so you never guess whether a toolchain is there. `agent cli-info` describes the
CLI; `context` describes one running container.

Move files with `agent copy` (`:` marks the container side):

    devcontainer-cli agent copy ./seed.sql :/home/devuser/seed.sql
    devcontainer-cli agent copy :/home/devuser/out.log ./out.log
    devcontainer-cli agent copy --asset install-claude-code

## Clean up

    devcontainer-cli agent clean --dry-run    # what would go
    devcontainer-cli agent clean --yes        # this project only

`agent clean` is irreversible: it brings the stack down with its volumes (data
in them is deleted), removes `.dc_<workspace>/` and `devcontainer.config.json`,
prunes the project's SSH host block, and removes the image built for it. An
image pulled from a registry is left alone — other projects share it.

Confirm with the user before running it, and **always** before
`agent clean --all`, which additionally sweeps every other managed container,
image, network and volume on the machine.

## Beyond the agent group

These have no `agent` spelling; use them when the task calls for it.

Any compose verb the wrappers do not cover, scoped to this project:

    devcontainer-cli compose ps
    devcontainer-cli compose exec -T postgres psql -U devuser -c '\l'

A throwaway container with no project to configure:

    devcontainer-cli run --no-interactive --profile nodejs --name scratch
    devcontainer-cli destroy --container scratch --yes

`run --profile` only accepts a `[remote]`-tagged id (or `ssh` for the hand-built
full image), since it always pulls.

A container on a **different Docker host**, through an existing SSH connection
to it:

    devcontainer-cli shell --via me@docker-host -c dc-ssh -- uname -a
    devcontainer-cli ssh --via me@docker-host --container dc-ssh

Global config, profiles and shared logins:

    devcontainer-cli config                       # current defaults
    devcontainer-cli config profile list          # bundles for --profile; [remote]/[local] tag
    devcontainer-cli config profile info <id>     # one profile's full resolved definition
    devcontainer-cli config skill list            # agent skills for --skill; built-in + user-defined
    devcontainer-cli config skill add <id> --ref owner/repo   # define your own
    devcontainer-cli config alias set ll "ls -la" # aliases for every container
    devcontainer-cli config alias sync            # apply them without a rebuild

`config shared sync` seeds a shared Docker volume from this machine's tool
configs (`~/.claude`, `~/.config/gh`, …) so logins persist across every
container. It copies credentials into a volume — only run it when the user asks
for it, and never as a side effect of another task.

Targeted cleanup across the machine, one category at a time:

    devcontainer-cli clean --dry-run           # preview across all categories
    devcontainer-cli clean containers          # stopped managed containers
    devcontainer-cli clean images
    devcontainer-cli clean all --yes           # sweep everything managed

Every `clean` subcommand touches only resources this CLI created (they carry a
managed label). Use `--dry-run` first and show the user what would go.

## Rules

- **Prefer `agent <verb>`.** Those never prompt. If you drop to a plain command
  that can prompt (generation, `run`, `down`, `destroy`, `clean`,
  `port-forward`), pass `--no-interactive` or it opens a wizard and hangs. Add
  `-y/--yes` for destructive commands — and get the user's agreement first for
  `agent clean`, `destroy`, `down -v` and `clean --all`.
- **Read `agent cli-info` before composing a create.** It is generated from the
  live catalogue, so it never names a module this binary does not have — and it
  includes the user's own profiles and skills, which no document can.
- **Read `agent context` after every create, before working inside.**
  `cli-info` says what this CLI could build; `agent context` says what the
  container actually has. Skipping it is how you end up running the wrong
  Python, guessing a database host, or installing something already there.
- **`bash -lc '…'` only for shell syntax** (`&&`, `|`, `*`, `cd`, `$VAR`) or the
  container's `pip`/`npm` indirections — not to find a binary, which the image
  PATH already handles. `command not found` from `agent exec` on an image built
  before that was in place means the PATH lives only in the rc files: re-run it
  wrapped, check with `context`, and rebuild with `update --rebuild`.
- **Install packages with the container's package manager**, and with the right
  one for the job: `uv add` for a project dependency, `uv tool install` for a
  CLI, `uv pip install` for something importable container-wide, `pnpm` for
  Node. An error telling you to create a venv, pass `--break-system-packages` or
  `apt install python3-xyz` means you used the wrong tool — switch, don't force
  it; the image already sets what `uv pip` needs.
- **Nothing outside the workspace mount and `/home/devuser` survives** a
  recreate. Install-and-forget inside the container is fine for a one-off; a
  toolchain that must persist belongs in `--with` and a re-run of `agent create`.
- **Changing the image is a host-side action.** Adding a module, a service or a
  published port means re-running `agent create` with the new flags — not
  `apt-get install` inside the container.
- Run every command from the project directory; the CLI resolves the workspace
  from the current directory unless `--workspace` says otherwise.
- Long-running commands (`logs -f`, `agent forward`, `agent ssh` without a
  command) do not return. Give them a command, a `--tail`, or run them in the
  background.
- `--help` on any command is authoritative and current; check it before
  inventing a flag.

<!-- devcontainer-cli:managed skill=devcontainer-cli — written by 'devcontainer-cli skill install'; local edits are replaced on reinstall. -->

