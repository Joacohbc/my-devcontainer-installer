# Referencia Automática de Comandos y Banderas

> Este documento es generado automáticamente a partir del árbol de comandos de Cobra de `devcontainer-cli`.
> Para actualizarlo tras modificar comandos o banderas, ejecute: `UPDATE_CLI_REF=1 go test ./internal/cli/commands -run TestCLIReference_UpToDate`.

## `devcontainer-cli`

Generate and manage reproducible Docker devcontainers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--build` |  | bool | `false` | Run 'docker compose build/pull' after generating |
| `--force` |  | bool | `false` | Overwrite existing files without prompting |
| `--force-prompt` |  | bool | `false` | Prompt even if config file exists |
| `--forward-ports` |  | string | - | Ports 'access forward' tunnels when called with no argument (e.g. 3000,8080:80, reverse:5432 for a host port reachable inside the container); 'none' clears them |
| `--image` |  | string | - | Image to run under mode=image: an image definition name (see 'image list'; a build definition must be built first, a remote one is pulled and verified) or a registry reference. Ignored for mode=custom, whose image is named by origin and content: devcontainer-cli/<profiles>-<hash> |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--mode` |  | string | - | Build mode: custom (default, full local build), profiles (pull a prebuilt image), or image (use a reusable image) |
| `--modules` |  | string | - | Comma-separated dockerfile modules (e.g. nodejs,golang,rust) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--ports` |  | string | - | Ports to publish on the devcontainer (e.g. 8080:80,5432:5432); bound to 127.0.0.1 unless an IP is given; 'none' clears them |
| `--profile` |  | string | - | Apply profile(s): comma-separated module bundles for mode=custom (e.g. 'nodejs,my-tools'), or a single pull target for mode=profiles (must be [remote]-tagged). See 'profile list'. |
| `--registry` |  | string | - | Container registry prefix for remote images (overrides global) |
| `--script` |  | stringArray | `[]` | Custom script to add, as <path>[:build\|start\|service\|manual] (default build); repeatable. build bakes it into the image, start runs once per container, service runs as background service on every start, manual only copies it to ~/post-script/ |
| `--services` |  | string | - | Comma-separated compose services (e.g. mongo,postgres,redis) |
| `--shared-config` |  | bool | `true` | Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out |
| `--skills` |  | string | - | Comma-separated agent skills installed into the project workspace; implies the nodejs module. See 'config skill list' (built-in + your own) |
| `--skills-mode` |  | string | - | How the project's agent skills get installed: manual (default — you run 'install-skills') or auto (on every container start, writing into the workspace unprompted) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--version` |  | bool | `false` | Print the CLI version |
| `--volumes` |  | string | - | Extra volume mounts on the devcontainer (e.g. myvol:/data,./cache:/cache); 'none' clears them |
| `--workspace` |  | string | - | Workspace name (default: current dir name) |

## `devcontainer-cli access`

Connect to and interact with running devcontainers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access copy`

Copy files or directories between the host and the container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--asset` |  | string | - | Copy a built-in script asset into the container instead of a local path |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access forward`

Forward host ports into a running container over SSH

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--alias` |  | string | - | SSH host alias to use (bypasses auto-discovery) |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `false` | Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--reverse` |  | bool | `false` | Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:') |
| `--service` |  | string | - | Compose service to map port to (default: localhost) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access network`

Attach or detach containers to the workspace network

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access network connect`

Attach containers to the workspace network

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--alias` |  | stringSlice | `[]` | Extra DNS alias(es) to register for the container(s) on the network; repeatable or comma-separated |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access network disconnect`

Detach containers from the workspace network

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access password`

Show or change the devuser password in a container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access password change`

Change the devuser password in a container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--password` |  | string | - | New password (non-interactive; omit to be prompted) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access password show`

Show the initial devuser password for a container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route`

Manage local domain routing for devcontainers via global Caddy proxy

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route add`

Route a domain to a container port in foreground

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--domain` |  | string | - | Subdomain to route (e.g. 'api' -> 'api.devcli.localhost') |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route ls`

List active domain routes

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route rm`

Remove a domain route from the router

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route status`

Show status of the global Caddy router

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access route stop`

Stop and remove the global Caddy router container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli access shell`

Open a shell (or run a command) in the devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--no-tty` | -T | bool | `false` | Disable pseudo-TTY allocation (use when piping output to a file, e.g. a DB dump); implied when stdin is not a terminal |
| `--type` |  | string | - | Shell to open: bash, zsh or sh (interactive shell defaults to zsh) |
| `--user` |  | string | - | User to run the command as; always honoured, but only an interactive shell defaults it to devuser |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--via` |  | string | - | Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias |
| `--workdir` | -w | bool | `false` | Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off |

## `devcontainer-cli access ssh`

Open a real SSH session into the devcontainer, configuring access first if needed

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `false` | Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks |
| `--forward` |  | bool | `false` | Also open SSH port-forwarding tunnels for the session (prompted when interactive and omitted) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--ports` |  | string | - | Ports to forward, e.g. '3000,8080:80' (implies --forward; skips the prompt). A 'reverse:' prefix on a port makes the container reach that port on this machine instead |
| `--setup` |  | bool | `false` | Re-run the SSH access setup (key + Host block) before connecting, even if an alias already exists |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--via` |  | string | - | Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli agent`

Drive the CLI from an AI agent with a small, non-interactive command set

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent cli-info`

Print the catalogue an 'agent create' is composed from

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--json` |  | bool | `false` | Emit the catalogue as JSON instead of the human-readable report |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent create`

Generate, build and start a devcontainer for the current directory

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--forward-ports` |  | string | - | Ports 'access forward' tunnels when called with no argument (e.g. 3000,8080:80, reverse:5432 for a host port reachable inside the container); 'none' clears them |
| `--image` |  | string | - | Image to run under mode=image: an image definition name (see 'image list'; a build definition must be built first, a remote one is pulled and verified) or a registry reference. Ignored for mode=custom, whose image is named by origin and content: devcontainer-cli/<profiles>-<hash> |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--mode` |  | string | - | Build mode: custom (default, full local build), profiles (pull a prebuilt image), or image (use a reusable image) |
| `--modules` |  | string | - | Comma-separated dockerfile modules (e.g. nodejs,golang,rust) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--no-up` |  | bool | `false` | Generate and build only; leave the containers stopped |
| `--ports` |  | string | - | Ports to publish on the devcontainer (e.g. 8080:80,5432:5432); bound to 127.0.0.1 unless an IP is given; 'none' clears them |
| `--profile` |  | string | - | Apply profile(s): comma-separated module bundles for mode=custom (e.g. 'nodejs,my-tools'), or a single pull target for mode=profiles (must be [remote]-tagged). See 'profile list'. |
| `--registry` |  | string | - | Container registry prefix for remote images (overrides global) |
| `--script` |  | stringArray | `[]` | Custom script to add, as <path>[:build\|start\|service\|manual] (default build); repeatable. build bakes it into the image, start runs once per container, service runs as background service on every start, manual only copies it to ~/post-script/ |
| `--services` |  | string | - | Comma-separated compose services (e.g. mongo,postgres,redis) |
| `--shared-config` |  | bool | `true` | Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out |
| `--skills` |  | string | - | Comma-separated agent skills installed into the project workspace; implies the nodejs module. See 'config skill list' (built-in + your own) |
| `--skills-mode` |  | string | - | How the project's agent skills get installed: manual (default — you run 'install-skills') or auto (on every container start, writing into the workspace unprompted) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--volumes` |  | string | - | Extra volume mounts on the devcontainer (e.g. myvol:/data,./cache:/cache); 'none' clears them |
| `--workspace` |  | string | - | Workspace name (default: current dir name) |

## `devcontainer-cli agent destroy`

Destroy the current project and remove what it left behind

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` |  | bool | `false` | Also remove every OTHER managed container, image, network and volume on this machine |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--dry-run` |  | bool | `false` | Report what would be removed without removing anything |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

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
| `--via` |  | string | - | Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias |
| `--workdir` | -w | bool | `false` | Run in the project's workspace mount instead of the container default; only the devcontainer has that directory, so a database container needs it left off |

## `devcontainer-cli agent forward`

Forward a port between this machine and the running container over SSH

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--alias` |  | string | - | SSH host alias to use (bypasses auto-discovery) |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `false` | Forward directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--reverse` |  | bool | `false` | Reverse every mapping without an explicit prefix: the container listens and reaches a service on this machine (a single mapping can still opt in with 'reverse:') |
| `--service` |  | string | - | Compose service to map port to (default: localhost) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent list`

List files inside the devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` | -a | bool | `false` | Show hidden files (ls -a) |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--json` |  | bool | `false` | Emit one JSON object per entry instead of the raw ls listing |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli agent run`

Spin up an ephemeral container from a prebuilt or named image (no project files)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--expose-all` |  | bool | `false` | Publish ports on all interfaces (0.0.0.0) instead of binding to 127.0.0.1 |
| `--image` |  | string | - | Image definition to run instead of a profile (see 'image list'); a build definition must be built first, a remote one is pulled and verified |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--name` |  | string | - | Container name (default: dc-<profile> or dc-<image>); if it belongs to a quick-run container started from the same source it is reused/restarted, but a name already used by any other container is rejected |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--ports` |  | stringSlice | `[]` | Port mappings (e.g. 2222:22); bound to 127.0.0.1 unless --expose-all; repeatable or comma-separated |
| `--profile` |  | string | - | Single [remote]-tagged profile id to run as a prebuilt image, e.g. base, nodejs, python (it or --image is required); see 'profile list' |
| `--registry` |  | string | - | Registry prefix override for --profile |
| `--shared-config` |  | bool | `true` | Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--volumes` |  | stringSlice | `[]` | Volume mounts (e.g. myvol:/workspace); repeatable or comma-separated |

## `devcontainer-cli agent ssh`

Open an SSH session into the project's devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--ephemeral` |  | bool | `false` | Connect directly via SSH without modifying ~/.ssh/config or relying on existing Host blocks (default true for agent) |
| `--key` |  | string | - | Private key path (default: the shared managed key under the CLI config dir) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--user` |  | string | `devuser` | SSH user inside the container |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--via` |  | string | - | Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias |

## `devcontainer-cli config`

Manage global CLI settings, skills, shared volume, and maintenance

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config alias`

Manage your own shell aliases for every container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config alias list`

List configured shell aliases

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config alias set`

Add or update an alias in the CLI config

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config alias unset`

Remove an alias from the CLI config

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config clean`

Clean up CLI-managed resources and stale configuration entries

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--dry-run` |  | bool | `false` | Preview items that would be removed without executing |
| `--include-active` |  | bool | `false` | Remove active/running resources in addition to unused ones |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--shared` |  | bool | `false` | Also remove the global shared tool-config volume (devcontainer-shared-config) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config clean all`

Sweep every category (catalog, containers, images, ssh, networks, volumes)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--dry-run` |  | bool | `false` | List items that would be removed without executing |
| `--include-active` |  | bool | `false` | Remove active/running resources in addition to unused ones |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--shared` |  | bool | `false` | Also remove global shared tool-config volume (devcontainer-shared-config) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config clean catalog`

Prune entries in images.json whose project directory no longer exists

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--dry-run` |  | bool | `false` | List stale catalog entries without removing them |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config clean containers`

Remove CLI-managed containers (by name, non-running, or --include-active)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--include-active` |  | bool | `false` | Remove active/running resources in addition to unused ones |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config clean images`

Remove CLI-managed images (by reference, unused, or --include-active)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--include-active` |  | bool | `false` | Remove active/running resources in addition to unused ones |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config clean networks`

Remove CLI-managed Docker networks

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--include-active` |  | bool | `false` | Remove active/running resources in addition to unused ones |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config clean ssh`

Prune stale SSH config blocks and their pinned host keys

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--dry-run` |  | bool | `false` | List stale blocks and orphaned host keys without removing them |
| `--include-active` |  | bool | `false` | Also list every managed block regardless of status, including alive ones (never auto-removed by --yes) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config clean volumes`

Remove CLI-managed Docker volumes (--shared includes shared tool config)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--include-active` |  | bool | `false` | Remove active/running resources in addition to unused ones |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--shared` |  | bool | `false` | Also remove global shared tool-config volume (devcontainer-shared-config) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config db-password`

Get, set or unset the default DB password

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--unset` |  | bool | `false` | Remove the db-password key (revert to default) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config db-user`

Get, set or unset the default DB user

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--unset` |  | bool | `false` | Remove the db-user key (revert to default) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config git-init`

Get, set or unset the default 'git init' offer for a project directory that is not a repository yet (true/false, default false)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--unset` |  | bool | `false` | Remove the git-init key (revert to default) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config registry`

Get, set or unset the default image registry

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--unset` |  | bool | `false` | Remove the registry key (revert to default) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config shared`

Manage the shared tool-config volume (logins/sessions reused by every container)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config shared backup`

Back up the shared config volume to a zip file

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--output` | -o | string | - | Destination zip file (default: shared-config-backup-<timestamp>.zip in the current directory) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config shared restore`

Restore the shared config volume from a zip backup

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--force` |  | bool | `false` | Replace entries that already have data in the volume with the zip's copy |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config shared sync`

Seed the shared config volume from this machine's tool configs

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--force` |  | bool | `false` | Replace entries that already have data in the volume with the host copy |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config skill`

List built-in and user-defined agent skills

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill add`

Add a user-defined agent skill or discover skills from a source

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` |  | bool | `false` | Select all discovered skills from source |
| `--context-body` |  | string | - | Body of its ~/CONTEXT.md entry (optional; a title with no body is dropped) |
| `--context-title` |  | string | - | Title of its ~/CONTEXT.md entry (optional; defaults to the label or id) |
| `--force` |  | bool | `false` | Overwrite an existing user-defined skill with the same id |
| `--label` |  | string | - | Human-readable label |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--pick` |  | string | - | Comma-separated list of discovered skill names or IDs to save |
| `--ref` |  | string | - | Install source the Skills CLI resolves (owner/repo, or a repository URL) |
| `--requires-modules` |  | string | - | Comma-separated Dockerfile module ids this skill's tooling needs |
| `--skill` |  | string | - | Skill selector, only needed when --ref holds more than one skill |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill info`

Show one agent skill's full resolved definition

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill install`

Install agent skills at project level in an ephemeral container

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` |  | bool | `false` | Install all available agent skills (built-in and user-defined) |
| `--dir` |  | string | - | Target project directory (default: current directory) |
| `--image` |  | string | `ghcr.io/joacohbc/devcontainer-utils:latest` | Docker image to run the skill installer in |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill list`

List built-in and user-defined agent skills

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config skill remove`

Delete user-defined agent skills (built-ins cannot be removed)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli config ssh-config-file`

Get, set or unset the default CLI-managed SSH config file

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--unset` |  | bool | `false` | Remove the ssh-config-file key (revert to default) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config ssh-key`

Manage the single shared SSH key reused by every devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--export` |  | string | - | Print an SSH setup snippet for an external host (user@host or host) |
| `--generate` |  | bool | `false` | Generate the managed key now if it does not exist |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--private` |  | bool | `false` | Print the private key contents |
| `--public` |  | bool | `false` | Print the public key contents |
| `--unset` |  | bool | `false` | Remove the ssh-key path (revert to default) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli config upgrade-cli`

Self-update the CLI binary from the latest GitHub release

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--check` |  | bool | `false` | Print current vs latest and exit (no download) |
| `--force` |  | bool | `false` | Reinstall even if the current version matches latest |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--pre-release` |  | bool | `false` | Install the latest pre-release (testing) if it is newer |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image`

Manage reusable, named devcontainer images

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image build`

Build the Docker image for an image definition

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image create`

Create a new reusable image definition

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--from` |  | string | - | Derive from another image definition or a devcontainer-cli registry image, installing only the missing modules |
| `--from-profile` |  | string | - | Seed the image modules and scripts from an existing profile |
| `--label` |  | string | - | Human-readable description label |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--modules` |  | string | - | Comma-separated dockerfile modules for the image (replaces the profile's or the github-cli default) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--remote` |  | string | - | Point image definition directly to a remote registry image reference |
| `--tag` |  | string | `latest` | Docker tag for the image |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image export`

Export image Dockerfile and assets to a directory

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image info`

Show detailed status of an image definition

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image list`

List registered devcontainer image definitions

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image pull`

Pull a remote image and verify compatibility

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image remove`

Remove an image definition

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--keep-docker-image` |  | bool | `false` | Keep the built Docker image in the daemon, removing only definition |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli image update`

Update (pull or rebuild) a project's container images

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` |  | bool | `false` | Update images for every project tracked in images.json |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--pull` |  | bool | `false` | Always pull (no-op for mode=custom without rebuild) |
| `--rebuild` |  | bool | `false` | Always rebuild (no-op for mode=profiles) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect`

Inspect container status, logs, files, and context

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect context`

Print the container's context and installed tools

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--json` |  | bool | `false` | Emit structured JSON instead of the human-readable report |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect info`

Show detailed info for the project's containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect logs`

View logs from the project's containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--follow` | -f | bool | `false` | Follow log output |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--tail` |  | string | `all` | Number of lines to show from the end of the logs |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect ls`

List files inside the devcontainer

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` | -a | bool | `false` | Show hidden files (ls -a) |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--long` | -l | bool | `false` | Use a long listing format (ls -l) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli inspect status`

Show a status table for the project's containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--all` |  | bool | `false` | Show all CLI-managed containers across all workspaces |
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli profile`

Manage reusable devcontainer profiles

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli profile copy`

Copy any profile into a new user profile

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli profile create`

Create a user profile via an interactive picker

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli profile info`

Show one profile's full resolved definition

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli profile list`

List built-in and user profiles with their modules

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli profile remove`

Delete user profiles (built-ins cannot be removed)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli run`

Spin up a container from a prebuilt or named image (no project files)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--expose-all` |  | bool | `false` | Publish ports on all interfaces (0.0.0.0) instead of binding to 127.0.0.1 |
| `--image` |  | string | - | Image definition to run instead of a profile (see 'image list'); a build definition must be built first, a remote one is pulled and verified |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--name` |  | string | - | Container name (default: dc-<profile> or dc-<image>); if it belongs to a quick-run container started from the same source it is reused/restarted, but a name already used by any other container is rejected |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--ports` |  | stringSlice | `[]` | Port mappings (e.g. 2222:22); bound to 127.0.0.1 unless --expose-all; repeatable or comma-separated |
| `--profile` |  | string | - | Single [remote]-tagged profile id to run as a prebuilt image, e.g. base, nodejs, python (it or --image is required with --no-interactive); see 'profile list' |
| `--registry` |  | string | - | Registry prefix override for --profile |
| `--shared-config` |  | bool | `true` | Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--volumes` |  | stringSlice | `[]` | Volume mounts (e.g. myvol:/workspace); repeatable or comma-separated |

## `devcontainer-cli workspace`

Manage project workspace lifecycle and containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace auto-init`

Auto-initialize a devcontainer by detecting project stack or cloning a repository

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--build` |  | bool | `true` | Build or pull container images after generating |
| `--forward-ports` |  | string | - | Ports 'access forward' tunnels by default (a 'reverse:' prefix tunnels a host port into the container) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--mode` |  | string | - | Build mode: custom (default) or profiles (pull prebuilt image) |
| `--modules` |  | string | - | Additional dockerfile modules (e.g. nodejs,golang,rust) |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--no-up` |  | bool | `false` | Generate and build only; leave containers stopped |
| `--ports` |  | string | - | Ports to publish on the devcontainer (e.g. 8080:80,5432:5432) |
| `--profile` |  | string | - | Apply profile bundle(s) (comma-separated for custom builds) |
| `--registry` |  | string | - | Container registry prefix for remote images |
| `--services` |  | string | - | Additional compose services (e.g. mongo,postgres,redis) |
| `--shared-config` |  | bool | `true` | Mount global shared tool config volume |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--volumes` |  | string | - | Extra volume mounts on the devcontainer |
| `--workspace` |  | string | - | Workspace name (default: directory name or repo name) |

## `devcontainer-cli workspace compose`

Run any docker compose command against the project's stack

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace destroy`

Tear down the project completely and delete its generated files

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli workspace down`

Stop and remove the project's containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--volumes` | -v | bool | `false` | Also remove named volumes (down -v); deletes their data, irreversible |
| `--yes` | -y | bool | `false` | Skip confirmation prompts |

## `devcontainer-cli workspace export`

Export the current project's config as YAML

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--output` | -o | string | - | Output file (default stdout) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace import`

Import a YAML config into devcontainer.config.json

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--force` |  | bool | `false` | Overwrite existing devcontainer.config.json without prompt |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace init`

Create a devcontainer for the current project with the interactive wizard

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--build` |  | bool | `false` | Run 'docker compose build/pull' after generating |
| `--force` |  | bool | `false` | Overwrite existing files without prompting |
| `--force-prompt` |  | bool | `false` | Prompt even if config file exists |
| `--forward-ports` |  | string | - | Ports 'access forward' tunnels when called with no argument (e.g. 3000,8080:80, reverse:5432 for a host port reachable inside the container); 'none' clears them |
| `--image` |  | string | - | Image to run under mode=image: an image definition name (see 'image list'; a build definition must be built first, a remote one is pulled and verified) or a registry reference. Ignored for mode=custom, whose image is named by origin and content: devcontainer-cli/<profiles>-<hash> |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--mode` |  | string | - | Build mode: custom (default, full local build), profiles (pull a prebuilt image), or image (use a reusable image) |
| `--modules` |  | string | - | Comma-separated dockerfile modules (e.g. nodejs,golang,rust) |
| `--no-interactive` |  | bool | `false` | Non-interactive mode (fail instead of prompting) |
| `--ports` |  | string | - | Ports to publish on the devcontainer (e.g. 8080:80,5432:5432); bound to 127.0.0.1 unless an IP is given; 'none' clears them |
| `--profile` |  | string | - | Apply profile(s): comma-separated module bundles for mode=custom (e.g. 'nodejs,my-tools'), or a single pull target for mode=profiles (must be [remote]-tagged). See 'profile list'. |
| `--registry` |  | string | - | Container registry prefix for remote images (overrides global) |
| `--script` |  | stringArray | `[]` | Custom script to add, as <path>[:build\|start\|service\|manual] (default build); repeatable. build bakes it into the image, start runs once per container, service runs as background service on every start, manual only copies it to ~/post-script/ |
| `--services` |  | string | - | Comma-separated compose services (e.g. mongo,postgres,redis) |
| `--shared-config` |  | bool | `true` | Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out |
| `--skills` |  | string | - | Comma-separated agent skills installed into the project workspace; implies the nodejs module. See 'config skill list' (built-in + your own) |
| `--skills-mode` |  | string | - | How the project's agent skills get installed: manual (default — you run 'install-skills') or auto (on every container start, writing into the workspace unprompted) |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |
| `--volumes` |  | string | - | Extra volume mounts on the devcontainer (e.g. myvol:/data,./cache:/cache); 'none' clears them |
| `--workspace` |  | string | - | Workspace name (default: current dir name) |

## `devcontainer-cli workspace restart`

Restart the project's already-created containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace start`

Start the project's stopped containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace stop`

Stop the project's running containers (without removing them)

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--container` | -c | string | - | Explicit target container name (managed by devcontainer-cli) |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

## `devcontainer-cli workspace up`

Create and start the project's containers

### Banderas

| Bandera | Atajo | Tipo | Por defecto | Descripción |
|---|---|---|---|---|
| `--build` |  | bool | `false` | Build images before starting (docker compose up -d --build); use after changing the Dockerfile or modules |
| `--log-level` |  | string | `warn` | Log level: debug\|info\|warn\|error |
| `--no-interactive` |  | bool | `false` | Fail if any value is missing instead of prompting |
| `--verbose` |  | bool | `false` | Enable debug logging (shortcut for --log-level debug) |

