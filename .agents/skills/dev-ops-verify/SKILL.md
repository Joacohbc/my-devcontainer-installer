---
name: dev-ops-verify
description: Run the deterministic Go toolchain verification checks (gofmt, go vet, go build, go test, and goreleaser check) from cli/ to ensure zero broken builds before committing or opening a PR.
---

# Deterministic Local Verification (`dev-ops-verify`)

Goal: Run all mandatory CI gate checks locally to guarantee clean integration.

## Mandatory Toolchain Invariants

1. All commands run strictly from the `cli/` directory.
2. The Go tree must remain 100% `gofmt`-clean (prints nothing).
3. `go vet` must report zero findings.
4. All package tests must pass (`go test ./...`).
5. Release assets and GoReleaser manifests must be valid (`goreleaser check`).

## Execution Protocol

Run each step sequentially from `cli/`:

### 1. Format Check
```bash
gofmt -l .
```
- **Rule**: Must return empty output. If files are listed, run `gofmt -w .` immediately.

### 2. Static Analysis (`go vet`)
```bash
go vet ./...
```
- **Rule**: Must return zero warnings or errors.

### 3. Build Check
```bash
go build ./...
```
- **Rule**: Must compile all binary entry points and packages cleanly.

### 4. Unit & Integration Test Suite
```bash
go test ./...
```
- **Rule**: All test cases across `internal/domain`, `internal/service`, `internal/cli`, and `internal/infra` must pass.

### 5. GoReleaser Configuration Check
```bash
goreleaser check
```
- **Rule**: Validates `.goreleaser.yaml` schema and release asset configurations.

## Output Format

Return a 4-line summary:
```
gofmt:       ✔ clean
go vet:      ✔ 0 findings
go build:    ✔ compiled cleanly
go test:     ✔ all suites passed (<n> tests)
goreleaser:  ✔ valid
```
