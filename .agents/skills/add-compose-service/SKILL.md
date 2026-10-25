---
name: add-compose-service
description: Add or update a Compose service in internal/domain/modules/compose/, register it in the catalog, configure ports, volumes, healthchecks, Context section, and add test coverage.
---

# Add or Update a Compose Service

Goal: Safely create or modify a companion Compose service (e.g., database, queue, secondary engine) integrated into the devcontainer stack.

## 1. Architectural Invariants

1. **Port Spec Compliance**: Published ports must comply with Compose port grammar (`[IP:][HOST:]CONTAINER`), validated by `domain.ValidatePortSpecs`.
2. **Deterministic Healthchecks**: Any stateful service must define a deterministic healthcheck condition so dependent containers can coordinate startup.
3. **Workspace Isolation**: Volumes must be scoped to the workspace (e.g. `<workspace>-postgres-data`) to prevent cross-project database clobbering.
4. **Context Documentation**: Implement `Context func(ctx RenderContext) *types.ContextSection` describing connection ports, default credentials, and connection commands for `~/CONTEXT.md`.

## 2. Implementation Steps

1. **Create Service Spec**:
   - Create or edit `internal/domain/modules/compose/<name>.go`.
   - Define `ServiceSpec` struct: `ID`, `Name`, `Category`, `Description`, `RequiresEnv`, `Context`, and `Render`.
2. **Catalog Registration**:
   - Register the service in `internal/domain/catalog/catalog.go` under `ComposeServices`.
3. **Render Implementation**:
   - Render compose service fragment specifying image, environment variables, volumes, and ports.

## 3. Mandatory Verification & Tests

1. **Catalog Resolution**: Assert service resolves in catalog (`catalog.GetComposeService(id)`).
2. **Generated Compose**: Verify emitted `docker-compose.yml` has valid syntax, services, network attachment, and environment variables (`domain/generator_test.go`).
3. **Context Section**: Test that `~/CONTEXT.md` contains connection instructions (`domain/context_test.go`).
4. **Wizard Selection**: Verify interactive selection works in `service/generate_wizard_test.go`.

## 4. Documentation Impact

- Update `.docs/domain-model.md` §3 table with the new service ID, image, ports, and volumes.
- Update `.docs/context.md` if new database clients or services are introduced.
