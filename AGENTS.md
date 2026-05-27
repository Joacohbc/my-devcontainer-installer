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

- `internal/modules/dockerfile/*.go` — Dockerfile modules (base, nodejs, python,
  java_temurin, java_openjdk, golang, bun, pnpm, sqlite, dbclients, github_cli,
  ai_clis, tmux, cleanup, shell_init).
- `internal/modules/compose/*.go` — compose services (devcontainer, dind_engine,
  mongo, redis, postgres, tunnel).
- `internal/core/registry.go` — the module/service registry.
- `internal/domain/{generator,resolver,validators,config}.go` — generator core.

### What to validate when adding/updating a module

1. **Registry**: module appears in `internal/core/registry.go` and is resolvable
   by id.
2. **Generated output**: the generated Dockerfile / compose contains the expected
   fragments (RUN, FROM, image, env, ports, volumes…). See `domain/generator_test.go`.
3. **Dependencies**: if the module declares `requires`/`conflicts`,
   `domain/resolver_test.go` covers the cases.
4. **Validation**: new options get valid/invalid cases in `domain/validators_test.go`.
5. **CLI flags / prompts**: new flags or prompts are exercised in
   `internal/commands/commands_test.go`.

---

## Architecture overview

`cli/` is split by **responsibility**, not by feature. A layer may import the
layers below it, never above. Everything lives under `internal/` so nothing is
importable from outside the module.

| Path | Why it exists / what belongs here |
|---|---|
| `cmd/devcontainer-cli/main.go` | Process wiring only: signal handlers (SIGINT/SIGTERM → exit 130), `version` var (injected via ldflags), root dispatch, error→exit-code mapping. No business logic. |
| `internal/commands/` | One file per command, each self-registering a `*cobra.Command`. A command *orchestrates* domain + infra; it holds no reusable logic of its own. `root.go` builds the root (the default `generate` command) and attaches subcommands. |
| `internal/domain/` | Business logic: generating Dockerfile/compose, resolving module dependencies, validating input, loading/persisting config, fingerprinting, subnet math. Decides *what* happens; delegates I/O and process spawning to infra. **Never imports `infra`.** |
| `internal/infra/` | The only layer that touches the outside world: running `docker` (`infra/docker`), resolving paths (`infra/project`), console output (`infra/ui`), interactive prompts (`infra/prompt`), embedded asset materialization (`infra/assets`), SSH helpers (`infra/sshdefaults`, `infra/sshinstructions`), container picking (`infra/containerpicker`). |
| `internal/core/` | Pure data shared across layers: `types.go` (`DevcontainerConfig`, `ComposeService`, `BuildMode`, `SCHEMA_VERSION`, `RemoteVariants`, `VariantLabels`, `BuildModes`), `labels.go` (Docker label constants + helpers), `registry.go` (catalogue of modules/services). No I/O, no command-specific logic. |
| `internal/modules/` | The catalogue itself — one file per installable thing: `dockerfile/` (image layers) and `compose/` (services), plus shared render helpers (`helpers.go`, `shell_init.go`). |

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

Import paths name the layer: `internal/core`, `internal/domain`,
`internal/infra/docker`, `internal/commands`.

**Type placement:** a type used by **more than one package** lives in
`core/types.go`; a type used by a **single command** stays local to that file
(e.g. each command's `genFlags`).

### The `CaptureFunc` injection pattern (domain never imports infra)

Domain functions that need to query Docker do **not** import `infra/docker`.
Instead they take a `domain.CaptureFunc func(args []string) (status int, stdout, stderr string)`
parameter, and the command passes a closure wrapping `docker.DockerCapture`
(see `captureFunc()` / `dockerCapture()` in `internal/commands/registry.go`).
This keeps the dependency direction clean and makes domain functions trivially
testable with a fake capture. Examples: `domain.ListUsedSubnets(capture)`,
`domain.LocalImageExists(image, capture)`.

---

## Infrastructure helper packages

Always prefer these over re-implementing the same logic.

### `internal/infra/docker` — Docker execution layer

Centralises **all** shell-outs to the `docker` binary (over `os/exec`).

| Export | Purpose |
|---|---|
| `Runner` | Indirection over `exec.Command`; replace in tests to avoid real Docker. |
| `ResetCache()` | Clears the availability cache — call in test setup/teardown. |
| `IsDockerAvailable()` | Cached check; `false` if docker is absent/daemon down. |
| `EnsureDocker()` | Returns an error if docker is unavailable. Call at the top of any docker-dependent handler. |
| `DockerInherit(args)` | `docker <args>` with inherited stdio; returns exit code. |
| `DockerCapture(args)` | `docker <args>` piped; returns `(status, stdout, stderr)`. |
| `DockerCompose(file, args, env)` | `docker compose -f <file> <args>`; returns exit code. |
| `DockerComposeOrThrow(file, args, env)` | Same but returns an error on non-zero exit. |

**Rule:** never call `exec.Command("docker", …)` directly anywhere else.

### `internal/infra/project` — Project path resolution

| Export | Purpose |
|---|---|
| `ResolveWorkspace(cwd, config)` | `config.Workspace` or sanitized basename of `cwd`. Single source of truth for the fallback. |
| `ProjectPaths(cwd, workspace)` | `{ProjectDir, BuildDir, ComposeFile, DockerfilePath, EnvPath}` for `.dc_<workspace>/`. |
| `ResolveProjectComposeFile(cwd)` | Resolves the compose file and errors if it doesn't exist yet. |

**Rule:** never build `.dc_<workspace>` paths by hand — always call `ProjectPaths`.

### `internal/infra/ui` — Console output

`ui.Log`, `ui.Ok`, `ui.Warn`, `ui.Success`, `ui.Bar` give consistent styling
(over `fatih/color`). Prefer them over bare `color.*` calls for structured
output lines.

### `internal/infra/prompt` — Interactive prompts

Wraps `charmbracelet/huh` (Select/MultiSelect/Input/Confirm). User aborts
(`huh.ErrUserAborted`) are mapped to `prompt.ErrCancelled`, which `main()`
turns into exit code 130. `Input` seeds the result with the default so pressing
Enter keeps it.

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

### `internal/core/labels.go` — Docker label constants

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

func runDown(cmd *cobra.Command, _ []string) error { /* orchestration */ }
```

- `register(c)` appends to the package-level `subcommands` slice consumed by
  `NewRootCommand`.
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
| `setup-ssh` | `setup_ssh.go` | Automated SSH key + config |
| `port-forward` | `port_forward.go` | Forward host ports into the running container |
| `run` | `run.go` | `docker run` from a remote image, no project files |
| `down` | `down.go` | `docker compose down [-v]` |
| `destroy` | `destroy.go` | down + delete `.dc_<ws>/` + config (confirmation) |
| `start`/`stop`/`restart` | `lifecycle.go` | `docker compose start`/`stop`/`restart` |
| `prune` | `prune.go` | Remove orphan `devcontainer-cli/*` images |
| `update` | `update.go` | Pull/rebuild images; `--all`; per-mode dispatch |
| `upgrade-cli` | `upgrade_cli.go` | Binary self-update from a GitHub release |
| `config` | `config.go` | Read/write global config (subcommand `registry`) |
| `cleanup-tips` | `cleanup_tips.go` | Print docker cleanup commands |
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

- **Mocking Docker:** replace `docker.Runner` (and call `docker.ResetCache()`
  before/after) so no real Docker is invoked.
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
3. **Release-asset naming** `devcontainer-cli-<triplet>[.exe]` — shared by
   `cli/.goreleaser.yaml`, `getTargetTriplet()` in `commands/upgrade_cli.go`,
   `cli/install.sh` and `cli/install.ps1`. Change one → change all four.
   Triplets: `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`,
   `windows-x64`. (`amd64`→`x64`; `arm64` stays `arm64`, including native
   `darwin-arm64` — there is no Rosetta fallback.)
4. **Build modes** — see above.
5. **Fingerprint** (`local-cached`) — SHA-256 of normalized Dockerfile +
   copyFile contents + sorted module ids, first 12 chars → `image =
   devcontainer-cli/<fp12>:latest`. Derived from the **Dockerfile**, not the
   compose, so YAML key ordering doesn't affect image sharing.

---

## Release, version injection and self-update

### Release (goreleaser)

`cli/.goreleaser.yaml` produces one raw binary per target (no archives) named
`devcontainer-cli-<triplet>[.exe]` plus a per-binary `<name>.sha256`
(`checksum.split: true`). `.github/workflows/cli-release.yml` runs
`goreleaser release` on `v*` tags. Validate locally with `goreleaser check` and
`goreleaser release --snapshot --clean`.

### Version injection

The version is injected at build time via
`-ldflags "-X main.version={{ .Version }}"` (goreleaser sets `.Version` from the
git tag). The default in `main.go` is `dev`.

### Self-update (`devcontainer-cli upgrade-cli [--check] [--force]`)

In `commands/upgrade_cli.go`: `getTargetTriplet()` derives the triplet,
`fetchLatestRelease()` hits the public GitHub API (honors `GITHUB_TOKEN` for
rate limits), `resolveAssetURL()` matches the asset + its `.sha256`,
`verifyChecksum()` validates with a constant-time compare, and `replaceBinary()`
swaps the file (on Windows it renames the running exe to `.exe.old`;
`CleanupStaleUpdate()` is called on every `main()` to delete it — keep that call).
Only the GitHub host allowlist is fetched (`isAllowedHost`).

Adding a new target: add it to the goreleaser `goos`/`goarch` (and `ignore` if
needed), `getTargetTriplet()`, `install.sh`, `install.ps1` — all at once.

---

## Installer script parity (sh ↔ ps1)

Install/uninstall ship in matched pairs in `cli/`:

- `install.sh` ↔ `install.ps1`
- `uninstall.sh` ↔ `uninstall.ps1`

**Change one in a pair → apply the equivalent change to its counterpart in the
same PR.** Mirror the *behavior*, not the syntax. Releases are raw binaries
(no tar/zip); both installers download the raw `devcontainer-cli-<triplet>[.exe]`.

| Concern | sh (Linux/macOS) | ps1 (Windows) |
|---|---|---|
| Install dir | `$HOME/.local/share/devcontainer-cli` | `$env:LOCALAPPDATA\devcontainer-cli` |
| PATH exposure | symlink in `$BIN_DIR` + rc `export PATH` | entry in user `Path` |
| Completion | zsh/bash via `devcontainer-cli completion …` + managed rc block | not installed |
| Opt-out | `SETUP_COMPLETION=0`, `KEEP_CONFIG=1` | `KEEP_CONFIG=1` |

A feature with no Windows analogue (bash/zsh completion) legitimately skips on
the ps1 side.

---

## PR checklist

- [ ] New/modified module or domain function has a matching `_test.go`.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` and `gofmt -l .` are clean.
- [ ] Installer change mirrored to its `.sh`/`.ps1` counterpart.
- [ ] Frozen contracts above kept in sync where touched.
- [ ] CI green.
