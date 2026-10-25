---
name: add-cli-command
description: Add or modify a CLI command or subcommand in internal/cli/commands/, wire flags, delegate business logic to internal/service/, enforce non-interactive execution and non-prompting recovery hints, and update documentation.
---

# Add or Modify a CLI Command

Goal: Implement or update a Cobra CLI command adhering to the strict layering rules, error contracts, and non-prompting recovery hints.

## 1. Architectural Invariants

1. **Zero Docker / Exec in Commands**: `internal/cli/commands/` must NEVER import `internal/infra/docker` or `os/exec`. All business orchestration belongs in `internal/service/`.
2. **Self-Registration**: Every command self-registers in an `init()` function via `register(new<Name>Command())`.
3. **Non-Interactive Mode**: New subcommands must support `--no-interactive`. Missing required arguments in non-interactive mode must return an immediate error without prompting.
4. **Non-Prompting Recovery Hints**: Error messages telling the caller what to run next must NEVER suggest the interactive wizard (`devcontainer-cli`). Always name commands that do not prompt (`up`, `start`, `agent create`).
5. **Cobra Documentation Contract**: `Short` is imperative and capitalized with no trailing period. `Long` explains what it does and when to use it without re-listing flags in prose. `Example` provides realistic invocations.

## 2. Implementation Steps

1. **Create/Edit Command File**:
   - `internal/cli/commands/<name>.go`.
   - Implement `new<Name>Command() *cobra.Command`.
   - Attach shared flag helpers from `flags.go` (`addYesFlag`, `addInteractiveFlag`, etc.).
2. **Delegate to Service**:
   - In `RunE`, parse flags, resolve paths, instantiate `svc := service.<Group>Service{Report: ui.Console{}}`, and invoke the service verb.
3. **If Facade Access is Needed (`agent`)**:
   - If an agent-oriented shortcut is appropriate, wire it in `internal/cli/commands/agent.go` reusing extracted flag helpers and enforcing strict unattended defaults (run as `devuser`, fail loud).

## 3. Mandatory Verification & Tests

1. **Layering Check**: Verify `grep -rln "infra/docker" internal/cli/commands` remains empty.
2. **Command Flags & Parsing**: Add tests in `internal/cli/commands/commands_test.go`.
3. **Recovery Hints Gate**: Run `go test -v ./internal/cli/commands -run TestErrorMessagesDoNotPointAtTheWizard`.
4. **Service Method Coverage**: Add matching test case in `internal/service/<group>_test.go` using `nopReporter`, `fakeRunner`, and `scriptedPrompter`.

## 4. Documentation Impact

- Update `.docs/code-conventions.md` §2–§3 if new flags or error patterns are introduced.
- Review and update `AGENTS.md` command table if a top-level command is added or modified.
