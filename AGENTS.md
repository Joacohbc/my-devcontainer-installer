# AGENTS.md / GEMINI.md / CLAUDE.md

Guide for agents (human or AI) modifying this repo.

The CLI lives in `cli/` and is written in **Go** (module
`github.com/joacohbc/my-devcontainer-installer/cli`, Go 1.25), built on
[`spf13/cobra`](https://github.com/spf13/cobra). The previous TypeScript/Node
implementation is frozen under `cli-ts-legacy/` for reference only — do not
modify it and do not port changes back to it.

## Mandatory rule: Go toolchain only

All commands run from the `cli/` directory:

- Build: `go build ./...`
- Test: `go test ./...`
- Vet: `go vet ./...`
- Format check: `gofmt -l .` (must print nothing)
- Format fix: `gofmt -w .`
- Run dev: `go run ./cmd/devcontainer-cli [args]`
- Release (local check): `goreleaser check` / `goreleaser release --snapshot --clean`

CI (`.github/workflows/cli-tests.yml`) runs gofmt, vet, build and test on every
push/PR touching `cli/**`. All four must pass. Keep the tree `gofmt`-clean and
`vet`-clean; an unformatted file or a vet finding fails the build.

## Mandatory rule: module changes require tests

Every time a module or domain function is **added**, **modified**, or
**updated**, the matching `_test.go` must reflect it. No code merges without
coverage. Tests use the standard library `testing` package (table-driven where
it fits) — no third-party test framework.

Applies to:

- `internal/domain/modules/dockerfile/*.go` — Dockerfile modules (base, aliases, nodejs, python,
  java_temurin, java_openjdk, golang, bun, pnpm, yarn, sqlite, dbclients, github_cli,
  ai_clis, zellij, cleanup, shell_init).
- `internal/domain/modules/compose/*.go` — compose services (devcontainer, dind_engine,
  mongo, redis, postgres).
- `internal/domain/catalog/catalog.go` — the module/service catalog.
- `internal/domain/{generator,resolver,validators,config,project,images}.go` — generator core.
- `internal/service/*.go` — every service method that adds/changes logic needs a matching
  case in the group's `_test.go` (Docker swapped via `docker.SetRunner`, fake
  `Reporter`/`Prompter`).

### What to validate when adding/updating a module

1. **Catalog**: module appears in `internal/domain/catalog/catalog.go` and is resolvable
   by id.
2. **Generated output**: the generated Dockerfile / compose contains the expected
   fragments (RUN, FROM, image, env, ports, volumes…). See `domain/generator_test.go`.
3. **Dependencies**: if the module declares `requires`/`conflicts`,
   `domain/resolver_test.go` covers the cases.
4. **Validation**: new options get valid/invalid cases in `domain/validators_test.go`.
5. **CLI flags / prompts**: new flags or prompts are exercised in
   `internal/cli/commands/commands_test.go`.
6. **Runtime env vars**: a tool that reads a token/secret at runtime declares it
   in the module's `RequiresEnv` (see below) — never hardcodes it in the
   Dockerfile, which would bake it into a shared image layer.
7. **Build-time env vars**: a PATH entry or a variable the module's own tools
   need is declared in `ProvidesEnv` (see "The container environment is declared
   as data"), never exported from a shell-init file — otherwise it exists only
   for processes that start a shell.

### Module-declared runtime env vars (`RequiresEnv`)

`dockerfile.ModuleSpec.RequiresEnv` is how a tool installed in the image gets a
value the user supplies per project. `domain.CollectRequiredEnvVars` folds
service- and module-declared vars into one deduplicated list, which drives the
generate wizard's prompt steps; answers land in `config.Env` → the generated
`.env`. `domain.devcontainerModuleEnv` then emits one `NAME=${NAME:-}` entry on
the **devcontainer** compose service, so the value reaches the container that
actually has the binary.

Two properties are load-bearing:

- **An empty answer is valid.** The wizard drops empty values, so the var never
  reaches the `.env` and arrives empty in the container (the `${NAME:-}` default
  keeps compose from warning about it). A module must therefore treat "unset" as
  a supported state, and the module's `Context` has to say what that state *is*
  rather than treating it as an error — for `cloudflared`, an empty token still
  leaves the free no-account quick tunnel (`cloudflared tunnel --url …`) and
  `cloudflared tunnel login`.
- **The var is passed, never baked.** It is a compose `environment:` entry, not a
  Dockerfile `ENV`/`ARG`, so the token stays out of the image and out of the
  fingerprint — two projects with different tokens still share one image.

---

## Mandatory rule: command changes require a doc review

Every time a command is **added**, or its **behavior or flags change** (new/
removed/renamed flag, changed default, new subcommand, different interactive vs
non-interactive handling, new positional arg, …), review and update the
documentation that describes it. Help text is part of the command's contract —
out-of-date docs count as a bug. Don't merge a behavior change with stale docs.

What to check on every command change:

1. **`Short` / `Long`**: the one-line summary and the description in
   `internal/cli/commands/<name>.go` still match what the command does. `Long`
   explains *what it does and when to use it* (and how it differs from sibling
   commands) — it does **not** re-list every flag in prose.
2. **Flag descriptions**: each flag is described **once**, in its own
   registration (`cmd.Flags().X(...)` / the shared helpers in `flags.go`). Fold
   any important nuance (a flag only valid with another, a destructive default)
   into that string rather than into `Long`. Cobra renders these in `--help`.
3. **`Example`**: add/adjust the `Example` block so a realistic invocation of the
   new/changed behavior is shown.
4. **Completions**: if a flag's accepted values changed, update its
   `RegisterFlagCompletionFunc` / `ValidArgsFunction`.
5. **The `### Current commands` table** below and the **root help text** when a
   command/subcommand is added, removed, or its one-line purpose changes.
6. **`README` / `install.sh` / other prose** that names the command or flag, when
   touched.

Keep the help consistent with the rest of the tree: imperative, capitalized
`Short` with no trailing period; `Long` and `Example` in the same voice as the
neighboring commands.

---

## Architecture overview

`cli/` is split by **responsibility**, not by feature. A layer may import the
layers below it, never above. Everything lives under `internal/` so nothing is
importable from outside the module.

| Path | Why it exists / what belongs here |
|---|---|
| `cmd/devcontainer-cli/main.go` | Process wiring only: signal handlers (SIGINT/SIGTERM → exit 130), `version` var (injected via ldflags), root dispatch, error→exit-code mapping. No business logic. |
| `internal/cli/` | User interaction layer. Contains subcommands (`cli/commands`), console output + prompts + wizard engine (`cli/ui`), and container picker (`cli/pick`). Commands only parse flags, drive prompts, and print — **they delegate all business logic to `internal/service` and never call `infra/docker` or `os/exec` directly.** |
| `internal/service/` | Orchestration layer between `cli` and `{domain, infra}`. Each service owns one operation group's logic (docker/ssh/exec orchestration + domain calls) and reports/prompts only through the `service.Reporter`/`service.Prompter` interfaces. **May import `domain` and `infra`; never imports `cli`.** See the service-layer section below. |
| `internal/domain/` | Core logic layer: generating Dockerfile/compose, resolving module dependencies, validating input, loading/persisting config, fingerprinting, subnet math, workspace name resolution. **Never imports `infra` or `cli`.** |
| `internal/domain/types/` | Shared domain data types: config structures, constants, label calculations. No I/O, no logic. (Was `internal/core`). |
| `internal/domain/catalog/` | The catalog of modules/services (`catalog.go`). (Was `internal/registry`). |
| `internal/domain/modules/` | Module definitions themselves: `modules/dockerfile/` and `modules/compose/`. |
| `internal/infra/` | I/O execution layer. Contains raw subprocess running (`infra/docker`), the git binary (`infra/git` — the only place in the tree that shells out to it, mirroring how `infra/docker` owns docker), assets embed extraction (`infra/assets`), SSH key default setup (`infra/sshdefaults`), and low-level project file-path definitions (`infra/project`). **Never imports `domain` or `cli`.** |

**Why `cmd/devcontainer-cli/main.go` and not `cli/main.go`:** `cli/` is the
module root — it holds `go.mod` and the release/installer artifacts
(`.goreleaser.yaml`, `install.sh`, …), not Go source for the binary. The
idiomatic Go layout puts each binary's entry point under `cmd/<binary-name>/`,
where the directory name *is* the produced binary (`devcontainer-cli`); this
keeps the root clean and leaves room for additional binaries (`cmd/foo/`,
`cmd/bar/`) without collisions. It also enforces the layering rule: all reusable
code lives in `internal/` (unimportable from outside the module), and `main.go`
is a thin `package main` that only *wires* those internal pieces together — no
business logic to misplace at the root.

Import paths name the layer: `internal/domain/types`, `internal/domain`,
`internal/service`, `internal/infra/docker`, `internal/cli/commands`. The allowed
import direction is `cli → service → { domain, infra }`.

**Type placement:** a type used by **more than one package** lives in
`internal/domain/types/types.go`; a type used by a **single command** stays local to that file
(e.g. each command's `genFlags`).

### The `CaptureFunc` injection pattern (domain never imports infra)

Domain functions that need to query Docker do **not** import `infra/docker`.
Instead they take a `domain.CaptureFunc func(args []string) (status int, stdout, stderr string)`
parameter, and the **service** passes a closure wrapping `docker.DockerCapture`
(see `captureFunc()` / `dockerCapture()` in `internal/service/capture.go`).
This keeps the dependency direction clean and makes domain functions trivially
testable with a fake capture. Examples: `domain.ListUsedSubnets(capture)`,
`domain.LocalImageExists(image, capture)`.

---

## The service layer (`internal/service`)

`cli → service → { domain, infra }`. Commands are a thin shell; every operation's
real logic lives in a service. The dependency rule: `service` may import `domain`
and `infra`, **never `cli`** (no `cli/ui`, no `cli/pick`).

### What goes where

- **Command (`cli/commands/<name>.go`)**: parse cobra flags, resolve cwd/workspace,
  run interactive pickers/prompts, construct the service with `ui.Console{}`, call a
  method, print results. **No `infra/docker`, no `os/exec`** (verified: `grep -rln
  infra/docker internal/cli/commands` is empty).
- **Service (`service/<group>.go`)**: a `struct { Report Reporter; Prompt Prompter }`
  (add `Prompt` only if it asks the user) plus verb methods holding the logic, calling
  `domain.*` and `infra/docker` directly.

### The boundary interfaces (`service/connector.go`)

- `service.Reporter` — output: `Info/Warn/Success/Error/Fatal/Debug`. Services emit
  through it and never touch the terminal.
- `service.Prompter` — input: `Ask/Confirm/Select/Multiselect`, plus
  `Wizard(build func(*State) []Step)` for multi-step flows with esc-back navigation.
- `service.Option`, and the declarative wizard types `Field/Step/State/FieldKind`
  (`service/wizard.go`). The service builds `[]Step`; the **renderer** `Stepper` lives
  in `cli/ui/stepper.go` and backs `ui.Console.Wizard`.
- The single terminal implementation of both interfaces is `ui.Console{}` (`cli/ui`).

### Service groups (one `_test.go` each, mandatory)

`generate.go`+`generate_wizard.go`, `run.go`, `destroy.go`, `update.go`,
`lifecycle.go` (up/down/start/stop/restart), `prune.go`, `inspect.go`
(shell/logs/status/copy/copy-asset/ls + completions), `config.go` (config/export/import/profile),
`ssh.go` (ssh connect + `--setup`/`--setup-external` flow), `portforward.go`, `network.go` (network connect/disconnect),
`upgrade.go`.

**Exceptions that still touch docker from `cli`:** `cli/pick` (container-picker UI) and
the `cleanup-tips` command (pure presentation — it prints docker commands, never runs
them).

### Testing services

No real Docker: swap the runner with `docker.SetRunner(fake)` + `defer
docker.ResetRunner()` (and `docker.ResetDockerCache()`); the injected docker closures
of the past are gone. Use fakes for `Reporter`/`Prompter`. See the shared
`nopReporter`, `fakeRunner`, `scriptedPrompter` and `useFakeDocker` helpers in
`internal/service/helpers_test.go`.

---

## Infrastructure helper packages

Always prefer these over re-implementing the same logic.

### `internal/infra/docker` — Docker execution layer

Centralises **all** shell-outs to the `docker` binary (over `os/exec`).

| Export | Purpose |
|---|---|
| `Runner` / `SetRunner(r)` / `ResetRunner()` | Indirection over `exec.Command`; swap in tests to avoid real Docker. |
| `ResetDockerCache()` | Clears the availability cache — call in test setup/teardown. |
| `IsDockerAvailable()` | Cached check; `false` if docker is absent/daemon down. |
| `EnsureDocker()` | Returns an error if docker is unavailable. Call at the top of any docker-dependent service. |
| `DockerInherit(args)` | `docker <args>` with inherited stdio; returns `(status, error)`. |
| `DockerCapture(args)` | `docker <args>` piped; returns `(status, stdout, stderr, error)`. |
| `DockerCompose(file, args, opts)` | `docker compose -f <file> <args>`; returns `(status, error)`. |
| `DockerComposeOrThrow(file, args, opts)` | Same but returns an error on non-zero exit. |
| `DockerExecStdin(input, args)` | `docker <args>` with `input` piped to stdin; returns `(status, error)`. |

**Rule:** never call `exec.Command("docker", …)` directly anywhere else, and call
these only from `internal/service` — `cli/commands` go through a service.

### `internal/infra/project` — Project path resolution

| Export | Purpose |
|---|---|
| `ProjectPaths(cwd, workspace)` | `{ProjectDir, BuildDir, ComposeFile, DockerfilePath, EnvPath}` for `.dc_<workspace>/`. |

**Rule:** never build `.dc_<workspace>` paths by hand — always call `ProjectPaths`.

### `internal/cli/ui` — Console output, prompts and wizard engine

`ui.Log`, `ui.Ok`, `ui.Warn`, `ui.Success`, `ui.Bar` give consistent styling
(over `fatih/color`). Prefer them over bare `color.*` calls for structured output
lines. `cli/ui` also wraps `charmbracelet/huh` for the one-off prompts
(`Select/Multiselect/Input/Confirm`) and the multi-step `Stepper`; user aborts map to
`ui.ErrCancelled`, which `main()` turns into exit code 130. `ui.Console{}` implements
`service.Reporter` and `service.Prompter`, so commands pass it into services instead of
calling these helpers ad hoc. (There is no separate `cli/prompt` package — it was
folded into `cli/ui`.)

### `internal/infra/assets` — Embedded shell scripts

The `.sh` scripts (plus the `.md` documents baked into the image — currently the
global agent skill) are embedded with `//go:embed *.sh *.md` (`embed.FS`) and
live **next to** `assets.go` in `internal/infra/assets/` — `go:embed` cannot
reference parent directories, so the files must be co-located. `Preflight`,
`AssetExists`, `ValidateRequiredFiles` and `IsGeneratedFile` materialize and
guard them. There is no disk fallback and no separate bundling step — the embed
is always present in the binary.

**Adding a new asset:** drop `internal/infra/assets/<name>.sh`, reference it
from the relevant module's `CopyFiles`, and add a test asserting it lands in the
generated build dir. No config file to update (unlike the old SEA flow).

**Copyable assets (`registry.go`):** the typed `Registry []Asset` marks each
embedded script with `Kind: KindScript`, exposing it as a runtime-copyable asset
via `CopyableAssets()`/`CopyableNames()`/`LookupCopyable(name)`. The
`copy --asset <name>` command (→ `InspectService.CopyAsset`) materializes the
selected script and `docker cp`s it into `types.DevUserHome` (`/home/devuser`),
left owned by devuser and executable. Add a new `.sh` → add a `Registry` entry
(name, file, label) so it's selectable/completable; cover it in `registry_test.go`.
The other three kinds are never copyable: `KindBuild` (build-time scripts) and
`KindDoc` (markdown baked into the image, e.g. `skill-devcontainer-context.md`) —
`docker cp`ing either would only leave a stale copy the next rebuild ignores —
and `KindHostDoc` (markdown installed on the **host**, currently
`skill-devcontainer-cli.md`, the agent skill for driving this CLI), which
describes the host and has no business inside a container at all.

### The container environment is declared as data, not as shell text

A module contributes to the container's environment through
`dockerfile.ModuleSpec.ProvidesEnv func(opts) ContainerEnv` — the build-time
counterpart of `RequiresEnv`, which is asked of the *user* at runtime. It must
never write `export PATH=…` into a shell-init file instead, and the reason is
what the whole design turns on:

**PATH is process environment, not shell ergonomics.** A value that only exists
because a shell sourced a file reaches exactly the consumers that start a
shell — so `devcontainer-cli shell -- uv pip install x` (a bare `docker exec`,
no shell at all) fails with `executable file not found` even though uv is
installed. Declared as data, the generator renders the same value to every
target that needs it:

| target | rendered by | reaches |
|---|---|---|
| `ENV` in the Dockerfile | `RenderDockerfileEnv` | every process, no shell involved |
| `~/.dc-env.sh` | `RenderEnvScript` | shells, incl. `ssh <host> <cmd>` (a non-interactive `zsh -c`) |

Both come from one `MergeContainerEnv` of every resolved module, emitted by
`domain.renderEnvironmentBlocks`. Adding a target later is one more renderer
function; no module changes.

Load-bearing details:

- **`EnvValue` carries the `$HOME` contract.** `ForShell()` keeps `$HOME` (a
  shell expands it); `Expanded()` resolves it to `types.DevUserHome` for targets
  that expand nothing. A renderer that forgets which one it needs is a bug the
  type makes visible. `PathEntry` is an alias of it, named for the use site.
- **The blocks are emitted after every module**, never inside one. The leading
  Dockerfile layers must stay byte-identical across variants for the
  `devcontainer-base` cache to be reused (see the frozen contracts), and an
  `ENV` whose content depends on the selected modules would break that if it
  came earlier. The build steps that need a PATH therefore still export it
  themselves inside their own `su - devuser -c '…'` (pnpm does).
- **`RenderEnvScript` prepends each entry through a `case` guard**, in reverse
  declaration order. The guard is what makes double-sourcing harmless — a login
  bash reads `.profile`, which on Ubuntu also sources `.bashrc`, and both carry
  the source line. The reversal is what makes the resulting order match
  `RenderDockerfileEnv`.
- **`.zshenv` is why `ssh <alias> <command>` works.** sshd runs `$SHELL -c`,
  which for zsh is neither login nor interactive, so `.zshrc`/`.zprofile` are
  never read. It is the only zsh startup file that always is.
- **Aliases and functions never go here.** A process cannot inherit them, so
  they stay in `alias.sh` (see below). `ContainerEnv` has nowhere to put one,
  which is the point.
- **Genuinely dynamic init stays in `Render`.** `eval "$(fnm env)"` and
  `nvm use` resolve per session, so the nodejs module keeps its own
  `~/.nodejs_init.sh` — which yarn's build step also sources.
- **A version manager needs a fixed path invented for it.** Neither fnm nor nvm
  exposes one (fnm resolves the active version from `fnm env`, nvm nests it under
  a version-named directory), so nothing could be declared. The nodejs build
  selects the version and records the resulting bin dir as
  `~/.node-current` (`dockerfile.NodeCurrentLink`), and *that* is the declared
  PATH entry. A shell still wins: the init script prepends whatever fnm/nvm
  selects for the session, so only shell-less callers see the built-in version.

### The two alias layers and `~/CONTEXT.md`

Every container sources two alias files, in this order (both wired up by the
always-on `aliases` module in `modules/dockerfile/aliases.go`, which must stay
directly after `BaseModule` in the catalog — the zsh installer inside
`BaseModule` rewrites `~/.zshrc` from scratch, so an earlier append is lost):

| File | Origin | Changing it |
|---|---|---|
| `~/.devcontainer_aliases.sh` | the `alias.sh` asset, `COPY`d at build time | edits the asset → new fingerprint → image rebuild |
| `~/.alias.sh` | rendered from the CLI config (`GlobalConfig.Aliases`) into the `alias.sh` **shared-config entry** | `config alias set/unset` + `config alias sync` → no rebuild |

The user's file is sourced last, so it always wins. `emitShellSources`
(`shell_init.go`) appends both source lines in one RUN; the user's is *guarded*
(`if [ -r … ]`) because it is created at runtime, not at build time.
Note `emitShellInit` **panics on any single quote**, which is exactly why the
defaults ship as a real `.sh` asset instead of literal lines — `kill_port` is a
shell function and the aliases are single-quoted.

**User aliases live in the CLI config, not in a file the user edits.**
`GlobalConfig.Aliases` (a `name → command` map) is the single source of truth.
`config alias set/unset` (`config_alias.go` → `ConfigService.SetAlias/UnsetAlias`)
write it; `domain.RenderUserAliases` turns it into a POSIX script (sorted,
single-quote-escaped); `config alias sync` (`SharedConfigService.SyncAliases`)
writes that script straight into the volume's `alias.sh` entry via a throwaway
helper container — no host `~/.alias.sh` file is ever read. So `config shared
sync` skips `alias.sh` (it has no host source); `runSyncConfig` drops it and
points at `config alias sync`. Backup/restore still include the entry (they work
volume↔zip). The entrypoint still seeds an empty `~/.alias.sh` when the volume is
opted out of, so sourcing never fails; those aliases just can't be pushed
without a volume.

Defaults in `alias.sh`: `kill_port <port>` (needs `lsof`, installed by
`BaseModule`), `npm`→`pnpm` / `npx`→`pnpm dlx`, `pip`/`pip3`→`uv pip` with
`UV_SYSTEM_PYTHON=1` **and** `UV_BREAK_SYSTEM_PACKAGES=1` (both halves are
needed — see the Python note below), and the agent launchers
(`claude_yolo`/`codex_yolo`/`copilot_yolo`/`agy_yolo`, each running its CLI with
that CLI's skip-permission flag). Every block is `command -v`-guarded so one
file is valid in every image variant. The agent aliases **never shadow the tool
itself** — plain `claude` stays the unmodified CLI, the `_yolo` name is the
opt-in — and they are **unconditional** (no build-time gate, no module option):
a tool that is not installed simply never gets its alias, so the shipped script
is identical in every image.

### Python: `uv pip install` needs two env vars and a `chown`

`uv pip install X` inside the container is a three-part contract, and dropping
any one of them brings back an error that looks like something else entirely.
All three live in `modules/dockerfile/python.go`:

- **`UV_SYSTEM_PYTHON=1`** picks the target: the container's own interpreter
  rather than a venv. The container is already the isolation boundary.
- **`UV_BREAK_SYSTEM_PACKAGES=1`** makes that target usable. Ubuntu 24.04 marks
  its interpreter externally managed (PEP 668), so `--system` **alone** is
  refused with `The interpreter at /usr is externally managed`. This half was
  missing for a long time, which meant `uv pip install` never worked in any
  image — while `install-scraper-tools.sh` carried the workaround
  (`sudo uv pip install --system --break-system-packages`) for its own use.
- **The `chown` in `Render`** is what removes the `sudo`. Past PEP 668 the
  install still has to write into root-owned directories. The paths are asked
  of the interpreter (`sysconfig.get_path("purelib")` / `("scripts")`) instead
  of hardcoded, because Debian uses the `posix_local` scheme — the answer is
  under `/usr/local` and carries the Python version, so it moves with the
  Ubuntu release. It is deliberately **not** recursive: it grants devuser the
  right to create entries there without rewriting the ownership of what other
  modules already put in the scripts dir (zellij drops a binary there).

**`VIRTUAL_ENV` must never be declared image-wide.** It looks like the tidier
fix — a venv in `~/.venv` owned by devuser sidesteps both PEP 668 and the
permissions — but uv does not read `VIRTUAL_ENV` for *project* commands, so
every `uv add`/`uv sync`/`uv run` in every project would print
`VIRTUAL_ENV=… does not match the project environment path … and will be
ignored`. That is permanent noise on the most common workflow, and for an agent
a warning it does not understand is an invitation to "fix" something that works.
`TestGenerateDockerfile_PythonDeclaresNoImageWideVirtualenv` pins it.

The three uv commands are **not** interchangeable, and `~/CONTEXT.md` names all
three (`dockerfile/context_test.go` enforces it) because a document that
mentions only `uv pip install` steers project work at the system interpreter:

| Case | Command | Lands in |
|---|---|---|
| A dependency of one project | `uv add X` / `uv run` | `.venv/` in the project dir |
| A command-line tool | `uv tool install X` | its own env, on PATH via `~/.local/bin` |
| A library importable container-wide | `uv pip install X` | the system interpreter |

### Every project path is per-project — there is no bare `/workspace`

The project is bind-mounted at `types.WorkspaceDir(ws)` = `/workspaces/<ws>`,
and the entrypoint adds a short alias at `types.WorkspaceAlias(ws)` =
`/workspace/<ws>`. `/workspace` is a **directory holding one link per project**,
never a link to the project itself.

That distinction is the whole feature. Agents key their session history by the
directory they were started in, and the shared-config volume makes that history
persist across containers — so a single `/workspace` shared by every project
merges all of their chats into one, which is exactly what the per-project mount
exists to prevent. The short path is the one people actually `cd` into, so
aliasing it bare quietly undoes the split.

Anything that resolves the mount must handle three layouts, in this order:
`/workspaces/<name>` (current), the `/workspace/<name>` alias, and a bare
`/workspace` for images built before the split. `entrypoint.sh`,
`get-devcontainer-context.sh` and `install-codex-cli.sh` each carry the same
glob; keep them identical. The entrypoint also drops a bare `/workspace`
symlink left in the writable layer by an older image before creating the dir.
Covered by `TestEntrypointAliasesWorkspacePerProject` and
`TestEntrypointWorkspaceAliasIsCreated` (which runs the real block).

### The shared-config volume is flat, the home it feeds is not

`devcontainer-shared-config` holds **one entry per `types.SharedConfigEntries`
row, keyed by its id** (`claude`, `agents`, `gh`, …), and the entrypoint
symlinks each one into the home at its `Target` (`~/.claude`, `~/.agents`,
`~/.config/gh`). Those two layouts do not match, and `~/.claude` is itself a
symlink into the flat root — so a **relative** cross-entry link written against
the home layout resolves into the volume root, not the home:

```
~/.claude/skills/x -> ../../.agents/skills/x     # what `npx skills add -g` writes
  physical dir: <volume>/claude/skills/  →  <volume>/.agents/skills/x   ✗ no such name
```

Two mechanisms keep those links alive, and neither may be dropped:

- **Home-shaped aliases at the volume root** — `<volume>/.claude -> claude`,
  `<volume>/.config/gh -> ../gh`, one per entry, derived from `Target`. Created
  by *both* the entrypoint (every container start, so existing volumes are
  repaired without a re-sync) and the sync helper (so a volume seeded before any
  container ever ran is already consistent). They give the relative link above a
  name to land on. Adding an entry needs no extra work — the alias falls out of
  `Target` — but it does need the matching row in `entrypoint.sh`'s table.
- **`fix_symlinks` in `service/sharedconfig.go`** — resolves every symlink of a
  copied entry against the **source tree under `/host`**, never against the copy
  in the volume. Resolving in the destination (what it used to do) can only
  succeed for links that never leave the entry, i.e. the ones needing no help,
  and silently left every cross-entry link dangling in the volume. Its three
  outcomes: target inside another shared entry → stays a **link** (both sides
  must remain one store; a relative one verbatim, an absolute host path
  repointed at `types.DevUserHome`); target outside the shared entries but
  present on the host → replaced with a **real copy**, the only way it survives
  into a container; target dangling on the host too → left alone (never an
  error — `cp -aL` used to abort the whole sync over `~/.claude/debug/latest`).

### Profiles carry modules **and** the user's own scripts

A **profile** (`internal/domain/catalog/profiles.go`, `catalog.Profile`) is the
reusable bundle a project starts from: a list of module ids plus, optionally,
scripts the user wrote. It was called a *preset* until the rename; `--preset`
and `config preset` still work as deprecated aliases, and
`domain.ProfileDirs()` reads the legacy `~/.devcontainer-cli/presets/` after the
current `profiles/`, so an existing installation keeps resolving. Nothing is
ever written to the legacy directory.

Two on-disk shapes, both valid:

| shape | when |
|---|---|
| `profiles/<id>.yml` | a pure module bundle |
| `profiles/<id>/profile.yml` + `*.sh` next to it | it carries scripts |

`Profile.Dir` is what the scripts resolve against, which is why the loader sets
it in both shapes (the containing directory for a flat file, the profile's own
directory otherwise).

**`--profile` is one flag with two roles, told apart by mode.** There used to
be a second flag, `--variant`, pulling double duty with `--profile`; it has
been removed outright (not deprecated — `generate.go`/`run.go` no longer
register it at all). Under mode=`custom`, `--profile <id>` is a module
bundle and always builds locally, for any profile. Under mode=`profiles` (and
always for `run`, which only ever pulls), the same flag instead names the pull
target: it skips the build and pulls `ghcr.io/devcontainer-<id>:latest`, and
that image only exists for the ids CI actually publishes
(`types.RemoteVariants`). `applyProfileBundle` tells the roles apart via
`effectiveBuildMode` (`flags.mode`, falling back to the config's current
mode) — load-bearing, because otherwise a mode=`profiles` regenerate with
`--profile <id>` would misapply the module-bundle branch and silently clear
the project's compose services.

**`ssh` is the one `--profile` value with no catalog entry behind it** — the
hand-built full image, a pull target only. It is therefore the one id that
`parseGenFlags` does not resolve (`remoteVariantSSH`), which leaves two places
that must agree: `applyProfileBundle` only clears the services for a profile
that actually **resolved** (clearing them for an id that
contributed no modules either would drop the project's DB services for
nothing), and `validateConfig` rejects `--profile ssh` outright once the
effective mode turns out not to be `profiles`. Any *other* unresolvable id is
still rejected earlier, at flag-parse time.

**`Profile.Remote` says whether the id is also a pull target.**
`Remote: true` on a `plainBuiltinProfiles` entry marks exactly the ids in
`types.RemoteVariants`; `TestBuiltinProfileRemoteFlagMatchesPublishedVariants`
keeps the two lists in lockstep so a profile can never claim a pull target
that 404s or hide one that works. `base` is deliberately `Remote: false` even
though `ghcr.io/devcontainer-base` exists — that image is the minimal
base-cache build (no github-cli), not this profile's content, so pulling it
under the `base` id would silently serve the wrong image. An embedded profile
that ships scripts (`scraper`) or a user's own is `Remote: false` unless the
user sets `remote: true` in its YAML themselves (for someone publishing their
own matching image under their own registry). `ProfileChoices`
(`service/generate_wizard.go`, backing the `run`/generate "Profile:" pickers)
and `validRemoteVariant`/`profileIDs(remoteOnly)` (`commands/generate.go`)
filter on this flag — `profileIDs(false)`, offering every profile, only backs
`--profile`'s own completion, since mode=`custom` accepts any of them; `run`'s
completion and the mode=`profiles` pull-target validation both use
`profileIDs(true)`/`validRemoteVariant`, remote-only. `config profile list`
renders it as a `[remote]`/`[local]` tag so the choice is visible before the
pull is attempted.

### A repo-shipped profile is data, not a Go literal

The CLI ships profiles from **two** places, and which one a profile belongs in
is decided by one thing: whether it carries files.

| where | for | example |
|---|---|---|
| `catalog.plainBuiltinProfiles` (Go literals in `profiles.go`) | a pure module bundle | `nodejs`, `bun`, `base` |
| `internal/domain/catalog/profiles/<id>/` (embedded) | a profile that ships scripts | `scraper` |

`go:embed` cannot reach out of its own directory, so a profile that references a
`.sh` **cannot** be a Go literal — the file has to sit next to the manifest, and
that directory has to be under the package that embeds it. `BuiltinProfiles` is
the concatenation of both, so nothing downstream knows the difference.

Adding one is a directory drop: `catalog/profiles/<id>/profile.yml` plus its
`.sh` files. `TestEmbeddedProfilesAreWellFormed` then checks it parses, that its
id matches its directory, that every module id it lists exists, and that every
script it names is really in the embedded tree.

Two rules hold it together:

- **`Embedded` marks where `Dir` points.** `catalog.Profile.Embedded` and
  `types.CustomScript.Embedded` say the path is inside `BuiltinProfileFS`, not on
  the host, and `ProfileScripts` joins it with `path` rather than `filepath`
  because an embedded FS is always slash-separated.
- **One reader for both origins.** `domain.ReadCustomScript` is the only way a
  script's bytes are fetched — embedded tree or host file. `prepareBuildDir` and
  `config profile copy` both go through it, which is why copying a repo-shipped
  profile writes its scripts to disk as a normal user profile.

The plain bundles stay Go literals on purpose: their ids are what the CI variant
matrix builds, and one readable table is easier to keep in sync than twelve
files. Keeping the pure module bundles there means a built-in that declares
scripts must be an embedded one — `TestBuiltinProfilesWithScriptsAreEmbedded`
enforces it, since a Go literal has no `Dir` to resolve a script against.

**A profile can also carry ports**, in two lists with two different grammars:

| field | goes to | grammar |
|---|---|---|
| `ports:` | `Compose.Ports`, published by the stack | docker compose — `[IP:][HOST:]CONTAINER` |
| `forward_ports:` | `ForwardPorts`, the tunnels `port-forward` opens with no argument | `port-forward` — `PORT`, `LOCAL:CONTAINER`, `LOCAL:SERVICE:CONTAINER` |

The middle field differs — a host IP in one, a compose **service name** in the
other — so each is checked by the parser that consumes it:
`domain.ValidatePortSpecs` for the published ones, `parsePortMapping` (through
`validateForwardPorts`) for the tunnels. One shared validator would reject
`5432:postgres:5432`, which is valid for a tunnel.

Both accept a bare container port (`3000`), which leaves the host port to Docker
so two projects from one profile can run at once, and an explicit mapping
(`3000:3000`), which is predictable but collides on the second project. The
profile chooses; `BindLoopback` renders either.

**A script declares when it runs** (`types.CustomScript.When`, default `build`),
and each value maps onto machinery that already existed:

| `when` | where it lands | who runs it |
|---|---|---|
| `build` | `/tmp/devcontainer-custom-scripts/` in a build layer | a `RUN` in the generated Dockerfile, as devuser, staging dir deleted in the same layer |
| `start` | `PostScriptStartDir` with a `90-` order prefix | `entrypoint.sh`'s sorted glob, once per container |
| `manual` | `PostScriptDir` | nobody — the user runs it |

The `start`/`manual` buckets are folded into `collectPartitionedPostScripts`, so
they share one COPY with the module-provided post-scripts; only `build` gets its
own block, `customScriptsDockerfileBlock`. `entrypoint.sh` needed no change.

Four properties are load-bearing:

- **The build block is emitted last**, after every module and after
  `renderEnvironmentBlocks`, for the same reason the environment blocks are: the
  leading Dockerfile layers must stay byte-identical across variants or the
  `devcontainer-base` cache stops being reused. It also happens to be what a user
  script wants — the toolchain is already installed.
- **Scripts are resolved and persisted, not referenced.** Applying a profile
  copies each `.sh` into `.dc_<ws>/build/` under a `custom-` prefix
  (`CustomScript.BuildFile()`) and writes the entries into
  `devcontainer.config.json`. `CustomScript.Source` is deliberately **not**
  persisted (`json:"-"`): after the first generate, the build-dir copy is the
  source of truth, so the project still regenerates when the profile has been
  edited or deleted. This mirrors how modules are persisted resolved.
- **They feed the fingerprint.** Materializing happens in `prepareBuildDir`
  *before* `assets.ValidateRequiredFiles` — which looks in the build dir first,
  and `Preflight` skips what is already there — so the scripts flow into
  `copyContents` like any embedded asset. Editing a script therefore changes the
  image, at every `when` (a `start` script is COPYed into the image too).
- **The file name is validated to a shell-safe set**
  (`domain.ValidateCustomScript`, `^[A-Za-z0-9][A-Za-z0-9._-]*\.sh$`, no
  directory part). It is interpolated into a `COPY` and into a single-quoted word
  of the `RUN` that executes it, so a quote or a path separator in it would be an
  injection, not a typo.

Adding a `when` value means: the constant + `types.ScriptWhens`, a branch in
`PartitionCustomScripts`, the rendering for it, and cases in
`domain/profile_test.go` and `domain/generator_custom_scripts_test.go`.

**A custom script is the wrong channel for an agent skill.** Both global agent
skill dirs (`~/.claude/skills`, `~/.agents/skills`) are symlinks into the
shared-config volume, so `npx skills add -g` from a script is wrong at every
`when`: at `build` the volume shadows the write and the skill is invisible, and
at `start` it succeeds but leaks that project's skill into every other container
mounting the volume. Declare skills under `skills:` instead — see the next
section. `TestEmbeddedProfilesDoNotScriptSkillInstalls` keeps repo-shipped
profiles from regressing to a script.

### Agent skills are a catalogue entry, not a script

`internal/domain/modules/skills/` is the third catalogue next to Dockerfile
modules and compose services. A `skills.Spec` is an id, a label, the `Ref` the
Skills CLI resolves (an `owner/repo` shorthand like `firecrawl/cli`, or a full
repository URL), the modules its tooling needs, and a `Context` section.
Adding one is appending to `skills.All`.

**A skill that describes a tool declares the modules installing it.** A skill's
`RequiresModules` names every module its instructions depend on — `webapp-testing`
names `python` and `chrome` — so selecting the skill without them is reported by
`domain.MissingSkillModules` rather than shipping a document about a binary that
is not there.

Skills are **project-scoped**: the installer runs `npx skills add` inside the
workspace mount, so they land in the project and never in the shared volume that
made the script approach wrong.

| piece | where |
|---|---|
| Catalogue | `internal/domain/modules/skills/skills.go`, re-exported as `catalog.AgentSkills` (built-in only) |
| User-defined skills | `catalog/user_skills.go` (`LoadUserSkills`), merged in by `catalog.AllAgentSkills(dirs...)`/`GetAgentSkill(id, dirs...)`/`AgentSkillIDs(dirs...)` |
| Selection + mode | `types.SkillsConfig` on `DevcontainerConfig` (`--skill`, `--skills-mode`, the wizard, a profile's `skills:`/`skills_mode:`) |
| Plumbing | `dockerfile.SkillsModule` — the installer on PATH, the `install_skills` alias, the entrypoint hook |
| Installer | `internal/infra/assets/install-project-skills.sh` (+ `autostart-project-skills.sh`) |

**The picker opens with one yes/no, then asks one category at a time, in pages
of ten.** The gate (`stepKeySkillsEnabled`) comes before any listing: a project
that wants no skills answers it once instead of paging through every category to
select nothing, and declining it is an *answer* — `selectedSkills` returns an
empty config rather than falling back to the profile's or the project's own
skills, which is what lets a regenerate clear them. It defaults to yes only when
something already seeded skills. Past the gate, the catalogue runs to dozens of
entries and a single multiselect that long is unreadable, so
`skillSteps` (`service/generate_wizard.go`) emits one step per page of one
`catalog.SkillGroup` — the skills grouped by their `category:`, in the order
`skills.yml` declares them, with the uncategorized ones (every user-defined
skill, which has no category field) in a trailing `other` group. Three details
carry weight: every page leads with the `skipSkillCategory` sentinel, which
drops **the whole category** rather than the page — so answering it also removes
the category's remaining pages from the wizard and discards what an earlier page
of it had selected; the sentinel is `skip:category`, whose `:` is outside the
charset `domain.ValidateSkillID` accepts, so it can never collide with a skill
id; and the answers live under one key per page (`skillStepKey`), so
`selectedSkills` reads them back through `pickedSkillIDs` instead of a single
step key. Adding a category is a `skills.yml` edit — the wizard needs no change.

**A skill can be user-defined**, the same idea as a user profile: a flat
`~/.devcontainer-cli/skills/<id>.yml` (`domain.SkillDirs()`/`SkillDir()`,
loaded by `catalog.LoadUserSkills`) carrying `id`/`label`/`ref`/`skill`/
`requires_modules`/an optional `context: {title, body}` in place of the
Go-only `Context func()`. No scripts, no directory shape — a skill's install
source is already a git ref the Skills CLI resolves, unlike a profile there
are no files of the user's own to carry alongside it. `config skill
list`/`info`/`add`/`remove` (`domain.ValidateSkillID` guards the id — same
charset as `ValidateProfileID`, it becomes a file name) manage them; `list`
shows built-in vs your own, tagged by group like `config profile list`. Every
site that used to read the package-level `catalog.AgentSkills` var directly
(the wizard's skill picker, `--skill` completion/validation,
`domain.SkillRefs`/`MissingSkillModules`/CONTEXT.md's skill sections) now goes
through the `dirs`-aware functions, passing `domain.SkillDirs()` — mirroring
exactly how `catalog.Resolve`/`catalog.All` take `domain.ProfileDirs()`. A
user-defined id shadows a built-in of the same id.

Four properties are load-bearing:

- **The module is `Internal`, never picked.** `domain.ApplySelectedSkills` adds
  it when the project selects skills, and `dockerfile.ModuleSpec.Internal` (the
  mirror of `compose.ServiceSpec.Internal`) keeps it out of the wizard's
  categories. It is applied **last** in `applyGenFlags`, because `--with`
  rewrites the module list and a derived module has to survive that.
- **Requiring nodejs is how npx is guaranteed.** The module declares
  `Requires: nodejs`, so the resolver pulls the toolchain in rather than the
  installer discovering it is missing inside a container.
- **Which skills and which mode are compose environment, not image content**
  (`DEVCONTAINER_SKILLS`, `DEVCONTAINER_SKILLS_MODE`, from `domain.SkillsEnv`).
  Changing the list is a compose change, so a project does not rebuild its image
  to add a skill. Only the installer itself is baked, and it is static.
- **The mode needs a caller, not a guess.** `manual` (the default) leaves
  `install-skills` (aliased `install_skills`) to the user; `auto` installs on
  every container start. The entrypoint runs start.d scripts with no arguments,
  so `autostart-project-skills.sh` exists to pass `--auto` — the installer must
  not infer intent from how it was launched. The default is `manual` because the
  installer writes into the bind-mounted workspace, which is the user's own
  repository.
- **Skills and their mode are asked outside the full wizard too.** The full
  wizard (`needsPrompts` in `initAndConfigure`) already asks via `skillsStep`;
  a quick `--with`/no-flags interactive `generate` used to skip the question
  entirely unless `--skill` was passed. `initAndConfigure` now calls
  `GenerateService.SelectSkills` there as well, whenever mode ends up `custom`
  and nothing already decided skills (`--skill`, or the applied profile's
  own). It sets `config.Skills` directly rather than `flags.skills` — that is
  what lets an explicit "none" answer clear an existing selection, since
  `applyGenFlags` only overwrites `config.Skills.Skills` when
  `flags.skills`/`profileSkills` is non-empty, and both stay empty here.
- **An entry is one token, and never names a branch.** A repo holding several
  skills is selected with `--skill <name>` (`Spec.Skill`), not by pointing `Ref`
  at the skill's directory URL: that URL carries a branch name, and
  `antibrow/anti-detect-browser-skills` defaults to `master` while
  `anthropics/skills` defaults to `main` — a rename would silently break the
  ref. `Spec.InstallRef()` folds the two into `source#skill` so the entries stay
  space-separated in `DEVCONTAINER_SKILLS`, and the installer splits them back.
  `TestAgentSkillCatalogue` enforces all three: single token, no separator
  inside either half, no `/tree/` in a ref. `firecrawl/cli` ships ten skills, so
  its selector is what keeps an unattended install from taking the other nine.

### `~/CONTEXT.md` is generated per project

`~/CONTEXT.md` is the orientation document an AI agent reads first. It is
**generated**, not a shipped asset: `domain.GenerateContext(config)`
(`internal/domain/context.go`) assembles it from a static preamble plus one
section per resolved Dockerfile module and compose service, so it only ever
describes what this image actually contains.

Each module/service declares its own entry through a `Context` field:

```go
// dockerfile.ModuleSpec
Context func(opts map[string]any) *types.ContextSection
// compose.ServiceSpec — same RenderContext its Render gets
Context func(ctx RenderContext) *types.ContextSection
```

`types.ContextSection{Title, Body}` is plain markdown; the generator writes the
title as an `## ` heading and drops any section with an empty body. **Everything
is resolved at build time**: a section prints the node version that was pinned,
the database password that went into the compose file, the ports that were
published. Return `nil` when the module has nothing to say for those options
(e.g. Java with every version deselected) — that is how a module stays out of
the document instead of announcing an empty toolchain. Bodies are written as
double-quoted Go strings joined by the local `ctxBody(...)` helper, because
markdown code spans use backticks and a raw Go literal cannot contain one.

Plumbing:

- **Adding a module/service** → give it a `Context`, and cover it in the
  package's `context_test.go` (option-dependent branches especially).
  `TestModuleContextSectionsAreNonEmpty` and `TestCatalogContextSectionsAreWellFormed`
  guard the shape.
- The document is written to `paths.ContextPath`
  (`.dc_<ws>/build/CONTEXT.md`) by `prepareBuildDir`, **not** by
  `assets.Preflight` — it is generated, so it is deliberately absent from the
  aliases module's `CopyFiles` (preflight would look for it in the embedded FS
  and report it missing).
- It is folded into `copyContents`, so it **feeds the fingerprint**. This is
  load-bearing: adding a database service changes `CONTEXT.md` but not the
  Dockerfile, and without it two such projects would share one image and one
  would ship the wrong document.
- Remote builds generate nothing (like `GenerateDockerfile`): the prebuilt image
  ships the document it was built with.

`get-devcontainer-context.sh` installs to `~/.local/bin/get-devcontainer-context`
and prints that document plus a *live* inventory (tools + versions, workspace,
reachable services), with `--json` for agents. Anything that can only be known at
runtime belongs there, not in `CONTEXT.md`.

`~/CONTEXT.md` (plus the skill below, which does nothing but point at it) is the
**only** channel the CLI uses to brief agents. The entrypoint must never write
into `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md` or any other agent memory file:
those live in the shared volume, they are the user's, and they are shared across
containers with different toolsets.
`TestEntrypointDoesNotTouchAgentMemoryFiles` enforces this.

### The always-installed `devcontainer-context` skill

Every image ships one global agent skill. It is deliberately thin: it says the
session is inside a `devcontainer-cli` Docker container, tells the agent to read
`~/CONTEXT.md` and run `get-devcontainer-context` first, and lists the handful of
rules that hold in every container (edit only in the workspace mount, anything
outside it and `/home/devuser` is discarded on recreate, passwordless sudo,
no systemd, only published ports are reachable, sibling services by compose
service name, image changes belong on the host). It carries no per-project
detail — that is `CONTEXT.md`'s job, and duplicating it here would go stale.

| Piece | Where |
|---|---|
| Content | `internal/infra/assets/skill-devcontainer-context.md` (embedded, `KindDoc`) |
| Baked into the image | `dockerfile.AliasesModule` → `~/.devcontainer-skills/devcontainer-context/SKILL.md` (constants `SkillsDir`/`ContextSkillName`/`ContextSkillAsset`) |
| Installed for the agents | `entrypoint.sh` symlinks it into `~/.agents/skills/` and `~/.claude/skills/` |

Two rules make this work and must not be "simplified":

- **The image bakes it outside the agents' config dirs.** `~/.claude`, `~/.agents`
  and friends are symlinks into the shared-config volume, so a build-time write
  there is either shadowed at runtime or leaks a stale skill into every other
  container. The Dockerfile only ever writes `~/.devcontainer-skills/`.
- **The entrypoint links, never copies** — and only when nothing real is in the
  way (a user-installed skill of the same name is kept, with a warning). The link
  lives in the volume, its target is the running image, so a rebuild always wins.
  The block runs *after* the shared-config block, or `mkdir -p ~/.claude/skills`
  would create the home dir as a real directory and the volume symlinks would
  never be wired up.

It is in the aliases module's `CopyFiles`, so editing the markdown changes the
fingerprint and rebuilds the image. Covered by `TestContextSkillDocument`,
`TestEntrypointInstallsContextSkill`, `TestContextSkillIsRegisteredAsADoc` and
`TestAliasesModuleBakesSkillOutsideAgentConfigDirs`.

### The host-side `devcontainer-cli` skill

The CLI ships **two** agent skills, and they are for opposite sides of the
boundary. Do not merge them:

| Skill | Reader | How it gets there |
|---|---|---|
| `devcontainer-context` | an agent **inside** a container | baked into the image, linked by `entrypoint.sh` |
| `devcontainer-cli` | an agent **on the host** | `devcontainer-cli skill install`, a plain file drop |

The host skill teaches an assistant on the user's machine to drive this CLI:
generate a project non-interactively, run commands in the container, forward
ports, inspect it, tear it down. It never ends up in an image.

| Piece | Where |
|---|---|
| Content | `internal/infra/assets/skill-devcontainer-cli.md` (embedded, `KindHostDoc`) |
| Published copy | `skills/devcontainer-cli/SKILL.md` at the **repo root** |
| Paths, scopes, state classification | `internal/domain/skill.go` |
| Install / remove / status | `internal/service/skill.go` (`SkillService`) |
| Command | `internal/cli/commands/skill.go` |

**The document is committed twice, on purpose.** The Skills CLI
(`npx skills add <owner/repo>@<skill>`) discovers a skill as `<dir>/SKILL.md`,
so the repo carries `skills/devcontainer-cli/SKILL.md` — that is what a machine
without this binary installs. `go:embed` cannot reach out of its own directory,
so the copy cannot be a symlink either; `TestHostSkillMirrorsTheRepoCopy` keeps
the two byte-identical instead. **Edit the embedded asset, then copy it over the
repo one** — never one alone. Byte equality is load-bearing beyond tidiness: it
is what makes an npx-installed copy classify as `current` rather than `foreign`.
`domain.HostSkillRepo`/`HostSkillRepoPath`/`HostSkillNpxCommand()` are the
single source of that invocation, printed by both `skill` and `skill install`.

Load-bearing details:

- **`KindHostDoc`, not `KindDoc`.** Both stay out of `CopyableAssets()`, but the
  kinds say where the document belongs: `KindDoc` is baked into the image,
  `KindHostDoc` is written on the host. The host skill is deliberately **not** in
  any module's `CopyFiles`, so editing it does not change the fingerprint or
  rebuild any image.
- **It installs into the same two dirs the entrypoint links the in-image skill
  into** — `~/.claude/skills` and `~/.agents/skills` (`domain.HostSkillAgents`).
  Adding an agent is one row there; nothing else changes.
- **The document carries `domain.HostSkillMarker`**, which is the only way
  `ClassifyHostSkill` can tell an older copy of ours (`outdated`, upgraded in
  place) from a skill the user wrote under the same name (`foreign`, never
  touched without `--force`). Never strip the marker from the markdown.
- **`TestHostSkillDocumentCoversTheCatalog`** (in `internal/service`, the only
  layer that may import both `domain/catalog` and `infra/assets`) asserts every
  module id, compose service id and remote variant is named in the document — a
  new module that never reaches the skill leaves the host agent guessing. Add
  the id to the `--with` list in the markdown when you add a module.

### The `agent` group is a facade, not a second CLI

`internal/cli/commands/agent.go` adds one command group aimed at an AI agent
driving this CLI: `agent cli-info | create | connect | exec | forward | copy |
list | clean`. It is **additive** — every command it wraps stays top-level and
unchanged, and the host skill teaches the group first with those as the escape
hatch (`TestAgentGroup_DoesNotReplaceTheHumanCommands` pins that).

Five of the eight are the same handler under a different name: `connect` →
`runSsh`, `exec` → `runShell`, `forward` → `runPortForward`, `copy` → `runCopy`,
`list` → `runLs`. The flags come from helpers extracted out of the human
commands' constructors (`addSshFlags`, `addShellFlags`, `addPortForwardFlags`,
`addCopyFlags`, `addLsFlags`), so each flag — description and completion — is
still registered exactly once. A new flag on `ssh` reaches `agent connect` for
free; adding it to only one of the two is the bug the helpers prevent, and
`TestExtractedFlagHelpers_KeepTheOriginalFlags` guards the extraction.

Four properties are load-bearing:

- **The defaults are the feature.** `agentDefaults` sets a flag only when
  `!Changed(name)`, from a `PreRunE`. That is what makes `--no-interactive=false`
  still win, and what keeps the inversion out of the shared handlers — `runSsh`
  has no idea it is being called by the facade. `agent create` flips
  `force` and `build` the same way, because a create that stops to ask about
  overwriting or building is a create that hangs.
- **`agent exec` requires a command, and runs as devuser.** `shell` with no
  command opens a login shell; for an unattended caller that is not a session
  but a hang, so the facade rejects it in `Args`. The user default is the same
  kind of call, and it deliberately **inverts** `shell`'s: `shell` leaves
  `--user` unset with an explicit command so it keeps working against a
  container that has no devuser (a database), and the container itself runs as
  root — so a command that writes into the workspace leaves **root-owned files
  in the user's real checkout on the host**, which they then cannot edit
  without sudo. The facade takes the other trade: devuser by default, and a
  database container now fails loudly with `unable to find user devuser` until
  `--user` names the one it has. A loud failure beats a silently corrupted
  checkout. `shell` itself is untouched — `TestShell_KeepsItsOwnUserSemantics`
  pins that the two stay different on purpose.
- **`agent create` reuses `runGenerate`, then ups.** It registers the whole
  `addGenerateFlags` set (one source of truth; `parseGenFlags` reads all of it)
  and only hides what does not apply — `--version`, `--preset`,
  `--non-interactive`, `--force-prompt` stay registered but out of `--help`.
  The `up` afterwards passes `build=false`: the image was already built by the
  generate.
- **`agent clean` is project-scoped by default.** `AgentService.Clean` composes
  `DestroyService.Run` (which already takes the containers, network and volumes
  with its `compose down -v`) with `PruneService.CleanImages` for the one thing
  destroy leaves behind: the image built for this project. It is resolved
  *before* the destroy — which drops the catalog entry — and only when
  `domain.IsLocalImage` says it is ours (a pulled image backs every project on
  the same profile) and it is still present locally (`CleanImages` treats an
  unknown ref as an error, and an already-deleted image must not fail the
  cleanup). `--all` adds `CleanAll` on top.

**`agent cli-info` is why the catalogue no longer has to be duplicated in
prose.** `AgentService.Info` reads `catalog.DockerfileModules`,
`catalog.ComposeServices`, `catalog.All(domain.ProfileDirs()...)`,
`catalog.AllAgentSkills(domain.SkillDirs()...)`, `types.ScriptWhens` and
`assets.CopyableAssets()`, so it describes this binary — including the user's
own profiles and skills, which no shipped document can. The service returns
data and the command renders it (text or `--json`), the same split as
`config profile list`. `Selectable` (`!Always && !Internal`) is the field that
matters: it is what an agent may pass to `--with`/`--service`, and it is why
adding an always-on module needs no doc edit here.

### `internal/domain/types/labels.go` — Docker label constants

`LabelNamespace`, `LabelManaged`, `LabelProject`, `LabelVersion`,
`LabelQuickRun`, plus `DockerfileLabelBlock(config)` and `ComposeLabels(config)`.
Single source of truth for labels on managed images/containers.

---

## Command authoring pattern

Each command is `internal/commands/<name>.go`. Commands **self-register** via an
`init()` so `root.go` only has to range over the collected list:

```go
func init() { register(newDownCommand()) }

func newDownCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:          "down",
        Short:        "...",
        SilenceUsage: true,
        RunE:         runDown,
    }
    addYesFlag(cmd)          // -y/--yes (destructive commands)
    addInteractiveFlag(cmd)  // --no-interactive
    return cmd
}

func runDown(cmd *cobra.Command, _ []string) error {
    // read flags, resolve cwd/workspace, then delegate to a service:
    svc := service.LifecycleService{Report: ui.Console{}}
    return svc.Down(composeFile, workspace, removeVolumes)
}
```

- `register(c)` appends to the package-level `subcommands` slice consumed by
  `NewRootCommand`.
- `RunE` orchestrates flags/prompts and **delegates the real work to a service**
  (`service.<Group>Service{Report: ui.Console{}}`). It must not import `infra/docker`
  or `os/exec`. New logic → a service method + its `_test.go`; see the service-layer
  section above.
- Flag helpers live in `flags.go`: `addYesFlag`/`addInteractiveFlag` and the
  readers `yesFlag(cmd)`/`interactiveFlag(cmd)` (interactive = NOT `--no-interactive`).
- The default `generate` command is the **root's** `RunE` (`runGenerate` in
  `root.go`); its flags are added by `addGenerateFlags`.
- Subcommands with children (e.g. `config registry`) attach the child via
  `parent.AddCommand(child)`.
- New subcommands accept only `--no-interactive` (not `--non-interactive`); the
  root accepts both for backwards compatibility.

When registering a new command, the only places to touch are: the new file
(with its `init()`), the root help text if applicable, and a test in
`commands_test.go`. Cobra centralises registration, help, and shell completion,
so there is no separate `COMMAND_SPECS`/completion table to maintain — `completion`
is Cobra-native.

### Current commands

| `argv[0]` | File | Purpose |
|---|---|---|
| _(default)_ | `root.go` (+ `generate_prompts.go`) | Generate Dockerfile + compose + .env |
| `clean` | `clean.go` | Consolidate all cleanup actions under subcommands: `clean catalog` (stale images.json entries), `clean containers` (managed containers, alias `rm`), `clean images` (managed images, alias `rmi`), `clean ssh` (stale SSH config blocks — both `kind=workspace` and `kind=container` — plus any duplicated/orphaned marker `FindOrphanedMarkers` finds regardless of target liveness, plus the host keys they pinned in the managed `known_hosts`, swept by address via `SshService.OrphanKnownHosts`/`ForgetHostKeys` so keys left behind by `destroy` go too; alias `sshs`. A `--via` block's liveness is checked against the daemon named in its marker's `host=`, not the local one (`LiveTargetPredicates`'s `existsRemoteContainer`); when that daemon can't be reached the block is reported *unverified* rather than stale or silently kept — interactive runs can still select it for removal, `--yes` skips it and says how many were skipped; `clean ssh --all` additionally lists every managed block regardless of status via `SshService.ListManagedSSHBlocks`, including currently-alive ones — `--all` only widens what's shown/selectable, `--yes` never auto-removes an alive block), `clean networks` (managed networks), `clean volumes` (managed volumes), and `clean all` (sweep every category). Bare `clean` interactively prompts for categories or sweeps all with `--all`/`-y` |
| `ssh` | `ssh.go` (setup flow in `setup_ssh.go`) | Open a real SSH session into the project's own devcontainer (unlike `shell`'s `docker exec`) **and own the whole SSH lifecycle — this is where `setup-ssh` used to live** (`runSshSetup` in `setup_ssh.go`). It auto-runs the setup flow when no managed alias exists yet for the target (`SshService.ManagedAlias` looks it up by the `kind=workspace`/`kind=container` marker, not by assuming alias==workspace); `--setup` forces that setup to run again (rotate key / repair block) before connecting. **Setup writes** the Host block into the **CLI-owned** SSH config (`~/.ssh/devcontainer-cli.config` by default, `config ssh-config-file`), never the user's `~/.ssh/config` — that file only ever gains one top-of-file `Include` (`SshService.EnsureInclude`; must precede every `Host`/`Match`). Older blocks in `~/.ssh/config` are lifted across by `SshService.MigrateManagedBlocks` (run by `ssh`, `destroy`, `clean ssh`). Alias collisions are checked against **both** files (`FindHostAliasConflict`; likewise `HostIsReferenced`, `AliasHostName`, `ConfigHostTargets`/`OrphanKnownHosts`, completion `bothConfigs`). Each block carries a marker (`# devcontainer-cli:managed v=1 kind=<workspace\|container> ref=<id> alias=<alias> [host=<via>]`) so `destroy`/`clean ssh` find it. Host-key verification is scoped to the CLI-managed `known_hosts` (`sshdefaults.HostKeyOptions`); the container's keys are read out-of-band through `docker exec` and re-pinned (`SshService.PinContainerHostKeys`), so a rebuilt image never trips "REMOTE HOST IDENTIFICATION HAS CHANGED". `--container` switches to loose mode (keyed by `kind=container`). `--via USER@HOST` (requires `--container`) reaches a container on another Docker host through an existing SSH connection (`service.WithHostOverride` sets `DOCKER_HOST=ssh://…`), so only the shared key's **public** half leaves this machine; its block has no static `HostName` — it reuses the `ProxyCommand`/`docker inspect` stanza (`Remote: f.via`), re-resolving the IP each connect. A `--via`/ProxyCommand block verifies host keys under the **alias** (no `HostName`), so both `pinHostKeys` (setup) and `refreshHostKey` (reconnect, via `SshService.ManagedViaHost` + `WithHostOverride`) pin under the alias — pinning under the IP would never be consulted. `--setup-external USER@HOST` is the old `--remote`: run **on** the Docker host, it prints a self-contained snippet (with the PRIVATE key) to paste on the connecting machine and does **not** connect. Before dialing an existing alias (`refreshExistingTarget`) it re-pins host keys, upgrades pre-pinning blocks (`EnsureHostKeyPinning`), and rules out a **stale target** deterministically by name from the owning daemon (`SshService.ContainerLiveness` → `TargetRunning`/`TargetStopped`/`TargetAbsent`, no SSH): an absent container aborts with a "stale alias" error instead of a confusing SSH failure, a stopped one warns. Everything is **anchored by the container name**, so a local block whose container came back on a new IP is rewritten in place (`SetManagedHostName`, old host key forgotten) rather than dialing a dead address — a `--via`/ProxyCommand block needs no such refresh since it re-resolves the IP live. At setup time the real SSH probe (`verifyTarget`/`confirmLiveProbe`) is shown and confirmed first, never run silently. `--forward`/`--ports` open SSH tunnels alongside the session (`PortForwardService.OpenTunnelsDetached`), torn down when it ends |
| `port-forward` | `port_forward.go` | Forward host ports into the running container |
| `run` | `run.go` | `docker run` from a remote image, no project files |
| `down` | `down.go` | `docker compose down [-v]` for the current directory's project |
| `destroy` | `destroy.go` | down + delete `.dc_<ws>/` + config (confirmation). Both modes prune the target's managed SSH state via `DestroyService.removeManagedSSH`: the Host block plus the host keys pinned for the address it dialed (`SshService.ManagedHostName` reads that address *before* the block goes; `HostIsReferenced` — which reads both configs — keeps the key when another block still dials it; `MigrateManagedBlocks` runs first so a block left in `~/.ssh/config` by an older version is still found). `--container` switches to a loose-container mode (`DestroyService.RunContainer`): stop + remove just that container and prune its `kind=container` block — no compose stack/project dir/config involved |
| `start`/`stop`/`restart` | `lifecycle.go` | `docker compose start`/`stop`/`restart` for the current directory's project. All accept `--container` to act on a single container instead of the whole stack (`start`→`StartContainer`, `stop`→`StopContainer`, `restart`→`RestartContainer`; start/stop complete stopped/running names respectively, restart completes any managed one) |
| `compose` (alias `dc`) | `compose.go` | Passthrough to `docker compose` scoped to the current project (`LifecycleService.Passthrough`): forwards every argument verbatim after `-f .dc_<ws>/build/docker-compose.yml --env-file .dc_<ws>/build/.env`. The escape hatch for compose verbs the curated wrappers don't cover (`exec`, `ps`, `top`, `config`, `kill`, `run`, `port`, …), reaching every service in the stack — database services included. Uses `DisableFlagParsing` so flags like `-it` reach compose instead of cobra; a bare invocation or lone `-h`/`--help` prints the command's own help, everything else (including `<verb> --help`) is forwarded. The `--env-file` flag is included only when the generated `.env` exists, otherwise compose falls back to its own autoload. Completion (`completeComposeArgs`, which still fires under `DisableFlagParsing`) offers compose verbs on the first token and the project's compose **service** keys (`ReadComposeServices`) on later tokens — service names, since that's what `docker compose <verb> <service>` takes, not container names |
| `update` | `update.go` | Pull/rebuild images; `--all`; per-mode dispatch |
| `upgrade-cli` | `upgrade_cli.go` | Binary self-update from a GitHub release |
| `config` | `config.go` | Read/write global config (subcommands `registry`, `db-user`, `db-password`, `ssh-key`, `ssh-config-file`, `git-init`, `alias`, `profile`, `skill`, `shared`); `config git-init` (default `false`) opts into `generate` offering to `git init` a project directory that is not a repository yet (`GitRepoService.EnsureRepo` → `infra/git`) — it writes into the user's own directory rather than the CLI's `.dc_<ws>/`, which is why it is off by default and why an interactive run still asks before doing it; `config ssh-config-file` sets where managed SSH Host blocks live (default `~/.ssh/devcontainer-cli.config`) — it is global rather than a per-command flag on purpose, so `ssh`, `destroy` and `clean ssh` can never disagree about which file to read. `config alias` (`config_alias.go`) manages the user's own shell aliases, stored in the CLI config (`GlobalConfig.Aliases`), not a host file: `config alias set <name> <command>` / `config alias unset <name>` edit the map, bare `config alias` lists it, and `config alias sync` renders it into the `alias.sh` shared-config volume entry (`SharedConfigService.SyncAliases`) so every container picks it up without an image rebuild; `config profile` (`profile.go`, aliased `preset`) lists/creates/copies/removes reusable **profiles** (module ids + custom scripts + agent skills) saved under `~/.devcontainer-cli/profiles/` (`remove <id...>` deletes user profiles only — built-ins are not removable; tab-completed, `-y` to skip confirmation); `config profile info <id>` prints one profile's full resolved definition — source/dir, then `yaml.Marshal` of the resolved `catalog.Profile` (`Source`/`Dir`/`Embedded` are `yaml:"-"`, so this is exactly the manifest a project resolving it would read). A profile is a list of module ids plus optional custom scripts and agent skills (no services/ports/volumes/mode); `create` prompts for the id, runs a modules-only wizard (`GenerateService.SelectModules`), collects scripts, then picks skills and their mode (`GenerateService.SelectSkills`). The interactive `generate` wizard's first question offers 3 starting points, not 2: plain `custom` (pick modules by hand), `custom` from a profile (built-in or the user's own — pre-selects its modules and carries its scripts before the user continues with services/ports/volumes), or `profiles` (pull target, see below). The middle one is `modeChoiceCustomFromProfile` in `generate_wizard.go` — still `types.BuildModeCustom` once normalized (`currentMode`), it just also triggers `profileStep`, which plain `custom` skips. `config skill` (`config_skill.go`) lists built-in and user-defined agent skills (`config skill list`, grouped like `config profile list`) — a user one is a flat `~/.devcontainer-cli/skills/<id>.yml`, no scripts/directory shape, since a skill's install source is already a git ref; `config skill info <id>` prints one skill's full definition (source, install ref, required modules, its `~/CONTEXT.md` entry if any) via the local `skillInfoView` — a plain yaml-shaped mirror of `catalog`'s unexported `userSkillManifest`, since `skills.Spec.Context` is a Go func and cannot be marshaled directly, so a built-in and a user-defined skill render identically either way; `config skill add <id> --ref <source>` writes `~/.devcontainer-cli/skills/<id>.yml` from flags (`--ref` required, prompted for when interactive and missing; everything else optional), refusing to clobber an existing *user* file without `--force` — but never refusing to shadow a built-in id, which is the documented, expected way to override one; `config skill remove <id...>` deletes user-defined skills only (same built-ins-are-not-removable rule as `config profile remove`). `config shared` (`config_shared.go`) groups the shared tool-config volume ops — `config shared sync` (`sync_config.go`, seed from host configs `~/.claude`, `~/.config/gh`, …; `--force` replaces), `config shared backup` (`backup_config.go`, zip the volume; `-o` sets the destination, default a timestamped file in cwd), `config shared restore` (`restore_config.go`, load a zip made by `config shared backup`; `--force` replaces existing entries) |
| `cleanup-tips` | `cleanup_tips.go` | Print docker cleanup commands |
| `context` | `context.go` | Print the container's own context and installed tools: runs `get-devcontainer-context` inside it via `InspectService.Context` and streams the output (`--json` for structured output). Falls back to `copy --asset get-devcontainer-context` when the image predates the `aliases` module. Complements `info` (Docker metadata) by reporting what is *inside* the container |
| `skill` | `skill.go` | Install the **host-side** agent skill (`skill-devcontainer-cli.md`) into the host's agent skill dirs, so an assistant running on the user's machine knows how to drive this CLI. Subcommands `skill install` / `skill remove` (alias `uninstall`) / `skill show`; bare `skill` lists every target with its state (`absent`/`current`/`outdated`/`foreign`). `--agent` narrows to one agent (default: all), `--scope global\|project` picks the base dir (home vs cwd). A target holding a file the CLI did not write (no managed marker) is reported `foreign` and never overwritten or deleted without `--force`; `remove` needs `-y` in non-interactive mode. Pure file I/O — no Docker, unlike every other command |
| `network` | `network.go` | Attach/detach any container to the workspace network; subcommands `network connect`/`network disconnect <container...>` (tab-completed); `connect` takes `--alias` (extra DNS names; prompted when interactive) |
| `agent` | `agent.go` (+ `agent_create.go`, `agent_info.go`) | The agent-facing facade: `agent cli-info` (the live catalogue as text or `--json`), `agent create` (generate + build + up), `agent connect`/`exec`/`forward`/`copy`/`list`/`clean`. Additive — every command it wraps stays top-level. See the section below |
| `completion` | _(Cobra built-in)_ | Print shell completion script |

---

## Test authoring patterns

```go
import "testing"

func TestParsePortMapping(t *testing.T) {
    cases := []struct{ in string; want int /* … */ }{ /* … */ }
    for _, c := range cases {
        got, err := parsePortMapping(c.in, "")
        // assert with t.Errorf / t.Fatalf
    }
}
```

- **Mocking Docker:** `docker.SetRunner(fake)` + `defer docker.ResetRunner()` (and
  `docker.ResetDockerCache()`) so no real Docker is invoked.
- **Services:** use the shared `nopReporter`/`fakeRunner`/`scriptedPrompter` and
  `useFakeDocker` helpers in `internal/service/helpers_test.go`.
- **Domain functions:** pass a fake `CaptureFunc` returning canned stdout.
- **Temp config home:** set `XDG_CONFIG_HOME` to a `t.TempDir()` and restore it.

---

## Error handling and exit-code contract

- **User-facing errors:** return an `error` from `RunE`. `main()` prints
  `Error: <msg>` to stderr and exits `1`.
- **User cancellation:** return `prompt.ErrCancelled` (or wrap it). `main()`
  detects it via `errors.Is` and exits `130`.
- **SIGINT / SIGTERM:** handled by `installSignalHandlers()` in `main.go` → exit `130`.
- Do **not** call `os.Exit()` inside command handlers — return an error so
  `main()` applies consistent formatting.
- **A failed step is a failed command.** `saveAndPostProcess` used to downgrade
  a failed `docker compose build` to a warning and return nil, so generation
  exited `0` with no image. A human watching the scroll sees the warning; a
  script and `agent create` see success and go on to use an image that was never
  produced. It returns the error now. Anything similar — a step whose failure
  leaves the command's promise unmet — belongs in the exit code, not only in the
  output.

### Recovery hints must name a command that does not prompt

A message that tells the caller what to run next is the highest-signal
documentation in the tree: it arrives exactly when they are stuck. Eight of them
used to say `Run 'devcontainer-cli'` — the interactive wizard, i.e. the one
command that **hangs a non-interactive caller with no way to answer**. They now
name `up`, `start` or `agent create`, all of which work for a human and a script
alike.

`TestErrorMessagesDoNotPointAtTheWizard` (`commands/recovery_hints_test.go`)
walks the source of `internal/cli/commands` and `internal/service` and fails on
any `fmt.Errorf`/`Report.Warn` line quoting the bare binary name. Write the hint
for the caller who cannot see a prompt.

## Interactive vs non-interactive mode

- `interactive == true` (default) — prompts are shown.
- `--no-interactive` — prompts must NOT be shown; missing required data must
  error out.
- `-y/--yes` — destructive confirmations are skipped.

Destructive pattern:

```go
if !yesFlag(cmd) {
    if !interactiveFlag(cmd) {
        return fmt.Errorf("... pass --yes to confirm in non-interactive mode")
    }
    proceed, err := prompt.Confirm("Are you sure?", false)
    if err != nil { return err }
    if !proceed { color.Yellow("Cancelled."); return nil }
}
```

---

## Build modes

Two `mode` values in `DevcontainerConfig` (constant `BuildModes` in
`core/types.go`):

- `custom` (`types.BuildModeCustom`, default) — full Dockerfile + compose
  pipeline. Computes a fingerprint and sets `config.Image` to
  `devcontainer-cli/<fp12>:latest`. Identical Dockerfiles share one
  daemon-level image; an existing image skips the build.
- `profiles` (`types.BuildModeProfiles`) — skips Dockerfile generation
  (returns nil) and rewrites the devcontainer compose service to
  `image: ghcr.io/<owner>/devcontainer-<variant>:latest` with no `build:` key.
  `--profile` (see "one flag with two roles" below) is a `catalog.Profile`
  with `Remote: true` (`config profile list` tags it `[remote]`) or `ssh` for
  the hand-built full image — not every profile qualifies. DB services are
  still emitted.

Compat shim: `config.go` upgrades legacy `mode: custom`/`standalone`/
`local-cached` → `custom`, and legacy `mode: remote` → `profiles`, on load.
(The pre-rename spellings were literally `local-cached`/`remote`; `custom` was
already an even older synonym for the same full-build mode, so it collides
with nothing.)

Mode is read in `domain/generator.go`, the generate flow in `root.go`
(preflight/conflict skip + fingerprint fast-path), and `commands/update.go`
(`custom` → build, `profiles` → pull). Adding a mode means updating all three
plus `BuildModes` and tests.

---

## Contracts that MUST stay in sync

1. **Remote variants** — `core/types.go` `RemoteVariants` ↔
   `.github/workflows/docker-image.yml` `build-variants` matrix ↔ `Remote: true`
   on the matching `plainBuiltinProfiles` entry in `catalog/profiles.go`. Adding
   a variant = all three sides + a `generator_test.go` assertion (the
   `RemoteVariants`/CI pair) and `TestBuiltinProfileRemoteFlagMatchesPublishedVariants`
   (the `RemoteVariants`/`Remote` pair). `ssh` is the full image from
   `build-base` → `devcontainer-ssh:latest` (not a `build-variants` entry, and
   not a catalog profile — `commands.validRemoteVariant` special-cases it).
2. **Full-image `--with` list** — `docker-image.yml` job `build-base` passes one
   id per Dockerfile module **except** `base`/`aliases`/`cleanup` (auto-applied) and
   `java-openjdk` (mutually exclusive with `java-temurin`; the full image uses
   `java-temurin`). Compose-only services (`postgres`, `redis`, `mongo`) never go
   in `--with`. Add a module → append its id here.
2b. **Base cache image** — `docker-image.yml` job `build-base-cache` builds a
   minimal base (no `--profile`/`--with` → always-on `base`+`cleanup` only) and
   publishes `devcontainer-base:latest` with `cache-to: type=inline`. The
   `build-variants` jobs `needs: build-base-cache` and `cache-from` it so the
   byte-identical Base leading layers are reused. `devcontainer-base` is **not**
   a `RemoteVariant` (don't add it to `types.go` or the variants matrix).
3. **Release-asset naming** `devcontainer-cli-<triplet>` — shared by
   `cli/.goreleaser.yaml`, `getTargetTriplet()` in `commands/upgrade_cli.go`
   and `cli/install.sh`. Change one → change all three.
   Triplets: `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`.
   (`amd64`→`x64`; `arm64` stays `arm64`, including native `darwin-arm64` —
   there is no Rosetta fallback.)
4. **Build modes** — see above.
5. **Fingerprint** (`custom` mode) — SHA-256 of normalized Dockerfile +
   copyFile contents + sorted module ids + sorted build args (the resolved host
   `USER_UID`/`USER_GID`), first 12 chars → `image =
   devcontainer-cli/<fp12>:latest`. Derived from the **Dockerfile**, not the
   compose, so YAML key ordering doesn't affect image sharing. The build args are
   folded in because the Dockerfile's `ARG …=1000` defaults are static, so two
   users with different host ids would otherwise share one image baked for a
   single UID. `ComputeFingerprint` and the compose `build.args` must stay in
   sync (both derive from `config.BuildUID/BuildGID`, resolved in `service.Plan`).

---

## Release, version injection and self-update

### Release (goreleaser)

`cli/.goreleaser.yaml` produces one raw binary per target (no archives) named
`devcontainer-cli-<triplet>` plus a per-binary `<name>.sha256`
(`checksum.split: true`). `.github/workflows/cli-release.yml` runs
`goreleaser release` when you **publish a GitHub Release** (event
`release: published`, from the UI or `gh release create`). Validate locally with
`goreleaser check` and `goreleaser release --snapshot --clean`.

**Pre-releases** — the pre-release checkbox set when publishing the GitHub
Release is the source of truth. GoReleaser always rewrites that flag from the tag
name (`prerelease: auto`), so the release workflow has a final `gh release edit
--prerelease=${{ github.event.release.prerelease }}` step that re-applies the
manual choice. `/releases/latest` excludes pre-releases, and `upgrade-cli` only
offers them as an opt-in (see Self-update below).

### Version injection

The version is injected at build time via
`-ldflags "-X main.version={{ .Version }}"` (goreleaser sets `.Version` from the
git tag). The default in `main.go` is `dev`.

### Self-update (`devcontainer-cli upgrade-cli [--check] [--force] [--pre-release]`)

`commands/upgrade_cli.go` orchestrates; `service/upgrade.go` does the work.
`UpgradeService.UpgradeTargets()` calls `ListReleases()` (the `/releases` list,
honoring `GITHUB_TOKEN` for rate limits) and `pickTargets()` returns the newest
release overall (`top`) and the newest stable (`stable`). The command defaults
to `stable`; when `top` is a newer pre-release build it prompts (or installs
it directly with `--pre-release`, or in `--no-interactive` mode falls back to
`stable` unless `--pre-release` is passed). `Install()` then: `getTargetTriplet()`
derives the triplet, `resolveAssetURL()` matches the asset + its `.sha256`,
`verifyChecksum()` validates with a constant-time compare, and `replaceBinary()`
swaps the file with an atomic rename.
Only the GitHub host allowlist is fetched (`isAllowedHost`).

Adding a new target: add it to the goreleaser `goos`/`goarch` (and `ignore` if
needed), `getTargetTriplet()` and `install.sh` — all at once.

---

## Installer scripts

`cli/install.sh` and `cli/uninstall.sh` are the only installers (Linux/macOS;
there is no Windows support). Releases are raw binaries (no tar/zip); the
installer downloads the raw `devcontainer-cli-<triplet>`.

| Concern | sh (Linux/macOS) |
|---|---|
| Install dir | `$HOME/.local/share/devcontainer-cli` |
| PATH exposure | symlink in `$BIN_DIR` + rc `export PATH` |
| Completion | zsh/bash via `devcontainer-cli completion …` + managed rc block |
| Opt-out | `SETUP_COMPLETION=0`, `KEEP_CONFIG=1` |

---

## PR checklist

- [ ] New/modified module, domain function, or service method has a matching `_test.go`.
- [ ] Added/changed command or flag has its docs reviewed (`Short`/`Long`/`Example`,
      flag descriptions, the `Current commands` table) — see "command changes require a doc review".
- [ ] No `infra/docker` or `os/exec` import in `internal/cli/commands` (logic belongs in a service).
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` and `gofmt -l .` are clean.
- [ ] Frozen contracts above kept in sync where touched.
- [ ] CI green.
