---
name: load-context
description: Read the documentation in .docs/ that governs the code you are about to touch, and only that. Maps an area (modules, compose services, commands, profiles, skills, SSH, docker runner, release) to the .docs/ sections, glossary terms, open roadmap items, and action skills that apply. Use before planning or editing code, or when checking which docs a change affects.
---

# Load the context for a change

Goal: before writing code, know the domain rules, architectural contracts, and pending decisions of the area you will touch, without reading all of `.docs/`.

## Steps

1. **Name the areas.** From the task description or the files you expect to change, pick every matching row of the table below.
2. **Read `AGENTS.md`** if it is not already in context.
3. **Read the documents of each area.** Read only the listed sections.
4. **Look up domain terms** of the task in `.docs/context.md`, and use those exact names in code.
5. **Check `.docs/roadmap.md`** for the area:
   - an open decision under "Decisiones abiertas" that blocks the task means **stop and ask the user**;
   - a backlog or debt item the task finishes must be deleted at the end.
6. **Read the code you will change**, plus one existing example of the same kind to match its idiom.

## Area → documents & action skills

| Area (packages, modules, domain flows) | Read |
|---|---|
| Dockerfile modules (`internal/domain/modules/dockerfile/`, `internal/domain/catalog/`) | `.docs/domain-model.md` §2, `.docs/architecture.md` §3 (Environment as Data, Python Contract); skill `add-dockerfile-module` |
| Compose services (`internal/domain/modules/compose/`, `internal/domain/catalog/`) | `.docs/domain-model.md` §3, `.docs/architecture.md` §1; skill `add-compose-service` |
| CLI commands and subcommands (`internal/cli/commands/`, `internal/cli/ui/`) | `.docs/code-conventions.md` §2–§3, `.docs/architecture.md` §1; skill `add-cli-command` |
| Service orchestration (`internal/service/`) | `.docs/architecture.md` §1–§2, `.docs/testing.md` §2, `.docs/code-conventions.md` §1 |
| Generator and fingerprint logic (`internal/domain/{generator,resolver,fingerprint}.go`) | `.docs/architecture.md` §3–§4 (D5, D6, D10), `.docs/domain-model.md` §1; skill `add-dockerfile-module` |
| Profiles and custom scripts (`internal/domain/catalog/profiles/`, `profiles.go`) | `.docs/domain-model.md` §4, `.docs/architecture.md` §4 (D8); skill `add-profile` |
| Agent skills catalog (`internal/domain/modules/skills/`, `skills.yml`) | `.docs/domain-model.md`, `.docs/architecture.md` §3 (D11); skill `add-agent-skill` |
| SSH lifecycle and port forwarding (`internal/service/{ssh,portforward}.go`, `commands/ssh.go`) | `.docs/architecture.md` §3–§4 (D13), `.docs/post-install.md` |
| Shared-config volume (`internal/service/sharedconfig.go`, `assets/shared-config.sh`) | `.docs/architecture.md` §3 (D7) |
| Docker runner, exec, and subprocesses (`internal/infra/docker/`) | `.docs/architecture.md` §1–§2 (CaptureFunc), `.docs/testing.md` §2 |
| Build, release, version injection, and upgrade (`.goreleaser.yaml`, `commands/upgrade_cli.go`) | `.docs/dev-environment.md` §2–§3 |
| Verification and test patterns (`*_test.go`, `helpers_test.go`) | `.docs/testing.md` §1–§3; skill `dev-ops-verify` |

The same table tells `code-reviewer` and `doc-editor` which documents a change can make stale.

## Output

Report in 4–6 lines:
- the areas and `.docs/` files read;
- the domain/architectural rules that constrain this task (with doc references);
- open decisions or roadmap items that touch it;
- any contradiction between `.docs/` and code (the code wins; flag the doc for `doc-editor`).
