---
name: add-cli-command
description: Add or modify a CLI command or subcommand in internal/cli/commands/, wire flags, delegate business logic to internal/service/, enforce non-interactive execution and non-prompting recovery hints, and update documentation.
---

# Add or Modify a CLI Command

Goal: Implement or update a Cobra CLI command adhering to the 6 canonical resource domains, strict layering rules, error contracts, canonical flag table, and non-prompting recovery hints.

## 1. Architectural Invariants

1. **6 Canonical Resource Domains**: Every command MUST be placed inside one of the 6 canonical resource domains (`workspace`, `access`, `inspect`, `profile`, `image`, `config`) or the `agent` facade. NEVER register a flat root command.
2. **Zero Command Aliases**: Every command in the tree must have zero aliases (`len(cmd.Aliases) == 0`). Every command has exactly one unambiguous invocation path.
3. **Canonical Flag Conventions**: Always consult the Canonical Flag Table in `.docs/code-conventions.md` §2:
   - **Plural CSV flags**: Use `--services`, `--skills`, `--modules`, `--ports`, `--volumes`, `--forward-ports`. Singular forms (`--service`, `--skill`, `--with`) are strictly banned for list options.
   - **Reserved shorthands**: Shorthands are reserved repository-wide and must strictly map to their canonical long flag names:
     - `-c` → `"container"`
     - `-y` → `"yes"`
     - `-a` → `"all"`
     - `-f` → `"follow"`
     - `-w` → `"workdir"`
     - `-v` → `"volumes"` (never `"version"`)
   - **Persistent `--no-interactive`**: Inherited from the root command. Subcommands must never re-register `--no-interactive` (except via `addInteractiveFlag` for standalone unit test harnesses) and never use `--non-interactive`. Missing required arguments in non-interactive mode must return an immediate error without prompting.
   - **Boolean flags**: Avoid `--x`/`--no-x` tri-state pairs. Use a single boolean flag supporting `--x=false` to opt out.
4. **No Redundant Agent Clones**: Prohibit creating 1:1 clones in `agent` when defaults are identical to the domain command. The `agent` facade exists strictly for unattended, never-interactive defaults (run as `devuser`, fail loud, skip confirmation).
5. **Zero Docker / Exec in Commands**: `internal/cli/commands/` must NEVER import `internal/infra/docker` or `os/exec`. All business orchestration belongs in `internal/service/`.
6. **Non-Prompting Recovery Hints**: Error messages telling the caller what to run next must NEVER suggest the interactive wizard (`devcontainer-cli`) or flat legacy commands (`devcontainer-cli up`). Always name canonical 6-domain or agent commands (`workspace up`, `workspace start`, `workspace down`, `access ssh`, `profile list`, `agent create`).
7. **Cobra Documentation Contract**: `Short` is imperative, capitalized, and without a trailing period. `Long` explains what it does and when to use it without re-listing flags in prose. `Example` provides realistic invocations.

## 2. Implementation Steps

1. **Create/Edit Command File**:
   - `internal/cli/commands/<name>.go`.
   - Implement `new<Name>Command() *cobra.Command`.
   - Attach shared flag helpers from `flags.go` (`addYesFlag`, `addInteractiveFlag`, `addBuildFlag`, `addContainerFlag`).
   - Register under its domain in `domains.go` (`workspace`, `access`, `inspect`), `config.go`, `image.go`, or `profile.go`.
2. **Delegate to Service**:
   - In `RunE`, parse flags, resolve paths, instantiate `svc := service.<Group>Service{Report: ui.Console{}}`, and invoke the service verb.
3. **If Facade Access is Needed (`agent`)**:
   - If a differentiated agent-oriented workflow is required, wire it in `internal/cli/commands/agent.go` reusing extracted flag helpers and enforcing strict unattended defaults (run as `devuser`, fail loud). Do not create a clone if behavior is identical.

## 3. Mandatory Verification & Tests

1. **Layering Check**: Verify `grep -rln "infra/docker" internal/cli/commands` remains empty.
2. **Paradigm Invariants Test**: Run `go test -v ./internal/cli/commands -run TestCLIParadigmInvariants`.
3. **Command Flags & Parsing**: Add tests in `internal/cli/commands/commands_test.go`.
4. **Recovery Hints Gate**: Run `go test -v ./internal/cli/commands -run TestErrorMessagesDoNotPointAtTheWizard`.
5. **CLI Reference Synchronization**: Verify and update `.docs/cli-reference.md` via `UPDATE_CLI_REF=1 go test ./internal/cli/commands -run TestCLIReference_UpToDate`.
6. **Service Method Coverage**: Add matching test case in `internal/service/<group>_test.go` using `nopReporter`, `fakeRunner`, and `scriptedPrompter`.

## 4. Documentation Impact

- Update `.docs/code-conventions.md` §2–§3 if new flags or error patterns are introduced.
- Review and update `AGENTS.md` command table if a domain command is added or modified.
