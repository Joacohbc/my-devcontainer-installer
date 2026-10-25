---
name: task
description: Work a task end to end in three phases - start (pick roadmap item, branch, load context, agree on acceptance criteria), plan (tests first, domain contracts, docs, Dn decisions, small commits) and finish (test-runner, domain subagents, code-reviewer, doc-editor, systemic retrospective, commit and PR).
argument-hint: "start|plan|finish [task or roadmap item]"
---

# Task workflow

Run the phase named in `$ARGUMENTS` (`start`, `plan`, or `finish`). Never skip a phase silently.

## start
1. **Identify the task** in `.docs/roadmap.md`. If an open decision under "Decisiones abiertas" blocks it, stop and ask the user.
2. **Check the working tree** (`git status`). Never stash, reset, or delete branches without asking.
3. **Create a branch** from up-to-date `main`: `git fetch origin && git switch -c <type>/<short-slug> origin/main`.
4. **Run `load-context`** for the affected areas.
5. **For a bug, reproduce it first** with a failing test or deterministic steps.
6. Present the **Task Card** (`Task`, `Source`, `Branch`, `Areas`, `Acceptance`, `Out of scope`, `Open questions`) and wait for confirmation.

## plan
1. Read the code that will change and its callers.
2. Respect every domain and architectural invariant surfaced by `load-context`.
3. **Tests first**: table-driven unit tests for business logic, fake runner/reporter for service methods, invariant test for new structural rules.
4. Follow the relevant action skills (`add-dockerfile-module`, `add-compose-service`, `add-cli-command`, `add-profile`, `add-agent-skill`).
5. Ask the user about unsettled design choices and record the answer as the next `Dn` in `.docs/architecture.md`.
6. Present the plan (`Steps`, `Docs`, `Roadmap`, `Decisions`, `Verify`, `Risks`) and wait for approval on non-trivial work. Never commit plan files into the repo.

## finish
1. **Verify**: Run `test-runner` (plus `docker-optimizer` or `contract-auditor` when the change touches their domain). Everything must pass.
2. **Review**: Run `code-reviewer` on `git diff main...HEAD`. Fix every `blocker` and `major`, then re-verify.
3. **Docs & Roadmap**: Run `doc-editor` to update affected `.docs/*.md` files, `.docs/context.md`, and delete the completed item from `.docs/roadmap.md`.
4. **Retrospective**:
   - Classify any mistake, user correction, or misleading doc as **one-off** (do nothing) or **systemic** (likely to happen again to another agent).
   - For systemic items only, propose the smallest effective fix in this order: 1) clarify the misleading sentence in the existing `.docs/` file; 2) add a mechanical test/architecture rule; 3) add one line in the `.docs/` file that owns the topic. A new `AGENTS.md` rule is the last resort.
5. **Commit & PR**: Create atomic Conventional Commits in English explaining the *why*, present the summary (`Summary`, `Roadmap`, `Tests`, `Docs`, `Retrospective`), and ask before pushing or opening a PR.
