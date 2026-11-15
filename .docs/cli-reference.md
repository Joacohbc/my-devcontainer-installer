# CLI Reference

> Generado automáticamente desde el árbol de comandos de Cobra (`TestCLIReference_UpToDate`). No editar a mano.

## `devcontainer-cli`

Provision and manage reproducible, SSH-capable devcontainers

### Banderas globales

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access`

Connect to a running container via shell, SSH, port-forwarding or file copy

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access copy`

Copy a file or directory between the host and a running container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--asset` |  | bool | `false` | Copy into /usr/local/share/devcontainer-assets/ inside the container (default destination when omitted; requires root) |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access forward`

Forward host ports into a running container over SSH

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--alias` |  | string | - | SSH host alias to use (bypasses auto-discovery) |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `true` | Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--reverse` |  | bool | `false` | Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:') |
| `--service` |  | string | - | Compose service to map port to (default: localhost) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access forward via`

Forward host ports into a container on a remote Docker host over SSH

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--ephemeral` |  | bool | `true` | Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--reverse` |  | bool | `false` | Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:') |
| `--service` |  | string | - | Compose service to map port to (default: localhost) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access network`

Expose a workspace network via an in-cluster VPN gateway container (Tailscale, WireGuard, etc.)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access network connect`

Attach a VPN container (Tailscale, WireGuard, ...) to a workspace's private network

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access network disconnect`

Detach a VPN container from a workspace's private network

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access network status`

Show which VPN containers are attached to workspace networks

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route`

Expose a container port on a clean *.devcli.localhost hostname via a local reverse proxy

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route add`

Forward http://<workspace>-<port>.devcli.localhost to a container port in the foreground

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--port` | -p | int | `8080` | Host port the local reverse proxy listens on |
| `--service` |  | string | - | Compose service on the workspace network (default: the devcontainer) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route list`

List active *.devcli.localhost routes

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route remove`

Remove a *.devcli.localhost route and stop the proxy if no routes remain

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access shell`

Open a shell (or run a command) in the devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-tty` | -T | bool | `false` | Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal |
| `--type` |  | string | - | Shell to open: bash, zsh or sh (interactive shell defaults to zsh) |
| `--user` |  | string | - | User to run the command as; always honoured, but only an interactive shell defaults it to devuser |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--workdir` | -w | bool | `false` | Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off |

## `devcontainer-cli access shell via`

Open a shell or run a command inside a container on a remote Docker host

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--no-tty` | -T | bool | `false` | Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal |
| `--type` |  | string | - | Shell to open: bash, zsh or sh (interactive shell defaults to zsh) |
| `--user` |  | string | - | User to run the command as; always honoured, but only an interactive shell defaults it to devuser |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--workdir` | -w | bool | `false` | Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off |

## `devcontainer-cli access ssh`

Open a real SSH session into the devcontainer, configuring access first if needed

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `true` | Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--forward` |  | bool | `false` | Also open SSH port-forwarding tunnels for the session (prompted when interactive and omitted) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--ports` |  | string | - | Ports to forward, e.g. '3000,8080:80,100.64.0.1:8080:80' (implies --forward; skips the prompt). Supports bind:local:container, local:host:container, and a 'reverse:' prefix to make the container reach a port on this machine |
| `--setup` |  | bool | `false` | Re-run the SSH access setup (key + Host block) before connecting, even if an alias already exists |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli access ssh via`

Connect over SSH to a container on a remote Docker host

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--ephemeral` |  | bool | `true` | Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--forward` |  | bool | `false` | Also open SSH port-forwarding tunnels for the session (prompted when interactive and omitted) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--ports` |  | string | - | Ports to forward, e.g. '3000,8080:80,100.64.0.1:8080:80' (implies --forward; skips the prompt). Supports bind:local:container, local:host:container, and a 'reverse:' prefix to make the container reach a port on this machine |
| `--setup` |  | bool | `false` | Re-run the SSH access setup (key + Host block) before connecting, even if an alias already exists |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli agent`

Non-interactive commands for AI agents (discoverable catalogue + one-shot creation)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent cli-info`

Emit the full machine-readable catalogue (images, services, profiles, skills, flags)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--json` |  | bool | `false` | Emit JSON instead of a human-readable summary |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent create`

Create a devcontainer project from flags without interactive prompts

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--apt` |  | string | - | Comma-separated extra apt packages |
| `--build` |  | bool | `false` | Build the Docker image immediately after generating |
| `--cwd` |  | string | - | Target directory (default: current working directory) |
| `--force` |  | bool | `false` | Overwrite existing devcontainer files |
| `--forward-ports` |  | string | - | Comma-separated host ports to forward on 'access forward' (e.g. '3000,8080:80') |
| `--image` |  | string | `base` | Base image id, optionally with :version (e.g. node:22, python:3.12, go) |
| `--json` |  | bool | `false` | Emit structured JSON result on stdout |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--name` |  | string | - | Workspace / compose project name (default: directory name) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--npm` |  | string | - | Comma-separated extra global npm packages |
| `--pip` |  | string | - | Comma-separated extra pip packages |
| `--profile` |  | string | - | Saved profile name from ~/.config/devcontainer-cli/profiles/ |
| `--services` |  | string | - | Comma-separated compose service ids (e.g. postgres,redis) |
| `--skills` |  | string | - | Comma-separated agent skill ids (see 'agent cli-info --json') |
| `--up` |  | bool | `false` | Build and start the workspace immediately after generating |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent exec`

Run a command inside the devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--no-tty` | -T | bool | `false` | Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal |
| `--user` |  | string | - | User to run the command as; always honoured, but only an interactive shell defaults it to devuser |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--workdir` | -w | bool | `false` | Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off |

## `devcontainer-cli agent exec via`

Run a command inside a container on a remote Docker host

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--no-tty` | -T | bool | `false` | Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal |
| `--user` |  | string | - | User to run the command as; always honoured, but only an interactive shell defaults it to devuser |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--workdir` | -w | bool | `false` | Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off |

## `devcontainer-cli agent forward`

Forward a port between this machine and the running container over SSH

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--alias` |  | string | - | SSH host alias to use (bypasses auto-discovery) |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `true` | Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--reverse` |  | bool | `false` | Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:') |
| `--service` |  | string | - | Compose service to map port to (default: localhost) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent forward via`

Forward a port between this machine and a container on a remote Docker host

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--ephemeral` |  | bool | `true` | Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--reverse` |  | bool | `false` | Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:') |
| `--service` |  | string | - | Compose service to map port to (default: localhost) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent ssh`

Open an SSH session into the project's devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `true` | Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent ssh via`

Open an SSH session into a container on a remote Docker host

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--ephemeral` |  | bool | `true` | Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (--ephemeral=false opts into ~/.ssh/devcontainer-cli.config) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config`

Manage global configuration, backups, SSH keys, aliases and skills

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config backup`

Export global config and profiles to a tar.gz archive

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config clean`

Remove orphaned managed state (e.g. SSH host blocks for deleted containers)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config clean ssh`

Remove managed SSH host blocks whose containers no longer exist

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--dry-run` |  | bool | `false` | List orphaned blocks without modifying ~/.ssh/devcontainer-cli.config |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config export`

Export all global configuration and profiles to stdout as JSON

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config import`

Import global configuration and profiles from a JSON export

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--merge` |  | bool | `false` | Keep existing profiles/keys when names collide instead of overwriting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config restore`

Restore global config and profiles from a backup archive

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--force` |  | bool | `false` | Overwrite existing config and profiles without prompting |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config set`

Read or update a global default (e.g. config set defaults.editor code)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill`

Manage global and per-project agent skills

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill add`

Add a custom skill from a GitHub repo or a local directory to the global catalogue

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--description` |  | string | - | Short description shown in the wizard and 'agent cli-info' |
| `--id` |  | string | - | Override the skill id (default: derived from directory name) |
| `--label` |  | string | - | Display label shown in the wizard |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--requires` |  | string | - | Comma-separated base-image module ids the skill needs (e.g. nodejs) |
| `--skill` |  | string | - | Subdirectory / skill name inside a multi-skill repository |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill install`

Install one or more skills into the current project (or globally with --global)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--cwd` |  | string | - | Project directory (default: current working directory) |
| `--force` |  | bool | `false` | Re-download and overwrite an already-installed skill |
| `--global` |  | bool | `false` | Install into ~/.agents/skills and ~/.claude/skills instead of the project |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill list`

List built-in, user-added, and project-installed agent skills

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--cwd` |  | string | - | Project directory (default: current working directory) |
| `--json` |  | bool | `false` | Emit JSON instead of a human-readable table |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill remove`

Remove a custom skill from the global catalogue (or uninstall from a project with --installed)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--cwd` |  | string | - | Project directory (default: current working directory) |
| `--global` |  | bool | `false` | Uninstall from ~/.agents/skills and ~/.claude/skills (with --installed) |
| `--installed` |  | bool | `false` | Uninstall from the project (or --global) instead of removing from the catalogue |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config ssh-alias`

Manage custom SSH host aliases on top of a devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config ssh-alias add`

Create a custom SSH alias pointing to a devcontainer (e.g. for Coolify / Traefik)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Target devcontainer name |
| `--force` |  | bool | `false` | Overwrite an existing alias without prompting |
| `--host` |  | string | - | Hostname or IP the alias connects to (default: container's live IP) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--port` | -p | int | `22` | SSH port on the target host |
| `--user` | -u | string | `devuser` | SSH user |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config ssh-alias list`

List custom SSH host aliases

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config ssh-alias remove`

Remove a custom SSH host alias

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config ssh-key`

Manage the shared SSH key used to reach devcontainers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--export` |  | string | - | Emit a self-contained SSH config block + private key for a remote client reaching this Docker host as USER@HOST |
| `--generate` |  | bool | `false` | Generate a new shared key pair if one does not exist yet |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--reset` |  | bool | `false` | Regenerate the shared key pair, overwriting any existing key (invalidates existing containers until re-run) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli context`

Show the active workspace, compose project, services and paths

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--json` |  | bool | `false` | Emit JSON instead of a human-readable summary |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli info`

Show installed devcontainer-cli version and paths

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect`

Inspect managed containers, compose services, logs and workspace paths

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect logs`

Tail logs from the devcontainer or a compose service

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--follow` | -f | bool | `false` | Follow log output |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--service` |  | string | - | Compose service name (default: devcontainer) |
| `--tail` | -n | int | `100` | Number of lines to show from the end of the logs |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect ls`

List devcontainer-cli managed containers on this host

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` | -a | bool | `false` | Include stopped containers |
| `--container` | -c | string | - | Filter to a single container name |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--long` | -l | bool | `false` | Show extended columns (image, ports, networks, compose project) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect status`

Show running status and SSH access configuration for the workspace

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect workdir`

Print the project's mounted workspace directory inside the container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli run`

Launch a throwaway container from an image catalog entry or profile

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--apt` |  | string | - | Comma-separated extra apt packages |
| `--command` |  | string | - | Command to run inside the container instead of an interactive shell |
| `--image` |  | string | - | Base image id, optionally with :version (e.g. node:22, python:3.12, go) |
| `--keep` |  | bool | `false` | Keep the container after exit instead of removing it with --rm |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--mount` | -m | string | - | Mount a host path into /workspace (defaults to the current directory; pass 'none' to skip) |
| `--name` |  | string | - | Explicit container name (default: ephemeral dc-<image>-<pid>) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--npm` |  | string | - | Comma-separated extra global npm packages |
| `--pip` |  | string | - | Comma-separated extra pip packages |
| `--profile` |  | string | - | Saved profile name from ~/.config/devcontainer-cli/profiles/ |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli update`

Check for and install a newer devcontainer-cli release from GitHub

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--check` |  | bool | `false` | Only check whether a newer release is available without installing |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli workspace`

Generate, build, start, stop and destroy devcontainer workspaces

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace compose`

Run a docker compose subcommand against the project's generated compose file

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace down`

Stop the project's running containers without removing them

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace generate`

Interactively generate .devcontainer/ and docker-compose.yml for a project

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--apt` |  | string | - | Comma-separated extra apt packages |
| `--build` |  | bool | `false` | Build the Docker image immediately after generating |
| `--cwd` |  | string | - | Target directory (default: current working directory) |
| `--force` |  | bool | `false` | Overwrite existing devcontainer files |
| `--forward-ports` |  | string | - | Comma-separated host ports to forward on 'access forward' (e.g. '3000,8080:80') |
| `--image` |  | string | - | Base image id, optionally with :version (e.g. node:22, python:3.12, go) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--name` |  | string | - | Workspace / compose project name (default: directory name) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--npm` |  | string | - | Comma-separated extra global npm packages |
| `--pip` |  | string | - | Comma-separated extra pip packages |
| `--profile` |  | string | - | Saved profile name from ~/.config/devcontainer-cli/profiles/ |
| `--services` |  | string | - | Comma-separated compose service ids (e.g. postgres,redis) |
| `--skills` |  | string | - | Comma-separated agent skill ids (see 'agent cli-info --json') |
| `--up` |  | bool | `false` | Build and start the workspace immediately after generating |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace init`

Create a blank workspace directory and launch the wizard inside it

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace profile`

Save, inspect, apply, export, import and delete reusable workspace profiles

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace profile apply`

Apply a saved profile to the current project

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--cwd` |  | string | - | Project directory (default: current working directory) |
| `--force` |  | bool | `false` | Overwrite existing devcontainer files |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace profile delete`

Delete a saved profile

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli workspace profile export`

Export a saved profile to stdout as YAML

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace profile import`

Import a profile from a YAML file

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--force` |  | bool | `false` | Overwrite an existing profile with the same name |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--name` |  | string | - | Override the profile name |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace profile list`

List saved profiles in ~/.config/devcontainer-cli/profiles/

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace profile save`

Save the current project's configuration as a reusable profile

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--cwd` |  | string | - | Project directory (default: current working directory) |
| `--description` |  | string | - | Optional description shown in 'profile list' |
| `--force` |  | bool | `false` | Overwrite an existing profile with the same name |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace profile show`

Print the YAML of a saved profile

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace sync`

Regenerate .devcontainer/ and docker-compose.yml from devcontainer.yml

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--build` |  | bool | `false` | Rebuild the Docker image after syncing |
| `--cwd` |  | string | - | Project directory (default: current working directory) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--up` |  | bool | `false` | Rebuild and restart the workspace after syncing |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace up`

Build (if needed) and start the project's containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--build` |  | bool | `false` | Force rebuild of the image before starting |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace destroy`

Stop and remove the project's containers, network and named volumes

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--volumes` | -v | bool | `false` | Also remove named volumes (database data, caches) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli workspace upgrade`

Rebuild the image from scratch without cache and recreate the container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
