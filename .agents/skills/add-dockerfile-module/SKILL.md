---
name: add-dockerfile-module
description: Add or update a Dockerfile module in internal/domain/modules/dockerfile/, register it in the catalog, configure ProvidesEnv/RequiresEnv, enforce apt --no-install-recommends, and add comprehensive unit test coverage.
---

# Add or Update a Dockerfile Module

Goal: Safely create or modify a composable Dockerfile module while respecting container environment data models, layer caching, and dependency contracts.

## 1. Architectural Invariants

1. **Environment as Data**: Declare process variables and `PATH` entries using `ProvidesEnv func(opts) ContainerEnv`. Never write `export PATH=...` in shell scripts.
2. **Secrets Never Baked**: If a tool requires tokens at runtime, declare them in `RequiresEnv`. Never write secrets into Dockerfile `ENV` or `ARG` directives.
3. **Layer Weight & Recommendations**: Every `apt-get install` MUST include `--no-install-recommends`. Adding a package with recommended dependencies requires updating `aptRecommendsAllowed` in `render_test.go` with justification.
4. **Dynamic Context Section**: Implement `Context func(opts) *types.ContextSection` returning markdown documentation for `~/CONTEXT.md`. Return `nil` when the tool is unconfigured or deselected.

## 2. Implementation Steps

1. **Create Module Spec**:
   - Create or edit `internal/domain/modules/dockerfile/<name>.go`.
   - Define the `ModuleSpec` struct: `ID`, `Name`, `Category`, `Description`, `Requires`, `Conflicts`, `CopyFiles`, `ProvidesEnv`, `RequiresEnv`, `Context`, and `Render`.
2. **Catalog Registration**:
   - Register the module in `internal/domain/catalog/catalog.go` under `DockerfileModules`.
3. **If Embedded Assets are Needed**:
   - Place shell scripts in `internal/infra/assets/<name>.sh`.
   - Reference in module's `CopyFiles`.
   - If user-copyable at runtime, register in `internal/infra/assets/registry.go` with `Kind: KindScript`.
4. **If Runtime Env Vars are Required (`RequiresEnv`)**:
   - Ensure an empty value is treated as valid (unset). The generator forwards them via compose `environment: NAME=${NAME:-}`.

## 3. Mandatory Verification & Tests

1. **Catalog Resolution**: Verify module is found by ID (`TestCatalogModulesResolvable`).
2. **Generated Output**: Verify emitted Dockerfile contains expected `RUN`, `ENV`, and package fragments (`domain/generator_test.go`).
3. **Dependencies & Conflicts**: Add cases in `domain/resolver_test.go`.
4. **Apt Recommendations**: Verify `TestAptInstallsSkipRecommends` passes.
5. **Context Generation**: Add cases in `internal/domain/context_test.go`.

## 4. Documentation Impact

- Update `.docs/domain-model.md` §2 table with the new module ID, category, and invariants.
- Update `.docs/context.md` if new domain tools or terminology are introduced.
