# AGENTS.md — devcontainer-cli

`devcontainer-cli` is a Go CLI tool (module `github.com/joacohbc/my-devcontainer-installer/cli`, Go 1.26, built on `spf13/cobra`) that generates, builds, runs, and operates disposable, reproducible Docker devcontainers for human developers and autonomous AI agents.

This file holds the constitutional rules an agent must follow. Detailed documentation lives in `.docs/` (Spanish) and the domain vocabulary in `.docs/context.md`.

## Where to look

| Need | Read |
|---|---|
| Product purpose, personas, and end-to-end flows | `.docs/product.md` |
| 4-layer architecture, boundaries, and decisions D1–D13 | `.docs/architecture.md` |
| Entities, Dockerfile modules, compose services, profiles | `.docs/domain-model.md` |
| Domain vocabulary (ubiquitous language) | `.docs/context.md` |
| Code conventions, `hbc-code-design`, command authoring | `.docs/code-conventions.md` |
| Local Go toolchain, CI workflows, GoReleaser, releases | `.docs/dev-environment.md` |
| Test pyramid, runner mocks, and invariant regression tests | `.docs/testing.md` |
| Cloudflare Tunnel private network access | `.docs/cloudflare-tunnel.md` |
| Post-install shell access, credentials, and DB operations | `.docs/post-install.md` |
| Pending work, open decisions, and technical debt | `.docs/roadmap.md` (the single task list) |

## Commands

All development commands run strictly from the `cli/` directory:

```bash
go build ./...                               # Build all packages
go test ./...                                # Run all unit and service tests
go vet ./...                                 # Static analysis (must be clean)
gofmt -l .                                   # Format check (must print nothing; fix with gofmt -w .)
go run ./cmd/devcontainer-cli [args]         # Run CLI in development
goreleaser check                             # Validate GoReleaser config
```

When operating on devcontainer workspaces, agents must strictly use the provided CLI commands:
- Inside container: `devcontainer-cli agent exec -w -- <cmd>`
- Port forwarding: `devcontainer-cli agent forward <ports>`
- SSH session: `devcontainer-cli agent ssh [--ephemeral]`
- Temporal environments: `devcontainer-cli agent create --temporal --profile <id>`

## Architecture in five lines

- **Strict 4-layer separation**: `cmd/devcontainer-cli` → `internal/cli` → `internal/service` → `{internal/domain, internal/infra}`. A layer imports only below it.
- **Commands are a thin shell**: `internal/cli/commands` parses flags and delegates entirely to `internal/service`. Commands NEVER import `infra/docker` or `os/exec`.
- **Domain is pure**: `internal/domain` handles generation, validation, and fingerprinting without importing `infra` or `cli` (Docker queries use `CaptureFunc`).
- **Workspace isolation**: Projects mount at `/workspaces/<workspace>` with `/workspace/<workspace>` alias, never bare `/workspace` (prevents cross-project agent history pollution).
- **Environment as data**: `ProvidesEnv` models `PATH` and variables as structured data, compiled to Dockerfile `ENV` and `~/.dc-env.sh`.

## MUST (Domain & Architectural Invariants)

- **Go toolchain only**: keep the tree `gofmt`-clean and `vet`-clean. CI blocks any unformatted file or vet finding.
- **Tests for logic changes**: every new or modified module, domain function, or service method requires a matching `_test.go`.
- **Documentation review**: every command or flag change requires updating its `Short`/`Long`/`Example`, flags description, and `.docs/code-conventions.md`.
- **Non-prompting recovery hints**: error hints must name commands that do not prompt (`up`, `start`, `agent create`), NEVER the interactive wizard.
- **Python container contract**: `UV_SYSTEM_PYTHON=1`, `UV_BREAK_SYSTEM_PACKAGES=1`, non-recursive `chown` on `sysconfig` paths. Never image-wide `VIRTUAL_ENV`.
- **Layer caching**: leading Dockerfile layers remain byte-identical across variants for `devcontainer-base` cache reuse; every `apt-get` passes `--no-install-recommends`.
- **Tests first for bugs**: every bug fix ships with a reproducing test.
- **Documentation hygiene**: update the affected `.docs/` file and `.docs/context.md` in the same change; when a roadmap item is completed, delete it from `.docs/roadmap.md`.

## NEVER

- **NEVER** run direct Docker commands (`docker`, `docker compose`, `docker exec`) to interact with a workspace.
- **NEVER** manually edit generated files inside `.dc_<workspace>/`.
- **NEVER** import `internal/infra/docker` or `os/exec` inside `internal/cli/commands/`.
- **NEVER** suggest `devcontainer-cli` nakedly in error recovery hints.
- **NEVER** hardcode secrets or tokens in Dockerfile layers (use `RequiresEnv`).
- **NEVER** install global agent skills into the shared-config volume (`skills:` are project-scoped).

## Languages

- Code, identifiers, comments, logs, commits, `AGENTS.md`, skills, subagents: **English**.
- `.docs/*.md`, `README.md`, conversation: **Neutral Spanish** (strictly no regionalisms or voseo: never use *tenés, podés, hacé, fijate, borrá, agregá, acá*).
- Commits follow Conventional Commits (`feat`, `fix`, `refactor`, `test`, `docs`, `chore`) and explain the *why*.

## Skills (`.agents/skills/`)

| Skill | Use when |
|---|---|
| `load-context` | Reading the exact `.docs/` sections, glossary terms, and action skills governing a change before touching code |
| `task` | Executing a task end-to-end (`start \| plan \| finish`) with task card, TDD first, verification, and systemic retrospective |
| `dev-ops-verify` | Running local Go toolchain verification (`gofmt`, `go vet`, `go build`, `go test`, `goreleaser check`) |
| `add-dockerfile-module` | Adding or updating a Dockerfile module in `internal/domain/modules/dockerfile/` |
| `add-compose-service` | Adding or updating a companion Compose service in `internal/domain/modules/compose/` |
| `add-cli-command` | Adding or modifying Cobra commands and subcommands in `internal/cli/commands/` |
| `add-profile` | Creating or updating built-in or embedded script profiles in `internal/domain/catalog/profiles/` |
| `add-agent-skill` | Adding curated agent skills to `internal/domain/modules/skills/skills.yml` |

## Subagents (`.agents/agents/`)

| Agent | Use when |
|---|---|
| `test-runner` | Running verification checks and tests in `cli/` and reporting a compact pass/fail summary |
| `code-reviewer` | Auditing diffs against `AGENTS.md`, layering boundaries, `hbc-code-design`, error contracts, and doc impact |
| `doc-editor` | Keeping `.docs/*.md`, `README.md`, `AGENTS.md`, skills, and subagents 100% synchronized with code |
| `docker-optimizer` | Auditing Dockerfile generators, layer caching, `apt-get --no-install-recommends`, and compose topology |
| `contract-auditor` | Auditing multi-file synchronization contracts (remote variants, release triplets, shared config, labels) |

## Workflow

`/task start` → `/task plan` → implement → `/task finish`.
