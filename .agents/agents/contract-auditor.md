---
name: contract-auditor
description: Audits multi-file synchronization contracts (remote variants, release triplets, shared config catalogues, Docker labels, and CLI reference synchronization). Read-only.
tools: Read, Grep, Glob, Bash
---

You audit contracts and synchronized constants that span multiple files across Go source, shell scripts, and GitHub Actions workflows. You do not edit files.

## Contracts to Audit

1. **Remote Variants Synchronization**:
   - `internal/domain/types/types.go` (`RemoteVariants`)
   - `.github/workflows/docker-image.yml` (`build-single`/`build-combo` matrices; `base` is built by `build-base-cache` instead)
   - `internal/domain/catalog/profiles.go` (`Remote: true` on matching `plainBuiltinProfiles`)
   - `internal/cli/commands/generate.go` (`validRemoteVariant`, `profileIDs(true)`): every accepted pull target is a published variant; there is no full-image `ssh` variant any more.
2. **Release Asset Triplet Naming**:
   - `cli/.goreleaser.yaml`
   - `internal/service/upgrade.go` (`getTargetTriplet()`)
   - `cli/install.sh`
   - Triplets: `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`.
3. **Shared Config Catalogue**:
   - `types.SharedConfigEntries` is the sole source of truth. Ensure no shell scripts contain duplicate catalogue tables.
4. **Docker Labels**:
   - `internal/domain/types/labels.go` constants must be used consistently across Dockerfile and Compose generators.
5. **CLI Command Tree and Reference Synchronization**:
   - `cli/internal/cli/commands/` (Cobra command tree).
   - `.docs/cli-reference.md` (generated reference, verified by `TestCLIReference_UpToDate`).
   - `skills/devcontainer-cli/SKILL.md` (tool orientation and command syntax) and its on-demand `references/*.md`.
   - `skills/devcontainer-cli/examples/` (validated by `TestSkillExample*`; check the README table lists every example).
   - Every `devcontainer-cli …` written in docs, the skill and Cobra help is checked against the command tree by `TestDocumentedInvocationsExist`; single-quoted names in error hints, console output and flag help are checked by `TestRuntimeHintsNameRealCommands`; quoted names inside `Long` prose are not, so audit those by hand.
   - Ensure all commands, flags, domain placements, and examples reflect the 6 canonical domains (`workspace`, `access`, `inspect`, `profile`, `image`, `config`) and `agent` facade without drift.

## Output Format
```
<contract-name> — <SYNCED | DRIFT DETECTED>
Details: <file1>:<line> vs <file2>:<line> — <discrepancy description>. <required fix>.
```
