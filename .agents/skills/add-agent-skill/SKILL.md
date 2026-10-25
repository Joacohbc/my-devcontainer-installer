---
name: add-agent-skill
description: Add or update a curated agent skill in internal/domain/modules/skills/skills.yml, define required modules, configure dynamic CONTEXT.md entries, and add test coverage.
---

# Add or Update an Agent Skill

Goal: Add a curated assistant skill to the catalogue so users and profiles can select it cleanly.

## 1. Architectural Invariants

1. **Project-Scoped Installation**: Skills are installed inside the workspace mount via `npx skills add`, NEVER globally in the shared volume.
2. **Module Dependency Declaration**: If a skill relies on tools (e.g. Python, Chrome, Node.js), declare them in `requires_modules:`. Missing dependencies are detected by `domain.MissingSkillModules`.
3. **No Branch Names in Refs**: A multi-skill repository must use `ref: owner/repo` and `skill: <name>`, never `/tree/<branch>` URLs that break on branch renames.
4. **Compose Environment Driven**: Skills configuration is injected via compose environment (`DEVCONTAINER_SKILLS`), meaning image rebuilds are unnecessary when adding or removing skills.

## 2. Implementation Steps

1. **Add to Catalog**:
   - Edit `internal/domain/modules/skills/skills.yml`.
   - Declare:
     ```yaml
     - id: <skill-id>
       name: <Display Name>
       category: <Category>
       ref: <owner/repo>
       skill: <sub-skill-name> # optional if repository contains multiple skills
       requires_modules:
         - <module-id>
       context:
         title: <Title in CONTEXT.md>
         body: <Instructions for the agent>
     ```
2. **Verify Dependencies**: Ensure any module listed in `requires_modules` exists in `internal/domain/catalog/catalog.go`.

## 3. Mandatory Verification & Tests

1. **Catalogue Well-Formedness**: Run `go test -v ./internal/domain -run TestAgentSkillCatalogue`.
2. **Missing Modules Check**: Run `TestMissingSkillModules`.
3. **Context Generation**: Test that selecting this skill renders the expected section in `~/CONTEXT.md`.

## 4. Documentation Impact

- Update `.docs/domain-model.md` §1 (Skills) if category structures change.
