# Profiles and images

Both live in the configuration directory `~/.config/devcontainer-cli/`, which
is a git repository: the `profile` and `image` commands commit every change
they make (nothing is pushed). Each record is a directory holding its manifest
and the scripts it names.

    ~/.config/devcontainer-cli/
    ├── profiles/<id>/profile.yml   + *.sh
    └── images/<name>/image.yml     + *.sh

## Which one you want

| | Profile | Image |
|---|---|---|
| What it is | a **project template** | a **toolchain artifact** (`devcontainer-cli-img/<name>:<tag>`) |
| Carries | modules, scripts, services, env, volumes, ports, forward ports, skills, an optional starting `image` | modules, scripts, an optional parent (`from`) |
| Built | per project, by `agent create --profile <id>` | once, by `image build <name>`, then shared |
| Use when | you want the same *project setup* again | several projects need the same slow-to-build toolchain |

The usual combination: an image holds the heavy toolchain, and a profile starts
from it (`image: <name>`) and adds what is per-project (services, env, a `start`
script that writes project config).

## profile.yml

```yaml
id: my-profile                 # must equal the directory name
label: "Python + Chromium"
modules: [python, chrome]      # plain module ids, as in --modules
image: py-tools                # optional: start from an image definition or registry ref
scripts:
  - file: install-tool.sh
    when: build
  - file: write-config.sh
    when: start
services:
  - id: postgres
env:
  DATABASE_URL: "postgres://postgres:postgres@postgres:5432/postgres"
volumes: ["cache:/home/devuser/.cache"]
ports: ["3000"]                # published; a bare port lets Docker pick the host port
forward_ports: ["5173", "reverse:5432"]
skills: [find-skills]
skills_mode: manual            # manual | auto
```

- Only `image` → the project runs that image as is (`mode=image`).
- `image` + `modules` → the project builds **derived** from that image, adding
  only the missing modules.
- Several profiles compose: `--profile a,b` unions modules, services, ports,
  volumes and skills, concatenates scripts and merges env (conflicts fail).

Use it:

    devcontainer-cli agent create --profile my-profile
    devcontainer-cli agent run --profile my-profile        # throwaway container

`profile create` is an interactive picker — **do not run it from an agent**.
Write the directory yourself (copy one from `../examples/profiles/`), or start
from an existing one with `devcontainer-cli profile copy <src> <new-id>`.
Check it with `devcontainer-cli profile info <id>`.

## image.yml

```yaml
name: py-tools                 # must equal the directory name
label: "Python toolbox"
source: build                  # build | remote
tag: latest                    # optional
from: base-image               # optional parent: another definition or a registry image built by the CLI
modules:                       # objects here, not plain ids
  - id: python
  - id: sqlite
scripts:
  - file: install-tools.sh
    when: build
```

Create one without prompts, then build it:

    devcontainer-cli image create py-tools --modules python,sqlite
    devcontainer-cli image create py-web --from py-tools --modules nodejs,pnpm
    devcontainer-cli image create ai-browser --from-profile ai-browser   # copies modules + scripts
    devcontainer-cli image create team --remote ghcr.io/acme/devcontainer-base:1
    devcontainer-cli image build py-tools
    devcontainer-cli image info py-tools

- `--from-profile` copies the profile's scripts **with their `when`**, so a
  `start` script stays `start`. To add scripts by hand, put the `.sh` next to
  `image.yml` and list it under `scripts:`.
- A derived image (`from`) needs its parent built first (`image build <parent>`);
  a registry parent is pulled. Only modules the parent lacks are installed.
- Rebuild after editing the definition or its scripts: `image build <name>`.
  A project derived from it sees the new parent in its fingerprint and rebuilds
  on its next `devcontainer-cli image update`.

## Before you build

The build warns if a `build` script touches the project. Read
`script-contract.md` before writing any script: the phase decides what exists.
