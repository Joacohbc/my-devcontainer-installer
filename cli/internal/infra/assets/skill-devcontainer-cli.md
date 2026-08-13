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

## Orient before acting

    devcontainer-cli --version
    devcontainer-cli status            # this project's containers
    devcontainer-cli status --all      # every managed container on the machine

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

Run the generator from the project directory. **Always pass
`--no-interactive`** — without it the CLI opens a TUI wizard that you cannot
answer:

    cd /path/to/project
    devcontainer-cli --no-interactive --force --with nodejs,pnpm,github-cli

That generates the files, builds the image and starts the stack. Useful flags
(`devcontainer-cli --help` for the full list):

| Flag | Effect |
|---|---|
| `--with a,b,c` | Dockerfile modules (toolchains/tools) to install |
| `--profile <id>` | With mode=custom (default): start from a profile — a module bundle plus its custom scripts. With mode=profiles: the `[remote]`-tagged id to pull instead of building. `config profile list` shows them |
| `--script <path>[:build\|start\|manual]` | Add a script of your own (repeatable): baked into the image, run once per container start, or only copied to `~/post-script/` |
| `--skill firecrawl,agent-browser,webapp-testing` | Agent skills to install **into the project workspace** via the Skills CLI. Adds the internal `skills` module, which requires `nodejs`. `config skill list` shows the full set — built-in plus any the user defined under `~/.devcontainer-cli/skills/` |
| `--skills-mode manual\|auto` | `manual` (default) leaves the `install-skills` command (aliased `install_skills`) to the user; `auto` installs on every container start, writing into the workspace unprompted |
| `--service mongo,postgres,redis` | Add database services to the compose stack |
| `--ports 3000:3000,8080:80` | Publish container ports (bound to 127.0.0.1 unless an IP is given) |
| `--volumes myvol:/data,./cache:/cache` | Extra mounts on the dev container |
| `--workspace <name>` | Override the workspace name (default: directory name) |
| `--mode profiles --profile nodejs` | Skip the local build, pull a prebuilt ghcr.io image for a `[remote]`-tagged profile (see `config profile list`) |
| `--force` | Overwrite existing generated files without asking |
| `--no-build` | Generate only; build/start later with `up --build` |

Module ids for `--with`: `github-cli`, `nodejs`, `pnpm`, `yarn`, `bun`,
`python`, `go`, `rust`, `php`, `c-cpp`, `java-temurin`, `java-openjdk`,
`sqlite`, `postgres-client`, `redis-client`, `mongo-client`, `claude-code`,
`codex-cli`, `copilot-cli`, `opencode`, `antigravity-cli`, `graphify`,
`caveman`, `zellij`, `chrome`, `ffmpeg`, `dod` (Docker-out-of-Docker), `ngrok`,
`cloudflared`. (`base`, `aliases` and `cleanup` are always applied; pick only
one of the two `java-*` modules. The `skills` module is added for you by
`--skill` and is not picked directly.)

Databases are compose **services**, not modules: use `--service postgres`, and
`--with postgres-client` only if you also want `psql` in the dev container.

Re-running the command with different flags regenerates the project; add
`--force` so it does not stop to ask about overwriting.

## Run commands inside

    devcontainer-cli shell --user devuser -- bash -lc 'uv pip install requests'
    devcontainer-cli shell --user devuser -- bash -lc 'cd /workspaces/myapp && pnpm install'
    devcontainer-cli shell -- go test ./...        # exit code propagated
    devcontainer-cli shell -T -- pg_dump -U devuser devdb > dump.sql

`shell -- <cmd>` is a `docker exec` wrapper and the normal way to do work in the
container.

**Wrap the command in `bash -lc '…'` when it needs a shell.** `docker exec` runs
your argv directly — it is not a shell — so without the wrapper there are no
pipes, no `&&`, no globs, no `$VAR` expansion and no `cd`, and the container's
`pip`→`uv pip` and `npm`→`pnpm` indirections (shell functions and aliases) do
not exist either. Use `bash`, not `zsh`: a non-interactive `zsh -lc` skips
`~/.zshrc`.

Finding a binary is a separate matter. The image declares its toolchain PATH in
the image environment, so `node`, `uv`, `pnpm`, `cargo`, `bun`, `go` and
anything in `~/.local/bin` are found by a bare `shell -- <cmd>`. Node resolves
to the version the image was built with; a shell still picks whatever
`fnm`/`nvm` selects for that session, so switching versions works as usual.

The exception is **an image built before this was in place**, which carries the
PATH only in its rc files. If a tool you know is installed comes back as
`executable file not found in $PATH`, re-run it wrapped in `bash -lc` and
rebuild with `devcontainer-cli update --rebuild`.

Other notes that matter:

- With an explicit command it does **not** default to `devuser`; pass
  `--user devuser` when the command must run as that user (it is who owns the
  workspace files, and whose rc files carry the PATH).
- Redirections you write on the host line (`> dump.sql`) are handled by *your*
  shell, so they need no `bash -lc` — but pass `-T` for them, so no TTY mangles
  the stream.
- With no command it opens an interactive login shell — useless to you, so
  always give a command.
- `-c/--container <name>` targets another container in the stack. Database
  containers are plain images with no devuser and often no bash: run those
  commands bare (`-c <ws>-postgres -- psql -U devuser -l`), not through
  `bash -lc`.

The project is mounted at `/workspaces/<workspace>` inside the container
(short alias `/workspace/<workspace>`), which is also the shell's working
directory. Never assume a bare `/workspace`.

For anything compose covers but the wrappers don't:

    devcontainer-cli compose ps
    devcontainer-cli compose exec -T postgres psql -U devuser -c '\l'

### Package managers are not the ones you expect

The image replaces the usual tools, and the replacements live in shell
functions and aliases — so the tool you reach for from `shell --` is often the
wrong one. Run `devcontainer-cli context` to see which of these the image has.

- **Python: `uv pip install X`.** Not `pip install`, and never
  `python3 -m pip install` — the base is Ubuntu 24.04, whose system Python is
  PEP 668 "externally managed", so plain pip refuses with
  `error: externally-managed-environment`. Inside a login shell `pip`/`pip3`
  are *functions* forwarding to `uv pip` (with `UV_SYSTEM_PYTHON=1`), but
  `python3 -m pip` bypasses them and hits the same wall. **Do not create a
  virtualenv and do not pass `--break-system-packages`**: the container is the
  isolation boundary. If the image was built without uv, `pip install --user X`
  is the fallback.
- **Node: call `pnpm` directly.** `npm`/`npx` are *aliases*, and a
  non-interactive shell does not expand aliases — so `bash -lc 'npm install'`
  really runs npm, not pnpm.

      devcontainer-cli shell --user devuser -- bash -lc 'uv pip install requests'
      devcontainer-cli shell --user devuser -- bash -lc 'pnpm add -D vitest'

## Lifecycle

    devcontainer-cli up                # create/recreate and start (detached)
    devcontainer-cli up --build        # rebuild the image first
    devcontainer-cli start | stop | restart
    devcontainer-cli down              # remove containers + network, keep volumes
    devcontainer-cli down -v --yes     # also delete the named volumes (data loss)
    devcontainer-cli destroy --yes     # down -v + delete .dc_<ws>/ and the config

`start`/`stop`/`restart` accept `-c <container>` to act on one container.
`destroy` is irreversible and requires `--yes` in non-interactive mode; confirm
with the user before running it.

    devcontainer-cli update            # rebuild (mode=custom) or pull (mode=profiles)
    devcontainer-cli update --rebuild  # force a rebuild

## Reach services in the container

Ports published at generation time (`--ports`) are reachable on `127.0.0.1`
directly. For a port that was not published, tunnel it instead of regenerating:

    devcontainer-cli port-forward 3000              # 127.0.0.1:3000 -> container:3000
    devcontainer-cli port-forward 8080:80           # 127.0.0.1:8080 -> container:80
    devcontainer-cli port-forward 5432:postgres:5432  # reach a sibling service

`port-forward` tunnels over SSH and stays in the **foreground** until Ctrl+C, so
run it in the background (or in a separate terminal) if you need to keep
working, and pass `--no-interactive` so it never stops to prompt.

Publishing a port permanently means regenerating with the port included:

    devcontainer-cli --no-interactive --force --ports 3000:3000

SSH access to the dev container (a real SSH session; it configures the key and
the Host block on first use):

    devcontainer-cli ssh -- go version
    devcontainer-cli ssh --forward --ports 3000,8080:80

Sibling services are reached **inside** the container by compose service name
(`postgres`, `redis`, `mongo`), never `localhost`.

To let an unrelated container talk to the project:

    devcontainer-cli network connect other-app

## Inspect

    devcontainer-cli context           # what is installed inside; --json for parsing
    devcontainer-cli status            # compact table (state, ports, image)
    devcontainer-cli info              # ports, mounts, IPs, timestamps
    devcontainer-cli logs -f
    devcontainer-cli logs postgres --tail 100
    devcontainer-cli ls -la /home/devuser

`context` is the one to read before planning work inside a container: it prints
the container's own `~/CONTEXT.md` plus the live tool inventory with versions,
so you never guess whether a toolchain is there.

Move files with `copy` (`:` marks the container side):

    devcontainer-cli copy ./seed.sql :/home/devuser/seed.sql
    devcontainer-cli copy :/home/devuser/out.log ./out.log

## Throwaway container, no project files

When there is no project to configure — a scratch environment, a quick
experiment:

    devcontainer-cli run --no-interactive --profile nodejs --name scratch
    devcontainer-cli run --no-interactive --profile python --name scratch-py \
      --volumes work:/work --ports 8000:8000

`run`'s `--profile` only accepts a
`[remote]`-tagged profile id (`config profile list` marks each one `[remote]`
or `[local]`) or `ssh` for the hand-built full image — those map to a
published `ghcr.io/devcontainer-<id>` image. The published ids are `nodejs`,
`bun`, `python`, `go`, `java-temurin`, `node-go`, `node-python`,
`node-java-temurin`, `bun-go`, `bun-python`, `bun-java-temurin`. A `[local]`
profile (e.g. `scraper`, or a user-created one) is rejected here — it has
nothing to pull; build it instead with `devcontainer-cli --profile <id>` on
the default `--mode custom`, which works for every profile.
Tear one down with `devcontainer-cli destroy --container <name> --yes`.

## Global config, profiles, shared logins

    devcontainer-cli config                       # current defaults
    devcontainer-cli config profile list          # bundles for --profile; [remote]/[local] tag
    devcontainer-cli config profile info <id>     # one profile's full resolved definition
    devcontainer-cli config skill list            # agent skills for --skill; built-in + user-defined
    devcontainer-cli config skill info <id>       # one skill's full resolved definition
    devcontainer-cli config skill add <id> --ref owner/repo   # define your own
    devcontainer-cli config skill remove <id>                 # delete a user-defined one
    devcontainer-cli config alias set ll "ls -la" # aliases for every container
    devcontainer-cli config alias sync            # apply them without a rebuild

`config shared sync` seeds a shared Docker volume from this machine's tool
configs (`~/.claude`, `~/.config/gh`, …) so logins persist across every
container. It copies credentials into a volume — only run it when the user asks
for it, and never as a side effect of another task.

## Cleanup

    devcontainer-cli clean --dry-run           # preview across all categories
    devcontainer-cli clean containers          # stopped managed containers
    devcontainer-cli clean images
    devcontainer-cli clean all --yes           # sweep everything managed

Every `clean` subcommand touches only resources this CLI created (they carry a
managed label). Use `--dry-run` first and show the user what would go.

## Rules

- **`--no-interactive` on every command that can prompt** (generation, `run`,
  `down`, `destroy`, `clean`, `port-forward`). Without it a wizard opens and the
  command hangs. Add `-y/--yes` for destructive commands — and get the user's
  agreement first for `destroy`, `down -v` and `clean --all`.
- **`bash -lc '…'` around anything using shell syntax** (`&&`, `|`, `*`, `cd`,
  `$VAR`) or the container's `pip`/`npm` indirections. `command not found` from
  `shell --` usually means the tool is there but the PATH is not — re-run it
  wrapped before concluding anything is missing, and check with `context`.
- **Install packages with the container's package manager** (`uv pip`, `pnpm`),
  not the one the project's docs assume. An error telling you to create a venv,
  pass `--break-system-packages` or `apt install python3-xyz` means you used the
  wrong one — switch, don't force it.
- **Nothing outside the workspace mount and `/home/devuser` survives** a
  recreate. Install-and-forget inside the container is fine for a one-off; a
  toolchain that must persist belongs in `--with` and a regenerate.
- **Changing the image is a host-side action.** Adding a module, a service or a
  published port means re-running generation with `--force`, then
  `up --build` — not `apt-get install` inside the container.
- **Prefer `--mode profiles --profile <v>`** when the user just needs an
  environment fast and a `[remote]`-tagged profile matches (`config profile
  list`): it pulls a prebuilt image instead of building one. A `[local]`
  profile (e.g. `scraper`) has no image to pull — use `--profile <id>` with
  the default `--mode custom` for those.
- Run every command from the project directory; the CLI resolves the workspace
  from the current directory unless `--workspace` says otherwise.
- Long-running commands (`logs -f`, `port-forward`, `ssh` without a command,
  `shell` with no command) do not return. Give them a command, a `--tail`, or
  run them in the background.
- `--help` on any command is authoritative and current; check it before
  inventing a flag.

<!-- devcontainer-cli:managed skill=devcontainer-cli — written by 'devcontainer-cli skill install'; local edits are replaced on reinstall. -->

