# AGENTS.md / GEMINI.md / CLAUDE.md

Guide for agents (human or AI) modifying this repo.

## Mandatory rule: pnpm only

This repo uses **pnpm** exclusively. Do **not** run `npm` or `yarn` — they
produce a conflicting lockfile and the scripts/CI assume pnpm.

- Install: `pnpm install`
- Test: `cd cli && pnpm test`
- Typecheck: `cd cli && pnpm run typecheck`
- Build: `pnpm run build` / `pnpm run build:binary`
- Run dev: `pnpm dev`

The only committed lockfile is `pnpm-lock.yaml`. If you ever see a
`package-lock.json` or `yarn.lock`, it was added by mistake — delete it.

## Mandatory rule: module changes require tests

Every time a CLI module is **added**, **modified**, or **updated**, tests in `cli/src/__tests__/` must reflect it. No code is merged without matching coverage.

Applies to:

- `cli/src/modules/dockerfile/*.ts` — Dockerfile modules (base, nodejs, python, java, go, bun, pnpm, sqlite, dbclients, github-cli, dod, cleanup, etc.).
- `cli/src/modules/compose/*.ts` — docker-compose services (devcontainer, docker-dind, mongo, redis, postgres, tunnel).
- `cli/src/registry.ts` — module/service registry.
- `cli/src/generator.ts`, `resolver.ts`, `validators.ts`, `cli.ts` — generator core.

## What to validate when adding/updating a module

Before merging, confirm tests cover:

1. **Registry**: module appears in `registry.ts` and is resolvable by id.
2. **Generated output**: generated Dockerfile / docker-compose contains expected fragments (RUN, FROM, image, env, ports, volumes, etc.). See patterns in `generator.test.ts`.
3. **Dependencies**: if module declares `requires` or `conflicts`, `resolver.test.ts` covers the cases.
4. **Validation**: if it introduces new options, `validators.test.ts` covers valid and invalid inputs.
5. **CLI flags / prompts**: if it exposes new flags or prompts, `cli.test.ts` exercises them.

## Required workflow

1. Module change → edit/add test in `cli/src/__tests__/`.
2. Run local: `cd cli && pnpm test && pnpm run typecheck`.
3. CI (`.github/workflows/cli-tests.yml`) runs on every push/PR touching `cli/**`. Must pass before merge.

---

## Architecture overview

The CLI source (`cli/src/`) follows a layered architecture introduced in the refactor. Each layer has a single responsibility and strict import direction (inner layers never import outer ones).

```
┌───────────────────────────────────────────────────────────────┐
│  Entry point  index.ts  — signal handlers + registry dispatch │
├───────────────────────────────────────────────────────────────┤
│  Commands     commands/command.ts (Command interface)         │
│               commands/registry.ts (single source of truth)   │
│               commands/{generate,down,destroy,prune,...}.ts   │
│               each exports a `Command` object (run/help/parse) │
├───────────────────────────────────────────────────────────────┤
│  Domain logic generator.ts · resolver.ts · validators.ts      │
│               preflight.ts · subnet.ts · docker-conflicts.ts  │
│               image-registry.ts · global-config.ts · config.ts│
├───────────────────────────────────────────────────────────────┤
│  Infrastructure helpers (import freely from any layer above)  │
│   docker.ts · project.ts · ui.ts · parse.ts · labels.ts      │
│   prompts.ts · ssh-defaults.ts · ssh-instructions.ts          │
├───────────────────────────────────────────────────────────────┤
│  Pure data  types.ts · registry.ts · version.d.ts            │
└───────────────────────────────────────────────────────────────┘
```

---

## Infrastructure helper modules

These modules were introduced during the refactor. Always prefer them over re-implementing the same logic.

### `cli/src/docker.ts` — Docker execution layer

Centralises **all** `child_process` calls that invoke the `docker` binary.

| Export | Purpose |
|---|---|
| `spawner` | Mutable object wrapping `spawnSync`. Replace in tests to avoid real Docker calls. |
| `resetDockerCache()` | Clears the availability cache — call in test `beforeEach` / `afterEach`. |
| `isDockerAvailable()` | Cached check; returns `false` if docker is absent/daemon down. |
| `ensureDocker()` | Throws if docker is unavailable. Call at the top of any docker-dependent handler. |
| `dockerInherit(args)` | `docker <args>` with `stdio: 'inherit'`; returns exit code. |
| `dockerCapture(args)` | `docker <args>` piped; returns `{ status, stdout, stderr }`. |
| `dockerCompose(file, args, opts?)` | `docker compose -f <file> <args>`; returns exit code. |
| `dockerComposeOrThrow(file, args, opts?)` | Same but throws on non-zero exit. |

**Rules:**
- **Never** call `child_process.spawnSync('docker', …)` directly in any other file.
- In tests, mock via `spawner.spawnSync = (cmd, args, opts) => { … }` and always restore in `finally`.
- `dockerCompose` accepts an optional `env` map that is merged on top of `process.env`.

### `cli/src/project.ts` — Project path resolution

Resolves the canonical paths for a project based on `cwd` + workspace name.

| Export | Purpose |
|---|---|
| `resolveWorkspace(cwd, config?)` | Returns `config.workspace` or `sanitizeDockerName(basename(cwd))`. |
| `projectPaths(cwd, workspace)` | Returns `{ projectDir, buildDir, composeFile, dockerfilePath, envPath }`. |
| `resolveProjectComposeFile(cwd)` | Resolves compose file and throws if it doesn't exist yet. |

**Rules:**
- **Never** build `.dc_<workspace>` paths manually with `path.join`. Always call `projectPaths()`.
- `resolveWorkspace` is the single source of truth for the workspace name fallback logic.

### `cli/src/ui.ts` — Console output

Provides a consistent visual style for all terminal output.

| Export | Usage |
|---|---|
| `ui.log(s)` | Section header / progress step (`==> …` in bold blue). |
| `ui.ok(s)` | Success item (`✓   …` in bold green). |
| `ui.warn(s)` | Warning item (`!   …` in bold yellow). |
| `ui.success(s)` | Final success banner (full bold green line). |
| `ui.bar()` | Horizontal separator (64 gray dashes). |

**Rules:**
- Prefer `ui.*` over bare `chalk.*` calls for structured output.
- Direct `chalk` use is still acceptable for inline colouring within a longer string (e.g. `chalk.gray('...')`), but avoid constructing complete log lines with chalk directly.
- Do **not** define local `log/ok/warn` helpers in subcommand files — they duplicate `ui` functionality.

> **Known inconsistency**: `setup-ssh.ts` defines its own `log/ok/warn` helpers locally. `update-images.ts` imports `ui` but then re-wraps calls in local aliases. When touching either file, migrate to direct `ui.*` calls.

### `cli/src/parse.ts` — Flag parsing utilities

Provides shared flag parsing logic and help-text formatting.

| Export | Purpose |
|---|---|
| `parseCommonFlags(argv)` | Parses `-h/--help`, `-y/--yes`, `--no-interactive`; returns `{ flags, remaining }`. |
| `CommonFlags` | Interface: `{ help, yes, interactive }`. |
| `helpBlock(cmd, desc, usage, flags[])` | Generates a standardised help string. |
| `HelpFlagDoc` | Interface: `{ name, description }`. |

**Rules:**
- When writing a new subcommand, call `parseCommonFlags(argv)` first to extract the three standard flags, then handle command-specific flags from `remaining`.
- Use `helpBlock()` to generate the help string so all commands share the same layout.
- `destroy.ts` is the canonical example of this pattern; see it for reference.

> **Known inconsistencies**:
> - `down.ts`, `prune.ts`, `lifecycle.ts`, `quick-run.ts`, `update-images.ts`, `port-forward.ts`, `setup-ssh.ts`, `self-update.ts` still use hand-rolled switch/case parsers instead of `parseCommonFlags`. When touching those files, migrate them.
> - `cli.ts` accepts both `--no-interactive` and `--non-interactive` as aliases. `down.ts` only accepts `--no-interactive`. `port-forward.ts` accepts both. New subcommands must only support `--no-interactive`.
> - Most subcommands use raw template literals for help text instead of `helpBlock()`. When touching a file, migrate it.

### `cli/src/labels.ts` — Docker label constants

Single source of truth for Docker labels applied to all managed images and containers.

| Export | Purpose |
|---|---|
| `LABEL_NAMESPACE` | Root namespace (`dev.devcontainer-installer`). |
| `LABEL_MANAGED` | `…managed` — marks images created by this CLI. |
| `LABEL_PROJECT` | `…project` — project identifier derived from the image name. |
| `LABEL_VERSION` | `…version` — schema version (`SCHEMA_VERSION`). |
| `LABEL_QUICK_RUN` | `…quick-run` — marks containers started by `devcontainer-cli run`. |
| `dockerfileLabelBlock(config)` | Returns the `LABEL …` Dockerfile stanza. |
| `composeLabels(config)` | Returns a `Record<string, string>` for compose label maps. |

### `cli/src/generate.ts` — generate subcommand (no-arg default)

Orchestrates the full generation flow: load config → interactive prompts → apply flags → validate → write Dockerfile/compose/.env → fingerprint → optional build/pull.

- `buildConfigFromPrompts(base)` — full interactive wizard; called only when `needsPrompts` is true.
- `applyFlags(config, flags)` — merges CLI flags onto the loaded/default config; called before validation.
- `writeOutput(path, content, force)` — writes a file, skipping if it exists and was not auto-generated.
- `maybeOverwrite(path, label, interactive, force)` — prompts the user when a non-generated file would be overwritten.
- `executeBuild / executePull` — thin wrappers over `dockerCompose` for the post-generation build step.

> **Coverage gap**: `generate.ts` has no dedicated test file. As of this writing, `applyFlags`, `writeOutput`, and `maybeOverwrite` are not unit-tested. When making changes to this file, add corresponding tests in `cli/src/__tests__/generate.test.ts`.

---

## Command authoring pattern

Every command lives in `cli/src/commands/<name>.ts` and exposes both the free
functions (`parse`/`help`/`run`, kept for tests and internal reuse) and a
`Command` object that implements the shared interface in
`cli/src/commands/command.ts`. Use `destroy.ts` as the canonical template:

```
cli/src/commands/<name>.ts
  ├── interface <Name>Flags { … }            // include help, interactive; add yes if destructive
  ├── function parse<Name>Flags(argv): <Name>Flags
  ├── function <name>Help(): string
  ├── async function run<Name>(argv): Promise<void>
  │     ├── const flags = parse<Name>Flags(argv)
  │     ├── if (flags.help) { console.log(<name>Help()); return; }
  │     └── // business logic using docker.ts, project.ts, ui.ts
  └── export const <name>Command: Command<<Name>Flags> = {
        name, summary, parse, help, run,   // + optional aliases / subcommands / requiredAssets
      }
```

When registering a new command:

1. Create `cli/src/commands/<name>.ts` exporting a `Command` object.
2. Add it to the `getCommands()` array in `cli/src/commands/registry.ts`. That
   list is the **single source of truth**: `index.ts` dispatches from it and
   `completion.ts` derives its completable command names from it.
3. Add the entry to `helpText()` in `cli/src/cli.ts` (root help text).
4. Add tests in `cli/src/__tests__/<name>.test.ts` (flag parsing + help at minimum).

Subcommands (e.g. `config registry`) are themselves `Command` objects listed in
the parent's `subcommands` array; the parent's `run` delegates via
`findCommand(subcommands, token)`.

The dispatcher validates each command's `requiredAssets` (static files shipped
under `assets/`) before calling `run`, so a missing reference fails fast. The
default command (`generate`) additionally validates every referenced module
script up front via `validateRequiredFiles()` before creating any files.

> Old top-level paths (`cli/src/down.ts`, etc.) remain as one-line re-export
> shims so existing imports/tests keep resolving. New code should import from
> `cli/src/commands/`.

### `setup-ssh.ts` — known outlier

`setup-ssh.ts` is the largest subcommand (≈600 lines) and currently deviates from the infrastructure helpers in three ways:

1. **Direct `child_process.spawnSync`**: uses raw `spawnSync('docker', …)` instead of routing through `docker.ts`. This makes Docker calls in this file **untestable** via `spawner` mocking.
2. **Local `log/ok/warn` helpers**: duplicates `ui.ts` with its own chalk-based helpers.
3. **Local `defaultComposeFile()` / `deriveWorkspace()`**: re-implements logic already in `project.ts` (`resolveProjectComposeFile`, `resolveWorkspace`).

When modifying `setup-ssh.ts`, prefer migrating these towards the shared helpers. Until then, do not replicate these patterns in new files.

### `cleanup-tips`

`cleanup-tips` is a normal command in the registry (`cleanupTipsCommand` in
`commands/cleanup-instructions.ts`). Its `run` loads the config and resolves the
workspace itself before printing per-project instructions.

### Current commands

All handler files live under `cli/src/commands/`.

| argv[0] | Handler file | Purpose |
|---|---|---|
| _(default)_ | `generate.ts` (logic in `cli/src/generate.ts`) | Generate Dockerfile + compose |
| `setup-ssh` | `setup-ssh.ts` | Automated SSH key + config |
| `port-forward` | `port-forward.ts` | Forward host ports into the running container |
| `run` | `quick-run.ts` | `docker run` from remote image, no project files |
| `down` | `down.ts` | `docker compose down [-v]` for current project |
| `destroy` | `destroy.ts` | Compose down + delete `.dc_<ws>/` + config |
| `start` | `lifecycle.ts` | `docker compose start` |
| `stop` | `lifecycle.ts` | `docker compose stop` |
| `restart` | `lifecycle.ts` | `docker compose restart` |
| `prune` | `prune.ts` | Remove orphan `devcontainer-cli/*` images |
| `update` | `update-images.ts` | Pull / rebuild images; `--all` for every tracked project |
| `upgrade-cli` | `self-update.ts` | Binary self-update from GitHub release |
| `config` | `config-cmd.ts` | Read/write global config (subcommand `registry`) |
| `cleanup-tips` | `cleanup-instructions.ts` | Print docker cleanup commands |
| `completion` | `completion-cmd.ts` (logic in `cli/src/completion.ts`) | Print shell completion script |

---

## Test authoring patterns

The project uses the **Node.js built-in test runner** (`node:test` + `node:assert/strict`). No Jest or Vitest.

### Flag parser tests (most common pattern)
```typescript
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parse<Name>Flags, <name>Help } from '@/<name>.js';

test('parse<Name>Flags: defaults', () => {
  const f = parse<Name>Flags([]);
  assert.equal(f.help, false);
  assert.equal(f.interactive, true);
});
test('parse<Name>Flags: --yes', () => {
  assert.equal(parse<Name>Flags(['--yes']).yes, true);
  assert.equal(parse<Name>Flags(['-y']).yes, true);
});
test('parse<Name>Flags: --help', () => {
  assert.equal(parse<Name>Flags(['--help']).help, true);
});
test('parse<Name>Flags: --no-interactive', () => {
  assert.equal(parse<Name>Flags(['--no-interactive']).interactive, false);
});
test('parse<Name>Flags: unknown flag throws', () => {
  assert.throws(() => parse<Name>Flags(['--banana']));
});
```

### Mocking `docker.ts` in tests
```typescript
import { spawner, resetDockerCache } from '@/docker.js';

test('description', () => {
  resetDockerCache();
  const original = spawner.spawnSync;
  spawner.spawnSync = (() => ({ status: 0, stdout: 'mock', stderr: '' })) as any;
  try {
    // ... assertions
  } finally {
    spawner.spawnSync = original;
    resetDockerCache();
  }
});
```

### I/O tests with a temp XDG home
```typescript
function withTempHome<T>(fn: () => T): T {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'dc-cli-test-'));
  const prev = process.env.XDG_CONFIG_HOME;
  process.env.XDG_CONFIG_HOME = tmp;
  try { return fn(); }
  finally {
    if (prev === undefined) delete process.env.XDG_CONFIG_HOME;
    else process.env.XDG_CONFIG_HOME = prev;
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}
```

### Known test coverage gaps

| File | Gap | Priority |
|---|---|---|
| `generate.ts` | No dedicated test file. `applyFlags`, `writeOutput`, `maybeOverwrite` are untested. | High |
| `setup-ssh.ts` | Docker-calling functions untestable (direct `spawnSync`). | High (blocked by R2) |
| `update-images.ts` | `updateOne()` and `updateAll()` untested. | Medium |
| `port-forward.ts` | `runPortForward()` untested. | Medium |

---

## Error handling and exit-code contract

- **User-facing errors**: throw `new Error('…')`. `main()` catches, prints with `chalk.red`, exits `1`.
- **User cancellation**: throw `PromptCancelledError` (from `prompts.ts`) or call `process.exit(130)`. `main()` catches `PromptCancelledError` and exits `130`.
- **SIGINT / SIGTERM**: handled in `installSignalHandlers()` in `index.ts`. Restores stdin raw mode before exiting `130`.
- **Unhandled rejections**: swallowed if `err.code === 'ERR_USE_AFTER_CLOSE'` or `err.message === 'canceled'` (enquirer cancel signal).
- Do **not** call `process.exit()` directly in subcommand handlers — throw instead so `main()` can apply consistent formatting.

---

## Interactive vs non-interactive mode

All subcommands that touch the user (prompts, confirmations) must respect an `interactive` flag:

- `flags.interactive = true` (default) — prompts are shown.
- `flags.interactive = false` (via `--no-interactive`) — prompts must not be shown; missing required data must throw.
- `flags.yes = true` (via `-y/--yes`) — destructive confirmation prompts are skipped (implies the user accepts).

Pattern for a destructive action:

```typescript
if (!flags.yes) {
  if (!flags.interactive) {
    throw new Error('Pass --yes to confirm in non-interactive mode.');
  }
  const proceed = await confirm('actionId', 'Are you sure?', false);
  if (!proceed) { console.log(chalk.yellow('Cancelled.')); return; }
}
```

---

## Known technical debt

These are tracked inconsistencies that do not need to be fixed immediately but **must not be worsened**:

| ID | Location | Issue |
|---|---|---|
| KTD-1 | `setup-ssh.ts` | Uses `child_process.spawnSync` directly instead of `docker.ts` |
| KTD-2 | 9 of 10 subcommands | Hand-rolled flag parsers instead of `parseCommonFlags` |
| KTD-3 | `setup-ssh.ts`, `generate.ts`, `quick-run.ts` | Use raw chalk instead of `ui.*` |
| KTD-4 | `setup-ssh.ts` | Local `deriveWorkspace()` duplicates `resolveWorkspace()` from `project.ts` |
| KTD-5 | `setup-ssh.ts` | Local `defaultComposeFile()` duplicates `projectPaths()` from `project.ts` |
| KTD-6 | `types.ts` | `GENERATED_HEADER` and `GENERATED_HEADER_YAML` are identical strings |
| KTD-7 | `docker-conflicts.ts` | Local `dockerAvailable()` is a no-op wrapper around `isDockerAvailable()` |
| KTD-8 | `update-images.ts` | Wraps `ui.*` in local `log/ok/warn` aliases; import placed mid-file |

---

## Build modes

The CLI supports two `mode` values in `DevcontainerConfig`. See DOC_CLI.md → "Modos de Build" for the user-facing summary. For agents modifying this code:

- `mode: 'local-cached'` (default) — goes through the full Dockerfile + compose generation pipeline. Computes a fingerprint (SHA-256 of normalized Dockerfile + copyFile contents + sorted module IDs, first 12 chars) and sets `config.image` to `devcontainer-cli/<fp12>:latest`. Multiple projects with identical Dockerfiles share a single daemon-level image; if the image already exists the build is skipped.
- `mode: 'remote'` — skips Dockerfile generation (returns `null`) and rewrites the devcontainer compose service to use `image: ghcr.io/<owner>/devcontainer-<variant>:latest` with no `build:` key. DB services are still emitted normally.

**Compat shim**: `config.ts` silently upgrades legacy `mode: 'custom'` or `mode: 'standalone'` values to `'local-cached'` on load. No other code needs to handle those values.

Where the mode is read:

- `cli/src/generator.ts` — `generateDockerfile` returns `null` for `remote`; `generateCompose` omits `build:` and sets the ghcr image for `remote`.
- `cli/src/index.ts` — `main()` skips preflight and conflict checks for `remote`; fingerprint fast-path for `local-cached`.
- `cli/src/update-images.ts` — per-mode dispatch: `local-cached` → `docker compose build`; `remote` → `docker pull`.

When adding a new mode, update **all three** files plus tests, plus the `BUILD_MODES` constant in `cli/src/types.ts`.

## Remote image variants (must stay in sync with the workflow)

The `remote` mode (and `devcontainer-cli run`) serve pre-built images published by `.github/workflows/docker-image.yml`. The variant list is duplicated:

1. `cli/src/types.ts` → `REMOTE_VARIANTS` constant and `VARIANT_LABELS` map.
2. `.github/workflows/docker-image.yml` → `build-variants.matrix.include[].name`.

When adding a variant: update **both** locations + add a test in `generator.test.ts` (`resolveRemoteImage`). Otherwise users will be unable to select the new image via `--variant` or `devcontainer-cli run`.

Exclusion: `ssh` is the full image from `build-base` (not from `build-variants`) — it maps to `devcontainer-ssh:latest`.

## Global config and image registry

- `cli/src/global-config.ts` — reads/writes `~/.config/devcontainer-cli/config.json` (XDG; `%APPDATA%` on Windows). Stores `registry` (default `ghcr.io/joacohbc/`). Accessed via `devcontainer-cli config registry`.
- `cli/src/image-registry.ts` — reads/writes `~/.config/devcontainer-cli/images.json`. Tracks every project the CLI has generated or pulled for, enabling `devcontainer-cli update --all` and `devcontainer-cli prune`. Also contains `computeFingerprint()` used by `local-cached` mode.

## Devcontainer-ssh full image `--with` list

`.github/workflows/docker-image.yml` job `build-base` builds `ghcr.io/<owner>/devcontainer-ssh:latest` with **every dockerfile module**. When a new module is added under `cli/src/modules/dockerfile/`, it **must** be appended to the `--with` flag on that job (line ~70).

Exclusions:

- `base`, `cleanup` — applied automatically, never passed in `--with`.
- `java-openjdk` — mutually exclusive with `java-temurin`. The full image uses `java-temurin`; do not add `java-openjdk` alongside it.

Compose-only modules (`postgres`, `redis`, `mongo`, `tunnel`) do not belong in `--with` — they are services, not image layers.

When adding a module: update the `--with` list in `build-base`, and consider whether it also fits any matrix entry in `build-variants`.

## Version updates (Node, Java, Go, etc.)

When the version installed by a module changes (e.g. Node 22 → 24, Java 17 → 21, Ubuntu 22.04 → 24.04):

- Update the matching assert in `generator.test.ts` (e.g. `temurin-17-jdk` → `temurin-21-jdk`, `FROM ubuntu:22.04` → `FROM ubuntu:24.04`).
- If the version is parameterizable, add a test for the new default and for an explicit override.

## Docker access & sandboxing (`dockerSocket` modes)

The devcontainer service exposes a `dockerSocket` option with three modes. The
prompt is **gated on the `dod` Dockerfile module** (`requiresModule: 'dod'` on
the option in `cli/src/modules/compose/devcontainer.ts`) — without the `docker`
CLI installed, asking about Docker access is meaningless, so it is not shown and
defaults to `none`.

- `none` (default) — no Docker access.
- `socket` — mounts the host `/var/run/docker.sock` (classic Docker-outside-of-
  Docker). Full control of the host daemon → **root-equivalent on the host**.
  Opt-in only; clearly labeled with a warning.
- `dind` — **isolated rootless Docker-in-Docker** sidecar (`docker-dind`,
  `cli/src/modules/compose/dind-engine.ts`). Workloads run against an isolated
  daemon over `tcp://docker-dind:2375`; the host socket is never mounted. The
  engine sits on a dedicated `*-engine-network` bridge (declared in
  `generator.ts` only when used) joined solely by the devcontainer and the
  engine — keep DB/other sidecars off it.

The `dockerSocket` mode is the **single source of truth** for the dind engine.
`docker-dind` is marked `internal: true`, so it is never offered in the service
picker; instead `resolveEnabledServices` in `generator.ts` auto-provisions it
whenever the mode is `dind`. There is no separate "enable the engine" step and
no fallback — selecting `dind` always brings the engine.

Invariants to preserve when touching these:
- Never auto-mount the host socket: only `socket` mode mounts it, and only as an
  explicit, warned opt-in. `generate.ts` warns (and prompts to confirm when
  interactive) if `socket` is chosen but `/var/run/docker.sock` is missing.
- Keep `docker-dind` `internal` and auto-provisioned from the mode — do not turn
  it back into a separately-selectable service.
- The dind engine image is **pinned** (`docker:NN-dind-rootless`), not `:latest`.
- `docker-dind` is `privileged: true` (needs cgroups/overlayfs/netns); that is
  expected — rootless contains the blast radius to a user namespace.

Any change here needs matching asserts in `generator.test.ts` (wiring, network
segmentation, socket mount, auto-provisioning).

## PR checklist

- [ ] New/modified module has test in `cli/src/__tests__/`.
- [ ] `pnpm test` passes locally.
- [ ] `pnpm run typecheck` passes.
- [ ] Installer script change mirrored to its `.sh`/`.ps1` counterpart (see "Installer script parity").
- [ ] CI green.

## Self-contained binary (SEA assets bundling)

The CLI ships as a single self-contained Node SEA binary. Shell scripts under `cli/assets/` are **embedded inside the binary** via the `node:sea` Asset API — they are not shipped alongside the binary.

### Adding a new asset (shell script in `cli/assets/`)

When you add a file under `cli/assets/`, you **must** also register it in `cli/sea-config.json` under the `assets` map. If you forget, `preflight()` will report it as missing at runtime when running from the SEA binary (but it will still work in `pnpm dev` because of the disk fallback — easy to miss).

1. Drop the file in `cli/assets/<your-script>.sh`.
2. Add the entry to `cli/sea-config.json`:
   ```json
   "assets": {
     "...": "...",
     "<your-script>.sh": "assets/<your-script>.sh"
   }
   ```
3. Reference it from the relevant module's `copyFiles` array (e.g. `cli/src/modules/dockerfile/<module>.ts`).
4. Rebuild: `pnpm run build:binary`. Smoke test in an empty dir to confirm the script is materialized.

### How asset resolution works

`cli/src/preflight.ts`:
- If `node:sea.isSea()` is true → reads the script via `sea.getAsset(name, 'utf8')` and writes it to cwd.
- Otherwise (dev runs via `pnpm dev`, `pnpm start`) → falls back to reading from `cli/assets/` on disk.

This means: **never delete the dev disk fallback** without a replacement — `pnpm dev` would break for everyone.

## Self-update flow (`devcontainer-cli upgrade-cli`)

The CLI has a built-in binary updater: `devcontainer-cli upgrade-cli [--check] [--force]`. Implementation lives in `cli/src/self-update.ts`.

> Historical note: the binary self-update used to be exposed as `devcontainer-cli update`. That command now manages container images via `cli/src/update-images.ts`. Self-update was renamed to `upgrade-cli` to free up the namespace. No alias is kept.

### Version injection

The version is injected at build time via esbuild `define: { __CLI_VERSION__ }`:
- `cli/build.mjs` and `cli/build-sea.mjs` read `process.env.VERSION || package.json.version || 'dev'`.
- In CI, `.github/workflows/cli-release.yml` sets `VERSION: ${{ github.ref_name }}` on the build job so the binary records the tag (e.g. `v1.2.3`).

When bumping the CLI version, update `cli/package.json` `version` field. The release tag passed to `git tag` should match.

### Release-asset naming contract

`runSelfUpdate` downloads `devcontainer-cli-<triplet>[.exe]` from the GitHub release. Keep these in sync:

- **Workflow**: `.github/workflows/cli-release.yml` matrix renames each binary to `devcontainer-cli-<triplet>${ext}` before upload.
- **self-update.ts**: `getTargetTriplet()` returns the triplet (`linux-x64`, `linux-arm64`, `darwin-x64`, `windows-x64`) and the ext (`.exe` on Windows).
- **install.sh / install.ps1**: download the same raw filename, no archive.

If you change the artifact naming, you **must** update all four (workflow + self-update.ts + install.sh + install.ps1) at once.

### Adding a new target (e.g. `darwin-arm64` native)

1. Add a matrix entry in `cli-release.yml` with the new `target` value.
2. Add the triplet to the branch logic in `getTargetTriplet()` (`cli/src/self-update.ts`).
3. Add the case to `install.sh` (the `uname_m` switch) and remove the Rosetta workaround for Apple Silicon if applicable.
4. Bump the CLI version and tag.

### Windows in-use replacement

On Windows you cannot overwrite a running `.exe`, but you _can_ rename it. `replaceBinary()` renames the current binary to `<exe>.old`, then moves the downloaded file into place. `cleanupStaleUpdate()` is called on every `main()` to delete the `.old` from the previous run. **Don't remove that cleanup call** — it leaks otherwise.

### Rate limits

`fetchLatestRelease()` hits the public GitHub API (60 req/hour anonymous). If a user reports throttling, advise setting `GITHUB_TOKEN` — it's already honored.

## Release format

Release artifacts are **raw binaries** (one per target), no tar/zip. The installer scripts (`cli/install.sh`, `cli/install.ps1`) and the `upgrade-cli` subcommand both expect this format.

Do not reintroduce archive packaging unless you also update both installers and `self-update.ts` consistently.

## Installer script parity (sh ↔ ps1)

The install/uninstall scripts ship in matched pairs — one POSIX shell (Linux/macOS), one PowerShell (Windows):

- `cli/install.sh` ↔ `cli/install.ps1`
- `cli/uninstall.sh` ↔ `cli/uninstall.ps1`

**When you change one script in a pair, you must apply the equivalent change to its counterpart in the same PR.** Examples: a new env override (`INSTALL_DIR`, `KEEP_CONFIG`, …), changed install layout, new PATH/rc/completion handling, changed asset naming.

Platform differences are expected — mirror the *behavior*, not the syntax:

| Concern | sh (Linux/macOS) | ps1 (Windows) |
|---|---|---|
| Install dir | `$HOME/.local/share/devcontainer-cli` | `$env:LOCALAPPDATA\devcontainer-cli` |
| PATH exposure | symlink in `$BIN_DIR` + rc `export PATH` | entry in user `Path` env var |
| Shell completion | zsh/bash files + managed rc block | not installed (no PowerShell completion yet) |
| Opt-out flags | `SETUP_COMPLETION=0`, `KEEP_CONFIG=1` | `KEEP_CONFIG=1` |

If a feature genuinely has no Windows analogue (e.g. bash/zsh completion), the ps1 side legitimately skips it — note that rather than forcing a port.
