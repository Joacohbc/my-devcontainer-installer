---
name: add-profile
description: Add or update a built-in plain profile in internal/domain/catalog/profiles.go, keep remote variants in lockstep, and add test coverage. Script-carrying profiles are user profiles, never built-ins.
---

# Add or Update a Profile

Goal: Safely create or update a reusable devcontainer profile bundle.

## 1. Architectural Invariants

1. **Built-ins Are Plain Module Bundles**: Every built-in profile is a Go literal in `catalog.plainBuiltinProfiles` (`profiles.go`). The CLI ships no profile with scripts: tool-specific setup (scrapers, media tools, accessory AI tools) belongs in a user profile under `~/.devcontainer-cli/profiles/<id>/` (`profile.yml` plus its `.sh` scripts). Do not reintroduce a repo-shipped script profile; `TestRetiredProfilesAreAbsent` guards the retired ones.
2. **Profiles Compose**: `--profile a,b` merges profiles in `mode=custom` (`catalog.MergeProfiles`): modules and skills are unioned, scripts concatenated, and two scripts with the same destination file fail. A built-in should stay a minimal language or toolchain bundle so it stacks cleanly with user add-on profiles.
3. **Script Execution Timing (`when`)** for user profiles:
   - `build`: Emitted as a `RUN` layer in Dockerfile, staging dir removed in same layer.
   - `start`: Placed in `~/post-script/start.d/90-<script>.sh`, runs once per container lifetime (`.done` sentinel).
   - `service`: Placed in `~/post-script/services.d/<script>.sh`, runs in background on every start.
   - `manual`: Placed in `~/post-script/<script>.sh`, run manually by user.
4. **No Scripted Agent Skills**: Never install agent skills via custom scripts (`npx skills add -g`). Global directories link to the shared volume; use the `skills:` declaration in the profile instead.
5. **Remote Variant Lockstep**: If `Remote: true` is declared, the ID MUST exist in `types.RemoteVariants` and in the `.github/workflows/docker-image.yml` `build-variants` matrix.

## 2. Implementation Steps

1. **Create/Update Profile**: Add or edit an entry in `plainBuiltinProfiles` in `internal/domain/catalog/profiles.go`, listing `github-cli` first so its layer is shared with the cached `devcontainer-base` image.
2. **If Remote Prebuilt Image is Published**: Set `Remote: true`. Add the ID to `types.RemoteVariants` and `docker-image.yml`.

## 3. Mandatory Verification & Tests

1. **Remote Variant Lockstep**: Run `TestBuiltinProfileRemoteFlagMatchesPublishedVariants`.
2. **Retired Profiles Stay Out**: Run `TestRetiredProfilesAreAbsent`.
3. **Composition**: If merge behaviour changes, extend the `MergeProfiles` table tests in `catalog/profiles_test.go`.
4. **Script Generation**: Verify script emission in `domain/generator_custom_scripts_test.go`.

## 4. Documentation Impact

- Update `.docs/domain-model.md` §4 with the new profile.
