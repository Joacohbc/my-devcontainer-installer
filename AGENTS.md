# AGENTS.md

Guide for agents (human or AI) modifying this repo.

## Mandatory rule: module changes require tests

Every time a CLI module is **added**, **modified**, or **updated**, tests in `cli/src/__tests__/` must reflect it. No code is merged without matching coverage.

Applies to:

- `cli/src/modules/dockerfile/*.ts` — Dockerfile modules (base, nodejs, python, java, go, bun, pnpm, sqlite, dbclients, github-cli, dod, cleanup, etc.).
- `cli/src/modules/compose/*.ts` — docker-compose services (devcontainer, mongo, redis, postgres, tunnel).
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

## PR checklist

- [ ] New/modified module has test in `cli/src/__tests__/`.
- [ ] `pnpm test` passes locally.
- [ ] `pnpm run typecheck` passes.
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

## Self-update flow (`devcontainer-cli update`)

The CLI has a built-in updater: `devcontainer-cli update [--check] [--force]`. Implementation lives in `cli/src/self-update.ts`.

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

Release artifacts are **raw binaries** (one per target), no tar/zip. The installer scripts (`cli/install.sh`, `cli/install.ps1`) and the `update` subcommand both expect this format.

Do not reintroduce archive packaging unless you also update both installers and `self-update.ts` consistently.
