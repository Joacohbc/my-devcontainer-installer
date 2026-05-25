# Referencia de la CLI — `devcontainer-cli`

`devcontainer-cli` genera y administra entornos de desarrollo dockerizados sin clonar este repositorio. Esta guía documenta todos sus subcomandos, flags y modos de build.

> Para instalar la CLI, los requisitos previos y la explicación general del entorno, ver [README.md](README.md).

## Generar el entorno

La CLI genera el `Dockerfile`, `docker-compose.yml`, `.env` y archivos auxiliares según los módulos que selecciones (Node, Java, Mongo, Postgres, Redis, Cloudflare Tunnel, etc.).

```bash
mkdir mi-proyecto && cd mi-proyecto
devcontainer-cli
```

Modo no-interactivo:

```bash
devcontainer-cli --with nodejs,java --service mongo,postgres --image mi-dev:local --no-build
```

Ayuda completa: `devcontainer-cli --help`.

Tras generar, inicia el entorno:

```bash
docker compose -f .dc_<workspace>/build/docker-compose.yml up -d
```

> Para personalizar la red (subred, túnel Cloudflare), edita `.dc_<workspace>/build/.env` antes de iniciar (ver [CLOUDFLARE_TUNNEL.md](CLOUDFLARE_TUNNEL.md)).

### Estructura generada

La CLI genera todo dentro de `.dc_<workspace>/` (donde `<workspace>` es el nombre del directorio actual, o el valor de `--workspace`):

```
<tu-proyecto>/
├── devcontainer.config.json          # Estado persistido (módulos, servicios, subnet)
└── .dc_<workspace>/
    └── build/                        # Contexto de `docker build`
        ├── Dockerfile
        ├── docker-compose.yml
        ├── .env                      # DOCKER_SUBNET, DEVCONTAINER_IP, TUNNEL_TOKEN…
        ├── entrypoint.sh
        ├── zsh-installer.sh
        ├── golang_utils.sh           # solo con módulo `go`
        ├── install-*.sh              # solo con módulo `ai-clis`
        ├── login-github-cli.sh
        └── update_golang.sh          # solo con módulo `go`
```

Los scripts post-instalación se hornean dentro de la imagen en `~/post-script/` (`/home/devuser/post-script/`), disponibles en cualquier contenedor sin montar archivos del host. Tu código vive en `<tu-proyecto>/` (la raíz), que se monta como `/workspace` dentro del contenedor.

## Subcomandos disponibles

| Subcomando                       | Qué hace                                                                                                    |
|----------------------------------|-------------------------------------------------------------------------------------------------------------|
| `devcontainer-cli`               | Genera/actualiza `Dockerfile`, `docker-compose.yml`, `.env` y scripts auxiliares (flujo interactivo o no). |
| `devcontainer-cli setup-ssh`     | Configura el acceso SSH autodetectando el compose del proyecto. Ver [Configurar acceso SSH](#configurar-acceso-ssh-setup-ssh). |
| `devcontainer-cli port-forward`  | Reenvía un puerto local al contenedor (o a un servicio interno) vía túnel SSH. Ver [Port Forwarding](#port-forwarding). |
| `devcontainer-cli run`           | Levanta un container desde una imagen remota sin crear archivos de proyecto. Ver [Contenedor rápido](#contenedor-rápido-run). |
| `devcontainer-cli start`         | `docker compose start` para el proyecto en el directorio actual.                                           |
| `devcontainer-cli stop`          | `docker compose stop` para el proyecto en el directorio actual.                                            |
| `devcontainer-cli restart`       | `docker compose restart` para el proyecto en el directorio actual.                                         |
| `devcontainer-cli down`          | `docker compose down` para el proyecto. Agrega `--volumes` para borrar también los volúmenes.              |
| `devcontainer-cli destroy`       | `down -v` + borra `.dc_<workspace>/` y `devcontainer.config.json`. Irreversible.                           |
| `devcontainer-cli prune`         | Elimina imágenes `devcontainer-cli/*` huérfanas (proyecto borrado). `--all` elimina todas.                 |
| `devcontainer-cli update`        | Actualiza la imagen del proyecto actual (pull o rebuild según el modo). `--all` recorre todos los proyectos.|
| `devcontainer-cli upgrade-cli`   | Reemplaza el binario de la CLI con la última release de GitHub.                                             |
| `devcontainer-cli config`        | Lee/escribe configuración global (ej. `config registry ghcr.io/mi-org/`).                                   |
| `devcontainer-cli cleanup-tips`  | Imprime los comandos `docker` para borrar contenedores, redes y volúmenes del proyecto.                     |
| `devcontainer-cli completion`    | Imprime el script de autocompletado para bash o zsh (`completion bash` / `completion zsh`).                 |

## Modos de Build

La CLI ofrece dos modos para controlar cómo se construye la imagen del devcontainer. Se elige con `--mode` o por prompt interactivo.

| Modo | Genera | Imagen | Para qué sirve |
|---|---|---|---|
| `local-cached` (default) | `Dockerfile` + `docker-compose.yml` | `devcontainer-cli/<fp12>:latest` | Build local. Deduplica por fingerprint del Dockerfile: si la imagen ya está en el daemon, no rebuildea. |
| `remote` | `docker-compose.yml` (sin Dockerfile) | `ghcr.io/<owner>/devcontainer-<variant>:latest` | Salta el build. Usa las imágenes pre-buildeadas por GitHub Actions. Mucho más rápido. |

**Variants disponibles** (para `remote`):

| Variant | Contenido |
|---|---|
| `ssh` | Imagen completa (todos los módulos) |
| `nodejs` | Node.js |
| `bun` | Bun |
| `java-temurin` | Java Temurin |
| `python` | Python |
| `go` | Go |
| `node-go` | Node.js + Go |
| `node-python` | Node.js + Python |
| `node-java-temurin` | Node.js + Java Temurin |
| `bun-go` | Bun + Go |
| `bun-python` | Bun + Python |
| `bun-java-temurin` | Bun + Java Temurin |

### Ejemplos

```bash
# Local-cached (default) — build local con dedup por fingerprint
devcontainer-cli --with nodejs,java-temurin --service mongo

# Remote — pull de imagen pre-buildeada, sin Dockerfile
devcontainer-cli --mode remote --variant node-java-temurin --service postgres
```

## Actualizar imágenes (`update`)

```bash
# Actualizar la imagen del proyecto actual (pull para remote, build --pull para local-cached)
devcontainer-cli update

# Actualizar TODAS las imágenes de los proyectos que conoce la CLI
devcontainer-cli update --all

# Forzar rebuild aunque el fingerprint coincida
devcontainer-cli update --rebuild
```

> **Nota:** `update` opera sobre imágenes de proyecto. Para actualizar el binario de la CLI usá `devcontainer-cli upgrade-cli`.

## Contenedor rápido (`run`)

Levanta un container desde una imagen remota **sin crear ningún archivo** en el proyecto (sin `.dc_*/`, sin `devcontainer.config.json`). Ideal para tareas puntuales o exploración rápida.

```bash
# Interactivo — elige la variante por prompt
devcontainer-cli run

# Directo — especifica variante
devcontainer-cli run --variant python

# Con volumen persistente en /workspace
devcontainer-cli run --variant nodejs --volume mi_workspace

# Con puerto SSH expuesto al host
devcontainer-cli run --variant ssh --port 2222

# Todo junto
devcontainer-cli run --variant node-go --name mi-dev --volume mi_vol --port 2222
```

El comando es **idempotente**: si el container ya existe (exited o paused) lo reinicia; si ya está corriendo no hace nada.

Al finalizar imprime el comando para continuar con SSH:

```bash
devcontainer-cli setup-ssh --container dc-<variant>
```

**Flags disponibles:**

| Flag | Descripción |
|---|---|
| `--variant <name>` | Variante de imagen remota |
| `--name <name>` | Nombre del container (default: `dc-<variant>`) |
| `--volume <name>` | Named volume montado en `/workspace` (opcional) |
| `--port <n>` | Expone el puerto 22 del container en el host |
| `--registry <url>` | Override del registry (default: `ghcr.io/joacohbc/`) |

## Port Forwarding

Redirige un puerto local de tu máquina al contenedor (o a un servicio interno de la red Docker) usando un túnel SSH. Requiere que `setup-ssh` esté configurado.

### Modo interactivo (varios containers y puertos)

Ejecuta el comando sin argumentos para abrir el selector interactivo:

```bash
devcontainer-cli port-forward
```

1. Elegís un container de la lista de **todos** los containers en ejecución (no solo devcontainers). Los devcontainers aparecen marcados con `(devcontainer)`.
2. Ingresás uno o más puertos a reenviar de ese container, separados por coma (ej. `3000, 8080:80`).
3. Te pregunta si querés agregar otro container y repetís el proceso.
4. Cuando confirmás, se muestra un resumen del plan y se abren **todos** los túneles en paralelo. `Ctrl+C` los cierra a la vez.

Los devcontainers con alias SSH propio se alcanzan directamente. Los containers que **no** son devcontainers (postgres, redis, etc.) se reenvían a través de un devcontainer que actúe como _jump host_ SSH; si hay varios, la CLI te pregunta cuál usar.

### Modo directo (un solo mapeo)

```bash
# Forward del puerto 3000 (local) → 3000 (contenedor)
devcontainer-cli port-forward 3000

# Puerto local distinto al remoto
devcontainer-cli port-forward 8080:80

# Acceder a un servicio interno (ej. la base de datos postgres del stack)
devcontainer-cli port-forward 5432:postgres:5432

# Usando flag --service
devcontainer-cli port-forward 5432 --service postgres

# Forzar un alias SSH específico
devcontainer-cli port-forward 3000 --alias mi-devcontainer
```

El proceso queda en foreground. `Ctrl+C` cierra el túnel.

**Flags disponibles:**

| Flag | Descripción |
|---|---|
| `<local>:<host>:<remote>` | Mapeo completo: puerto local, host destino, puerto remoto |
| `<local>:<remote>` | Puerto local distinto al remoto (host = `localhost`) |
| `<port>` | Puerto idéntico en ambos extremos |
| `--service <name>` | Servicio compose destino del túnel |
| `--alias <name>` | Alias SSH a usar (sin autodetección) |
| `--no-interactive` | Falla si falta algún parámetro |

## Configurar acceso SSH (`setup-ssh`)

La CLI puede generar la clave, copiarla al contenedor y dejar listo el bloque `~/.ssh/config` por ti. Detecta automáticamente el `container_name` desde `docker-compose.yml` (incluyendo el prefijo del workspace) y elige el modo (`local`, `windows` o `remote`) según el contexto.

Desde la carpeta del proyecto (donde está el `docker-compose.yml`):

```bash
# Local / Windows (autodetecta): genera clave si falta, copia al contenedor, escribe ~/.ssh/config
devcontainer-cli setup-ssh

# Forzar un modo concreto
devcontainer-cli setup-ssh --mode local
devcontainer-cli setup-ssh --mode windows --port 2222

# Acceso remoto vía servidor con ProxyCommand
devcontainer-cli setup-ssh --remote usuario@servidor

# No interactivo (CI o scripts)
devcontainer-cli setup-ssh --yes --alias devcontainer --key ~/.ssh/id_devcontainer
```

Ayuda completa: `devcontainer-cli setup-ssh --help`.

Tras esto puedes conectar directamente con `ssh devcontainer` (o el alias que hayas pasado).

> Si preferís configurar SSH a mano (sin la CLI), el paso a paso manual está en [README.md → Acceso y Uso](README.md#acceso-y-uso).

## Limpieza de recursos

### Bajar el proyecto (`down`)

Corre `docker compose down -v` para el proyecto en el directorio actual:

```bash
devcontainer-cli down

# Sin confirmación
devcontainer-cli down --yes
```

Elimina los contenedores y volúmenes del stack. Requiere que exista `.dc_<workspace>/build/docker-compose.yml`.

### Limpiar imágenes huérfanas (`prune`)

Elimina las imágenes `devcontainer-cli/*` cuyo proyecto ya no existe en disco:

```bash
# Solo huérfanas (proyecto borrado o movido)
devcontainer-cli prune

# Todas las imágenes devcontainer-cli/* sin excepción
devcontainer-cli prune --all

# Sin confirmación
devcontainer-cli prune --yes
```

### Configurar registry global

```bash
# Ver el registry actual
devcontainer-cli config registry

# Cambiarlo (por ejemplo si forkeaste y publicaste en tu propio org)
devcontainer-cli config registry ghcr.io/mi-org/

# Restaurar el default
devcontainer-cli config registry --unset
```

El registry también se puede override por invocación con `--registry ghcr.io/foo/`.
