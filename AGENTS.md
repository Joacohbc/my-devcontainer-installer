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

## Version updates (Node, Java, Go, etc.)

When the version installed by a module changes (e.g. Node 22 → 24, Java 17 → 21, Ubuntu 22.04 → 24.04):

- Update the matching assert in `generator.test.ts` (e.g. `temurin-17-jdk` → `temurin-21-jdk`, `FROM ubuntu:22.04` → `FROM ubuntu:24.04`).
- If the version is parameterizable, add a test for the new default and for an explicit override.

## PR checklist

- [ ] New/modified module has test in `cli/src/__tests__/`.
- [ ] `pnpm test` passes locally.
- [ ] `pnpm run typecheck` passes.
- [ ] CI green.
