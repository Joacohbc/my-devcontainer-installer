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

- `internal/domain/modules/dockerfile/*.go` — Dockerfile modules (base, nodejs, python,
  java_temurin, java_openjdk, golang, bun, pnpm, yarn, sqlite, dbclients, github_cli,
  ai_clis, zellij, cleanup, shell_init).
- `internal/domain/modules/compose/*.go` — compose services (devcontainer, dind_engine,
  mongo, redis, postgres, tunnel).
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
| `internal/infra/` | I/O execution layer. Contains raw subprocess running (`infra/docker`), assets embed extraction (`infra/assets`), SSH key default setup (`infra/sshdefaults`), and low-level project file-path definitions (`infra/project`). **Never imports `domain` or `cli`.** |

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
(shell/logs/status/copy/copy-asset/ls + completions), `config.go` (config/export/import/preset),
`ssh.go` (setup-ssh), `portforward.go`, `network.go` (network connect/disconnect),
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

The `.sh` scripts are embedded with `//go:embed *.sh` (`embed.FS`) and live
**next to** `assets.go` in `internal/infra/assets/` — `go:embed` cannot
reference parent directories, so the scripts must be co-located. `Preflight`,
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
| `setup-ssh` | `setup_ssh.go` | Automated SSH key + config. Each generated `~/.ssh/config` Host block is tagged with a structured managed marker (`# devcontainer-cli:managed v=1 kind=<workspace\|container> ref=<id> alias=<alias>`) — workspace mode keys by the (unique) workspace name, loose `--container` mode by the container name — so `destroy`/`clean-ssh` can find and remove it |
| `clean-ssh` | `clean_ssh.go` | Prune managed `~/.ssh/config` Host blocks whose target no longer exists (destroyed workspace or removed container); lists + confirms, backs up to `config.bak`; `--dry-run` previews, `-y` skips confirmation. Distinct from `destroy` (one project's block) — it sweeps every stale managed block |
| `sync-config` | `sync_config.go` | Seed the shared tool-config volume from host configs (`~/.claude`, `~/.config/gh`, …); `--force` replaces |
| `backup-config` | `backup_config.go` | Back up the shared tool-config volume to a zip file; `-o` sets the destination (default: timestamped file in cwd) |
| `restore-config` | `restore_config.go` | Restore the shared tool-config volume from a zip made by `backup-config`; `--force` replaces existing entries |
| `port-forward` | `port_forward.go` | Forward host ports into the running container |
| `run` | `run.go` | `docker run` from a remote image, no project files |
| `down` | `down.go` | `docker compose down [-v]` |
| `destroy` | `destroy.go` | down + delete `.dc_<ws>/` + config (confirmation) |
| `start`/`stop`/`restart` | `lifecycle.go` | `docker compose start`/`stop`/`restart` |
| `prune` | `prune.go` | Remove managed images+networks+volumes by label; subcommands `prune images`/`network`/`volume`; `--all` (all) vs default (unused) |
| `remove-container` (alias `rm`) | `remove_container.go` | Remove managed containers by label; positional `name...` (tab-completed) removes just those, else `--all` (all) vs default (non-running) |
| `remove-image` (alias `rmi`) | `remove_image.go` | Remove managed images by label; positional `ref...` (tab-completed) removes just those, else `--all` (all) vs default (unused) |
| `update` | `update.go` | Pull/rebuild images; `--all`; per-mode dispatch |
| `upgrade-cli` | `upgrade_cli.go` | Binary self-update from a GitHub release |
| `config` | `config.go` | Read/write global config (subcommands `registry`, `preset`); `config preset` (`preset.go`) lists/creates/copies/removes reusable **module-bundle** presets saved under `~/.devcontainer-cli/presets/` (`remove <id...>` deletes user presets only — built-ins are not removable; tab-completed, `-y` to skip confirmation). A preset is just a list of module ids (no services/ports/volumes/mode); `create` prompts for the id and runs a modules-only wizard (`GenerateService.SelectModules`). The interactive `generate` wizard also offers an optional "start from a user preset" step (local-cached only) that pre-selects the preset's modules before the user continues with services/ports/volumes |
| `cleanup-tips` | `cleanup_tips.go` | Print docker cleanup commands |
| `network` | `network.go` | Attach/detach any container to the workspace network; subcommands `network connect`/`network disconnect <container...>` (tab-completed); `connect` takes `--alias` (extra DNS names; prompted when interactive) |
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

- `local-cached` (default) — full Dockerfile + compose pipeline. Computes a
  fingerprint and sets `config.Image` to `devcontainer-cli/<fp12>:latest`.
  Identical Dockerfiles share one daemon-level image; an existing image skips
  the build.
- `remote` — skips Dockerfile generation (returns nil) and rewrites the
  devcontainer compose service to `image: ghcr.io/<owner>/devcontainer-<variant>:latest`
  with no `build:` key. DB services are still emitted.

Compat shim: `config.go` upgrades legacy `mode: custom`/`standalone` →
`local-cached` on load.

Mode is read in `domain/generator.go`, the generate flow in `root.go`
(preflight/conflict skip + fingerprint fast-path), and `commands/update.go`
(`local-cached` → build, `remote` → pull). Adding a mode means updating all
three plus `BuildModes` and tests.

---

## Contracts that MUST stay in sync

1. **Remote variants** — `core/types.go` `RemoteVariants`/`VariantLabels` ↔
   `.github/workflows/docker-image.yml` `build-variants` matrix. Adding a variant
   = both sides + a `generator_test.go` assertion. `ssh` is the full image from
   `build-base` → `devcontainer-ssh:latest` (not a `build-variants` entry).
2. **Full-image `--with` list** — `docker-image.yml` job `build-base` passes one
   id per Dockerfile module **except** `base`/`cleanup` (auto-applied) and
   `java-openjdk` (mutually exclusive with `java-temurin`; the full image uses
   `java-temurin`). Compose-only modules (`postgres`, `redis`, `mongo`, `tunnel`)
   never go in `--with`. Add a module → append its id here.
2b. **Base cache image** — `docker-image.yml` job `build-base-cache` builds a
   minimal base (no `--preset`/`--with` → always-on `base`+`cleanup` only) and
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
5. **Fingerprint** (`local-cached`) — SHA-256 of normalized Dockerfile +
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
