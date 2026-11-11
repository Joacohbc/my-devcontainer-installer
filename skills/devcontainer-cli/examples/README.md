# Examples

Every directory here is a working definition, loaded and linted by the CLI's
test suite (`cli/internal/domain/skill_examples_test.go`), so it cannot drift
from what the CLI accepts. Copy one into your config dir and adapt it:

    cp -r profiles/start-project-config ~/.config/devcontainer-cli/profiles/my-profile
    cp -r images/py-tools ~/.config/devcontainer-cli/images/my-image

Then rename the `id:` (profile) or `name:` (image) inside to match the directory.

| Example | Shows |
|---|---|
| `profiles/build-tool` | `when: build` — bake a tool into the image |
| `profiles/start-project-config` | `when: build` + `when: start` — install at build, write per-project config at start |
| `profiles/service-dev-server` | `when: service` — a daemon started on every container boot |
| `profiles/manual-db-reset` | `when: manual` — an interactive helper the user runs by hand |
| `profiles/from-image` | a profile that starts from an image definition instead of modules |
| `images/py-tools` | an image definition with modules and a build script |
| `images/py-tools-web` | a derived image (`from:`) that only adds what its parent lacks |
