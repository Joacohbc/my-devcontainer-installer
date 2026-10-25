---
name: add-profile
description: Add or update a built-in plain profile or embedded script-carrying profile in internal/domain/catalog/profiles/, configure custom script timing, align remote variants, and add test coverage.
---

# Add or Update a Profile

Goal: Safely create or update a reusable devcontainer profile bundle.

## 1. Architectural Invariants

1. **Two Physical Shapes**:
   - Pure module bundle: Defined as a Go literal in `catalog.plainBuiltinProfiles` (`profiles.go`).
   - Profile with scripts: Defined as an embedded directory `internal/domain/catalog/profiles/<id>/profile.yml` plus its `.sh` scripts.
2. **Script Execution Timing (`when`)**:
   - `build`: Emitted as a `RUN` layer in Dockerfile, staging dir removed in same layer.
   - `start`: Placed in `~/post-script/start.d/90-<script>.sh`, runs once per container lifetime (`.done` sentinel).
   - `service`: Placed in `~/post-script/services.d/<script>.sh`, runs in background on every start.
   - `manual`: Placed in `~/post-script/<script>.sh`, run manually by user.
3. **No Scripted Agent Skills**: Never install agent skills via custom scripts (`npx skills add -g`). Global directories link to the shared volume; use `skills:` declaration in the profile instead.
4. **Remote Variant Lockstep**: If `Remote: true` is declared, the ID MUST exist in `types.RemoteVariants` and in the `.github/workflows/docker-image.yml` `build-variants` matrix.

## 2. Implementation Steps

1. **Create/Update Profile**:
   - For pure bundles, add an entry to `plainBuiltinProfiles` in `internal/domain/catalog/profiles.go`.
   - For script profiles, create `internal/domain/catalog/profiles/<id>/profile.yml` declaring `id`, `name`, `modules`, `scripts` (with `name` and `when`), and optional `skills`.
2. **Script Validation**:
   - Script file names must match `^[A-Za-z0-9][A-Za-z0-9._-]*\.sh$`.
3. **If Remote Prebuilt Image is Published**:
   - Set `Remote: true`. Add ID to `types.RemoteVariants` and `docker-image.yml`.

## 3. Mandatory Verification & Tests

1. **Well-Formed Embedded Profiles**: Run `TestEmbeddedProfilesAreWellFormed`.
2. **Remote Variant Lockstep**: Run `TestBuiltinProfileRemoteFlagMatchesPublishedVariants`.
3. **No Scripted Skills**: Run `TestEmbeddedProfilesDoNotScriptSkillInstalls`.
4. **Script Generation**: Verify script emission in `domain/generator_custom_scripts_test.go`.

## 4. Documentation Impact

- Update `.docs/domain-model.md` §4 with the new profile and script behavior.
