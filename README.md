<div align="center">

# DevContainer Installer CLI

[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](#)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](#)
[![Linux](https://img.shields.io/badge/Linux-FCC624?logo=linux&logoColor=black)](#)
[![macOS](https://img.shields.io/badge/macOS-000000?logo=apple&logoColor=white)](#)

**Crea y gestiona entornos de desarrollo aislados en Docker desde un solo comando, sin instalar toolchains ni bases de datos en tu máquina.**

</div>

---

## ¿Qué es y para qué sirve?

`devcontainer-cli` genera, construye y opera contenedores Docker listos para programar o ejecutar tareas automatizadas.

Úsalo para:
- **Aislar proyectos**: cada repositorio tiene sus propias versiones de Node, Python, Go, Java, Rust o PHP y sus bases de datos (`postgres`, `redis`, `mongo`) sin conflictos en tu sistema.
- **Conectar tu editor o terminal en segundos**: configura acceso SSH automático para **VS Code Remote - SSH** o entra directo con `access shell` y `zellij`.
- **Probar cosas sin ensuciar tu equipo**: levanta un contenedor desechable con `run --profile <perfil>` y bórralo al terminar.
- **Darle un sandbox seguro a agentes de IA**: Claude Code, Antigravity, Codex, OpenCode y Copilot ejecutan builds, tests y comandos dentro del contenedor sin acceso `root` al host.

---

## Requisitos e Instalación

Requiere **Docker** en ejecución (Linux o macOS, `x64` / `arm64`).

```bash
curl -fsSL https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/install.sh | sh
```

> Binarios manuales disponibles en [GitHub Releases](https://github.com/Joacohbc/my-devcontainer-installer/releases/latest). El autocompletado para Zsh y Bash se configura al instalar.

---

## Primeros pasos (1 minuto)

### Opción A: Crear un entorno para tu proyecto

```bash
cd mi-proyecto

# 1. Elige lenguajes, bases de datos y herramientas en el asistente interactivo
devcontainer-cli

# 2. Levanta los contenedores
devcontainer-cli workspace up

# 3. Conéctate (elige SSH para VS Code/terminal o shell directa)
devcontainer-cli access ssh
# o:
devcontainer-cli access shell
```

Si quieres saltar la compilación local, usa una imagen precompilada de GHCR:

```bash
devcontainer-cli --profile nodejs
```

### Opción B: Contenedor temporal sin archivos de proyecto

```bash
devcontainer-cli run --profile python
```

---

## Uso con Agentes de IA

`devcontainer-cli` incluye el grupo `devcontainer-cli agent` (sin menús interactivos ni prompts) para que un asistente que corre en tu máquina ejecute código dentro del contenedor como `devuser`:

```bash
devcontainer-cli agent create --mode profiles --profile node-python --services postgres
devcontainer-cli agent exec -w -- go test ./...
devcontainer-cli agent forward 3000
devcontainer-cli agent destroy --yes
```

### Instalar la habilidad (skill) en tu agente del host (muy recomendado)

Enseña a Claude Code, Antigravity u otros agentes compatibles a usar `devcontainer-cli` automáticamente para no tocar tu máquina física:

```bash
# Con npx skills:
npx skills add Joacohbc/my-devcontainer-installer@devcontainer-cli -g
```

Sin `npx` (descarga directa de la carpeta de la habilidad):

```bash
# ~/.claude/skills/devcontainer-cli (Claude Code) o ~/.agents/skills/devcontainer-cli (Antigravity / OpenCode)
DEST=~/.claude/skills/devcontainer-cli
mkdir -p "$DEST" && \
curl -fsSL https://codeload.github.com/Joacohbc/my-devcontainer-installer/tar.gz/main | \
  tar -xz -C "$DEST" --strip-components=3 my-devcontainer-installer-main/skills/devcontainer-cli
```

---

## Componentes y Tecnologías Soportadas

| Categoría | Tecnologías y Herramientas | Descripción |
| :--- | :--- | :--- |
| **Runtimes y Lenguajes** | [![Python](https://img.shields.io/badge/Python-3776AB?logo=python&logoColor=white)](#) [![Node.js](https://img.shields.io/badge/Node.js-6DA55F?logo=node.js&logoColor=white)](#) [![Bun](https://img.shields.io/badge/Bun-000000?logo=bun&logoColor=white)](#) [![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](#) [![Rust](https://img.shields.io/badge/Rust-000000?logo=rust&logoColor=white)](#) [![Java](https://img.shields.io/badge/Java-ED8B00?logo=openjdk&logoColor=white)](#) [![PHP](https://img.shields.io/badge/PHP-777BB4?logo=php&logoColor=white)](#) [![C++](https://img.shields.io/badge/C%2B%2B-00599C?logo=c%2B%2B&logoColor=white)](#) | Módulos dinámicos para Dockerfile con versiones configurables (`nvm`/`fnm`, Corepack, `rustup`, OpenJDK/Temurin, CMake). |
| **Gestores de Paquetes** | [![npm](https://img.shields.io/badge/npm-CB3837?logo=npm&logoColor=white)](#) [![pnpm](https://img.shields.io/badge/pnpm-F69220?logo=pnpm&logoColor=white)](#) [![Yarn](https://img.shields.io/badge/Yarn-2C8EBB?logo=yarn&logoColor=white)](#) [![pip](https://img.shields.io/badge/pip-3776AB?logo=pypi&logoColor=white)](#) [![uv](https://img.shields.io/badge/uv-DE5FE9?logo=astral&logoColor=white)](#) | Integración nativa con los gestores de paquetes más populares. |
| **Bases de Datos** | [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql&logoColor=white)](#) [![MongoDB](https://img.shields.io/badge/MongoDB-47A248?logo=mongodb&logoColor=white)](#) [![Redis](https://img.shields.io/badge/Redis-DC382D?logo=redis&logoColor=white)](#) [![SQLite](https://img.shields.io/badge/SQLite-003B57?logo=sqlite&logoColor=white)](#) | Servicios orquestados en red privada (`postgres`, `mongo`, `redis`) o cliente local (`sqlite3`). Incluye clientes CLI opcionales (`psql`, `mongosh`, `redis-cli`). |
| **Agentes de IA y Herramientas** | [![Claude Code](https://img.shields.io/badge/Claude_Code-D97757?logo=claude&logoColor=white)](#) [![GitHub Copilot](https://img.shields.io/badge/GitHub_Copilot-000000?logo=githubcopilot&logoColor=white)](#) [![Codex CLI](https://img.shields.io/badge/Codex_CLI-10A37F?logo=openai&logoColor=white)](#) [![OpenCode](https://img.shields.io/badge/OpenCode-4B32C6?logo=gnubash&logoColor=white)](#) [![Antigravity CLI](https://img.shields.io/badge/Antigravity_CLI-4285F4?logo=google&logoColor=white)](#) [![Herdr](https://img.shields.io/badge/Herdr-111827?logo=gnu-bash&logoColor=white)](#) | CLIs e instaladores integrados para Claude Code, Copilot CLI, Codex CLI, OpenCode, Antigravity CLI y Herdr. |
| **Herramientas de Sistema** | [![Docker](https://img.shields.io/badge/Docker_DoD-2496ED?logo=docker&logoColor=white)](#) [![GitHub CLI](https://img.shields.io/badge/GitHub_CLI-181717?logo=github&logoColor=white)](#) [![Chromium](https://img.shields.io/badge/Chromium-4285F4?logo=googlechrome&logoColor=white)](#) [![FFmpeg](https://img.shields.io/badge/FFmpeg-007808?logo=ffmpeg&logoColor=white)](#) | Docker-out-of-Docker (`/var/run/docker.sock`), `gh`, Chromium headless para testing/web scraping y FFmpeg para procesamiento multimedia. |
| **Red y Conectividad** | [![OpenSSH](https://img.shields.io/badge/OpenSSH-000000?logo=openssh&logoColor=white)](#) [![Cloudflare](https://img.shields.io/badge/Cloudflare_Tunnel-F38020?logo=cloudflare&logoColor=white)](#) | Automatización de claves SSH, ProxyCommand para servidores remotos, túneles seguros con Cloudflare Zero Trust y gestión de puertos. |
| **Entorno de Terminal** | [![Zsh](https://img.shields.io/badge/Zsh-F15A24?logo=zsh&logoColor=white)](#) [![Zellij](https://img.shields.io/badge/Zellij-000000?logo=gnu-bash&logoColor=white)](#) [![Micro](https://img.shields.io/badge/Micro-4A154B?logo=visualstudiocode&logoColor=white)](#) | ZSH con Oh My Zsh preconfigurado, multiplexor Zellij, editores `micro`/`nano` y manual integrado (`cat ~/help`). |

---

## Cómo se conecta tu equipo al contenedor (Red y SSH)

Cada proyecto corre en una subred privada de Docker (ej. `172.25.0.0/28`) **sin publicar puertos al host por defecto**. El contenedor principal (`<workspace>-devcontainer-ssh`) ejecuta OpenSSH en su **puerto 22 interno**, y puedes conectarte por IP privada o salto SSH según dónde corra Docker. Para abrir apps web o DBs desde el host bajo demanda, usa `devcontainer-cli access forward <puerto>` (túnel a `127.0.0.1`) o `devcontainer-cli access route add <puerto>` (`http://<workspace>-<puerto>.devcli.localhost`).

<details>
<summary><b>1. En tu misma máquina — <code>devcontainer-cli access ssh</code> (conexión directa por IP privada)</b></summary>

Úsalo cuando Docker corre en tu propio equipo. La CLI obtiene la IP privada del contenedor, instala tu clave y registra un alias en `~/.ssh/devcontainer-cli.config` apuntando a `<IP-privada>:22` (sin exponer puertos al exterior).

```bash
devcontainer-cli access ssh
```

```mermaid
graph LR
    subgraph Host["Tu Computadora (Host)"]
        VSCode["VS Code / Terminal"]
        
        subgraph DockerEnv["Subred Privada Docker (ej. 172.25.0.0/28)"]
            DevContainer["<workspace>-devcontainer-ssh (:22)"]
            DBs[("Bases de Datos")]
        end
    end
    
    VSCode -- "SSH Directo (<IP-privada>:22)" --> DevContainer
    DevContainer <--> DBs
```
</details>

<details>
<summary><b>2. Desde tu laptop hacia un servidor en tu red/LAN — <code>devcontainer-cli access ssh --via usuario@servidor</code></b></summary>

Úsalo cuando los contenedores corren en otro equipo (Mini PC, Home Server, Raspberry Pi). Tu laptop usa el servidor como puente SSH (`ProxyCommand` con `nc`) hasta `<IP-privada>:22` del contenedor, sin abrir puertos en el servidor. (Si el cliente no tiene acceso al daemon Docker o prefieres copiar un bloque manual, usa `--setup-external usuario@servidor` en el servidor).

```bash
devcontainer-cli access ssh --via usuario@servidor --container <workspace>-devcontainer-ssh
```

```mermaid
graph LR
    subgraph Laptop["Tu Laptop (Cliente)"]
        VSCode["VS Code / Terminal"]
    end
    
    subgraph Server["Servidor Docker (Host Remoto)"]
        SSHD["SSHD del Servidor"]
        
        subgraph DockerEnv["Subred Privada Docker"]
            DevContainer["<workspace>-devcontainer-ssh (:22)"]
            DBs[("Bases de Datos")]
        end
    end
    
    VSCode -- "SSH (ProxyCommand)" --> SSHD
    SSHD -- "nc <IP-privada> 22" --> DevContainer
    DevContainer <--> DBs
```
</details>

<details>
<summary><b>3. Desde cualquier red externa — módulo <code>cloudflared</code> + cliente WARP</b></summary>

Úsalo para conectarte desde fuera de tu red sin abrir puertos en el router. El módulo `cloudflared` se instala dentro de `<workspace>-devcontainer-ssh` y publica la subred Docker (`DOCKER_SUBNET`) hacia Cloudflare Zero Trust; luego te conectas desde cualquier lugar con el cliente WARP activo (`ssh devuser@<DEVCONTAINER_IP>`). También admite túneles rápidos efímeros (`cloudflared tunnel --url http://localhost:3000`). Ver [.docs/cloudflare-tunnel.md](.docs/cloudflare-tunnel.md).

```mermaid
graph LR
    subgraph Remote["Remoto (Fuera de tu red)"]
        Laptop["Laptop + WARP Client"]
    end
    
    subgraph Internet["Cloudflare Edge"]
        ZeroTrust["Zero Trust Private Network"]
    end
    
    subgraph LocalNetwork["Tu Red Local (Docker)"]
        subgraph DockerEnv["Subred Privada (ej. 172.25.0.0/28)"]
            DevContainer["<workspace>-devcontainer-ssh (sshd :22 + cloudflared)"]
            DBs[("Bases de Datos")]
        end
    end
    
    Laptop -- "Tráfico WARP cifrado" --> ZeroTrust
    ZeroTrust -- "Túnel saliente" --> DevContainer
    DevContainer -- "Ruteo IP Privada" --> DBs
```
</details>

---

## Comandos frecuentes

| Comando | Para qué sirve |
|---|---|
| `devcontainer-cli` | Abre el asistente para configurar el proyecto actual |
| `devcontainer-cli workspace up` | Construye (si hace falta) y levanta los contenedores |
| `devcontainer-cli access ssh` | Configura `~/.ssh/devcontainer-cli.config` y conecta por SSH (ideal para VS Code) |
| `devcontainer-cli access shell` | Abre una terminal Zsh directa dentro del contenedor |
| `devcontainer-cli access forward 3000` | Reenvía un puerto del contenedor a `127.0.0.1:3000` en tu máquina (`reverse:5432` en sentido inverso) |
| `devcontainer-cli access copy <orig> <dest>` | Copia archivos entre el host y el contenedor (`:` marca el lado del contenedor) |
| `devcontainer-cli inspect context` | Muestra herramientas instaladas, versiones y `~/CONTEXT.md` del contenedor |
| `devcontainer-cli workspace destroy` | Elimina contenedores, red y volúmenes del proyecto actual |

> ¿Necesitas dejar un proceso interactivo o TUI (como `claude --remote-control`) corriendo en segundo plano al salir de la shell? Consulta [UTILITIES.md](UTILITIES.md).

---

## Mantenimiento y solución rápida

<details>
<summary><b>Actualizar, desinstalar y problemas frecuentes</b></summary>

```bash
# Actualizar la CLI
devcontainer-cli config upgrade-cli

# Desinstalar la CLI
curl -fsSL https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/uninstall.sh | sh

# Limpiar entradas SSH antiguas o recursos Docker sin uso
devcontainer-cli config clean ssh -y
devcontainer-cli config clean all
```

- **Permission denied (publickey) / host key changed**: ejecuta `devcontainer-cli access ssh` para reparar claves o actualizar la IP registrada.
- **Credenciales compartidas entre contenedores**: usa `devcontainer-cli config shared sync` para persistir sesiones (`.claude`, `.codex`, `.gemini`, `.config/gh`, `.config/herdr`) en el volumen compartido.
- **Error de Docker socket (`dod`)**: verifica que el módulo `dod` esté activo y que `devuser` pertenezca al grupo `docker`.

</details>

---

## Guías de Uso

- [**`UTILITIES.md`**](UTILITIES.md) — Sesiones persistentes en segundo plano con Zellij (PTY) y procesos desacoplados.
- [**`MANUAL.md`**](MANUAL.md) — Casos de uso paso a paso (VS Code, servidores LAN con `--via`, bases de datos, agentes).
- [**`.docs/cli-reference.md`**](.docs/cli-reference.md) — Referencia completa de comandos y flags.
- [**`.docs/post-install.md`**](.docs/post-install.md) — Operaciones post-instalación y administración de bases de datos.
- [**`.docs/cloudflare-tunnel.md`**](.docs/cloudflare-tunnel.md) — Acceso remoto privado con Cloudflare Zero Trust y WARP.

> *¿Contribuir al desarrollo de la CLI?* La arquitectura, convenciones y flujo interno están en [`AGENTS.md`](AGENTS.md) y [`.docs/`](.docs/).
