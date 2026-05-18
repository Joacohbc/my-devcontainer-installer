# my-devcontainer-installer

Este repositorio proporciona un entorno de desarrollo completo y remoto basado en Docker. Está diseñado para simplificar la configuración de proyectos que requieren múltiples tecnologías (bases de datos, lenguajes, herramientas) y una conectividad de red avanzada.

El entorno ofrece un contenedor principal (`devcontainer-ssh`) accesible vía SSH, con acceso directo a bases de datos y herramientas de desarrollo preinstaladas, simulando una máquina virtual ligera pero con la flexibilidad de Docker.

## Arquitectura

El siguiente diagrama ilustra cómo interactúan los componentes del entorno:

```mermaid
graph TD
    subgraph Host["Tu Computadora (Host)"]
        VSCode["VS Code / Terminal"]
        SSHKey["Clave SSH Privada"]
        DockerSock["/var/run/docker.sock"]
    end

    subgraph DockerEnv["Entorno Docker"]
        
        subgraph Services["Red Privada (172.25.0.0/24)"]
            direction TB
            DevContainer["🖥️ Devcontainer-SSH (Ubuntu, Go, Node, Python)"]
            Postgres[("🐘 PostgreSQL")]
            Mongo[("🍃 MongoDB")]
            Redis[("🔴 Redis")]
        end

    end

    %% Conexiones
    VSCode -- "SSH" --> DevContainer
    SSHKey -. "Autenticación" .-> DevContainer
    
    DevContainer -- "Acceso Interno" --> Postgres
    DevContainer -- "Acceso Interno" --> Mongo
    DevContainer -- "Acceso Interno" --> Redis
    
    DockerSock -. "Montado (Bind Mount)" .-> DevContainer
```

### 1. Acceso Local (Tu PC es el Host)

**Recomendado para**: Trabajar directamente en la máquina donde corre Docker.

Se establece una conexión SSH directa desde tu terminal o VS Code hacia la IP privada del contenedor, aprovechando que ambos están en el mismo equipo. Sin exponer puertos.

```mermaid
graph LR
    subgraph Host["Tu Computadora (Host)"]
        VSCode["VS Code / Terminal"]
        
        subgraph DockerEnv["Entorno Docker"]
            DevContainer["🖥️ Devcontainer-SSH"]
            DBs[("🗄️ Bases de Datos")]
        end
    end
    
    VSCode -- "SSH Directo (IP Privada)" --> DevContainer
    DevContainer <--> DBs
```

### 2. Acceso Remoto LAN (Laptop -> Servidor)

**Recomendado para**: Conectar tu laptop a un servidor doméstico (ej. Raspberry Pi, Mini PC) dentro de tu red.

Utiliza el servicio SSH del servidor anfitrión como puente seguro. El tráfico se redirige internamente hacia el contenedor (usando netcat), lo que evita tener que exponer puertos del contenedor a toda la red local.

```mermaid
graph LR
    subgraph Laptop["Tu Laptop (Cliente)"]
        VSCode["VS Code"]
    end
    
    subgraph Server["Servidor (Host)"]
        SSHD["SSHD (Host)"]
        
        subgraph DockerEnv["Entorno Docker"]
            DevContainer["🖥️ Devcontainer-SSH"]
        end
    end
    
    VSCode -- "SSH (ProxyCommand)" --> SSHD
    SSHD -- "netcat (nc): Reenvía tráfico TCP" --> DevContainer
```

### 3. Acceso Remoto Seguro (Cloudflare Tunnel)

**Recomendado para**: Acceder desde cualquier lugar (cafeterías, viajes) sin abrir puertos en el router.

Mediante Cloudflare Zero Trust, se crea un túnel cifrado de salida. Esto permite que tu dispositivo remoto (autenticado con WARP) acceda a la red privada del contenedor de forma segura, como si estuvieras conectado localmente.

```mermaid
graph LR
    subgraph Remote["Remoto (Cafetería/Casa)"]
        Laptop["Laptop + WARP Client"]
    end
    
    subgraph Internet["Cloudflare Edge"]
        ZeroTrust["Zero Trust Network"]
    end
    
    subgraph LocalNetwork["Tu Red Local"]
        
        subgraph DockerEnv["Entorno Docker"]
            Cloudflared["Cloudflared (Túnel)"]
            DevContainer["🖥️ Devcontainer-SSH"]
            DBs[("🗄️ Bases de Datos")]
        end
    end
    
    Laptop -- "Túnel Seguro" --> ZeroTrust
    ZeroTrust -- "Túnel Encriptado" --> Cloudflared
    Cloudflared -- "Ruteo IP Privada" --> DevContainer
```

## Características Principales

* **Sistema Base:** Ubuntu 22.04 LTS.
* **Conexión SSH:** Acceso seguro mediante OpenSSH Server. Ideal para usar con VS Code Remote - SSH o tu terminal favorita.
* **Persistencia y Sincronización:**
  * El directorio del repositorio se monta en `/workspace` dentro del contenedor.
  * Los archivos de configuración y datos de usuario (`/home`, `/root`, `/etc`) se persisten en volúmenes Docker.
* **Bases de Datos (Dockerizadas):**
  * MongoDB 8.0
  * Redis 7.4 (Alpine)
  * PostgreSQL 17 (Alpine)
* **Lenguajes y Herramientas Preinstalados:**
  * **Java:** JDK Temurin 11 y 17 + Maven.
  * **Python:** Python 3 + pip.
  * **Node.js:** NVM (Node Version Manager) preinstalado para gestionar versiones.
  * **Bun.js:** Runtime moderno para JavaScript y TypeScript (instalado oficialmente).
  * **Go:** Última versión (instalada vía script utilitario).
  * **SQLite:** sqlite3.
  * **Herramientas CLI:** `git`, `gh` (GitHub CLI), `docker-ce-cli` (Docker outside Docker), `nano`, `wget`, `jq`.
* **Terminal Mejorada:** ZSH preconfigurado con frameworks y plugins útiles.

## Requisitos Previos

* Docker y Docker Compose.
* Cliente SSH (OpenSSH) instalado en tu máquina local.
* Conocimiento básico de terminal.

### Nota Importante para Windows

Si estás utilizando **Windows**, se recomienda encarecidamente usar **Git Bash** como terminal. Esto garantiza la compatibilidad con los comandos de Linux (`grep`, `tail`, `ssh-copy-id`, etc.) y facilita la configuración.

1.  **Formato de Archivos (LF vs CRLF):** Los scripts y archivos de configuración deben tener terminaciones de línea estilo UNIX (`LF`). Se ha incluido un archivo `.gitattributes` para manejar esto automáticamente.
2.  **Firewall:** Para conectarte al contenedor, es necesario que el firewall de Windows permita las conexiones al puerto expuesto (por defecto `2222`).

## Instalación y Configuración

La CLI genera el `Dockerfile`, `docker-compose.yml`, `.env` y archivos auxiliares según los módulos que selecciones (Node, Java, Mongo, Postgres, Redis, Cloudflare Tunnel, etc.) — sin clonar el repo.

### 1. Instalar la CLI (one-liner)

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/install.sh | sh
```

Detecta OS/arch automáticamente. Instala binario en `~/.local/bin/devcontainer-cli` y assets en `~/.local/share/devcontainer-cli/assets`.

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/install.ps1 | iex
```

Instala en `%LOCALAPPDATA%\devcontainer-cli` y agrega al PATH del usuario.

**Variables opcionales:**

| Var          | Default                                  | Descripción                       |
|--------------|------------------------------------------|-----------------------------------|
| `VERSION`    | `latest`                                 | Tag específico (ej. `v1.0.0`)     |
| `INSTALL_DIR`| `~/.local/share/devcontainer-cli` (unix) | Carpeta binario + assets          |
| `BIN_DIR`    | `~/.local/bin` (unix)                    | Symlink al binario                |

Ejemplo versión fija:

```bash
curl -fsSL https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/install.sh | VERSION=v1.0.0 sh
```

**Descarga manual** (alternativa): https://github.com/Joacohbc/my-devcontainer-installer/releases/latest — assets disponibles: `linux-x64`, `linux-arm64`, `darwin-x64` (tar.gz) y `windows-x64` (zip).

> El binario lee `assets/` junto al ejecutable. No separes ambos.

### 2. Generar el entorno

```bash
mkdir mi-proyecto && cd mi-proyecto
devcontainer-cli
```

Modo no-interactivo:

```bash
devcontainer-cli --with nodejs,java --service mongo,postgres --image mi-dev:local --no-build
```

Ayuda completa: `devcontainer-cli --help`.

La CLI genera `Dockerfile` + `docker-compose.yml` + scripts auxiliares en el directorio actual.

### 3. Iniciar el entorno

**Linux/Mac:**
```bash
docker compose up -d
```

**Windows (Git Bash):**
Para exponer el puerto SSH localmente en Windows, se genera además `docker-compose.windows.yml`:
```bash
docker compose -f docker-compose.yml -f docker-compose.windows.yml up -d
```

Si deseas personalizar la configuración de red (opcional), edita el `.env` generado antes de iniciar (ver [CLOUDFLARE_TUNNEL.md](CLOUDFLARE_TUNNEL.md)).

## Acceso y Uso

La forma recomendada y más segura de acceder es mediante **Claves SSH**. El uso de contraseñas debería limitarse únicamente a la configuración inicial.

### 1. Preparación (Host)

Primero, obtén la contraseña temporal generada durante la instalación. La necesitarás **solo una vez** para instalar tu clave SSH.

```bash
docker compose logs devcontainer-ssh | grep "devuser password" | tail -n 1
```

### 2. Configurar Acceso SSH (Recomendado)

Sigue estos pasos desde tu máquina local (tu PC o Laptop) para autorizar tu acceso sin contraseña.

#### A. Generar par de claves (Si no tienes una)

Se recomienda usar una clave específica para este entorno:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_devcontainer -N "" -q
```

#### B. Instalar la clave en el contenedor

**Opción 1: Entorno Local (Linux/Mac)**
(Si Docker corre en la misma máquina que estás usando)

```bash
# 1. Obtener IP del contenedor
IP_SSH=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' devcontainer-ssh)

# 2. Copiar clave (te pedirá la contraseña obtenida en el paso 1)
ssh-copy-id -i ~/.ssh/id_devcontainer.pub devuser@$IP_SSH

# 3. Guardar configuración en tu SSH config
cat <<EOF >> ~/.ssh/config
Host devcontainer
    HostName $IP_SSH
    IdentityFile ~/.ssh/id_devcontainer
    User devuser
EOF
```

**Opción 2: Entorno Local (Windows con Git Bash)**
(Usando el puerto expuesto 2222)

```bash
# 1. Copiar clave pública al contenedor (te pedirá la contraseña)
# Nota: Git Bash suele incluir ssh-copy-id, si no, usa el comando manual:
ssh-copy-id -p 2222 -i ~/.ssh/id_devcontainer.pub devuser@localhost

# Alternativa manual si ssh-copy-id no está disponible:
# cat ~/.ssh/id_devcontainer.pub | ssh -p 2222 devuser@localhost "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys"

# 2. Añadir a config (Ejecuta esto en Git Bash)
cat <<EOF >> ~/.ssh/config
Host devcontainer
    HostName localhost
    Port 2222
    User devuser
    IdentityFile ~/.ssh/id_devcontainer
EOF
```

**Opción 3: Entorno Remoto**
(Si Docker corre en un servidor y tú estás en tu laptop)

Sustituye `usuario@servidor` por los datos de conexión a tu servidor físico.

```bash
cat ~/.ssh/id_devcontainer.pub | ssh usuario@servidor "docker exec -i -u devuser devcontainer-ssh sh -c 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys'"
```

Para conectar fácilmente, añade esto a tu `~/.ssh/config`:

```ssh
Host devcontainer-remote
    User devuser
    IdentityFile ~/.ssh/id_devcontainer
    ProxyCommand ssh usuario@servidor "nc -q0 \$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' devcontainer-ssh) 22"
```

### 3. Conectarse

Ahora puedes conectar simplemente con el alias que hayas configurado:

```bash
ssh devcontainer
# O
ssh devcontainer-remote
```

## Pasos Post-Instalación

Una vez dentro, puedes ejecutar los scripts de configuración incluidos para terminar de preparar tu entorno:

* **GitHub CLI:** `/workspace/login-github-cli.sh` - Te ayuda a iniciar sesión y configurar tus credenciales de GitHub.
* **Actualizar Go:** `/workspace/update_golang.sh` - Actualiza la instalación de Go a la última versión estable disponible.
* **Actualizar Sistema:** Es relevante mantener el entorno al día (incluyendo e.g. el cliente de Docker) ejecutando `sudo apt update && sudo apt upgrade -y`.

Consulta [POST_INSTALL_STEPS.md](POST_INSTALL_STEPS.md) para más detalles sobre backups y herramientas adicionales.

## Agregar Servicios Extra (Docker Run)

Para levantar un contenedor nuevo (por ejemplo, una base de datos extra o un servicio temporal) asegurando que el DevContainer pueda verlo y conectarse a él, es necesario que ambos compartan la misma red Docker.

Puedes usar el siguiente comando "universal", que detecta automáticamente el nombre completo de la red del proyecto y conecta el nuevo servicio:

```bash
docker run -d \
  --name <nombre-del-servicio> \
  --network $(docker network ls -q -f name=local-network) \
  <imagen>
```

**Explicación de las banderas:**

*   `--network $(...)`: Busca dinámicamente el ID de la red que contiene 'local-network' (útil porque Docker Compose suele agregar prefijos al nombre de la red).
*   `--name`: Asigna el hostname. Esto permite que, desde dentro del DevContainer, puedas hacer ping o conectarte usando este nombre (ej: `ping <nombre-del-servicio>`).

## Solución de Problemas (Troubleshooting)

### Error: "Permission denied (publickey)"

* Asegúrate de haber copiado tu clave pública (`.pub`) al contenedor correctamente.
* Verifica los permisos en el contenedor: la carpeta `~/.ssh` debe tener `700` y `authorized_keys` debe tener `600`.

### Error: "Connection refused"

* Verifica que el contenedor esté corriendo: `docker compose ps`.
* Si usas IP dinámica (Local), asegúrate de que la IP no haya cambiado. Si reiniciaste el contenedor, es posible que necesites actualizar la IP en tu `~/.ssh/config`.
* **Windows:** Verifica que el Firewall de Windows permita conexiones entrantes al puerto 2222.

### Problemas con Docker dentro del contenedor

* Si comandos como `docker ps` fallan dentro del contenedor, verifica que el socket esté montado correctamente en `docker-compose.yml`:
    `- /var/run/docker.sock:/var/run/docker.sock`
* Asegúrate de que el usuario `devuser` pertenezca al grupo `docker` (esto se hace automáticamente en el Dockerfile).

### Conflicto de Puertos

* Si Docker falla al iniciar porque un puerto (ej. 27017, 5432, 2222) ya está en uso, detén el servicio local que lo ocupa en tu máquina host o modifica el mapeo de puertos en `docker-compose.yml` o `docker-compose.windows.yml`.

## Estructura del Proyecto

Todo se genera mediante la CLI. Tras ejecutar `devcontainer-cli` en tu directorio de trabajo obtendrás:

* `Dockerfile`: Configuración de la imagen base.
* `docker-compose.yml` (+ `docker-compose.windows.yml` en Windows): Orquestación de servicios.
* `entrypoint.sh`, `zsh-installer.sh`, `golang_utils.sh`: Scripts de build/runtime.
* `login-github-cli.sh`, `update_golang.sh`: Scripts post-instalación (accesibles dentro del contenedor en `/workspace/`).

## Bases de Datos

Credenciales por defecto: **Usuario:** `devuser` / **Password:** `devpass`.

* **PostgreSQL:** `psql -h postgres -U devuser -d devdb`
* **MongoDB:** `mongosh --host mongo -u devuser -p devpass --authenticationDatabase admin`
* **Redis:** `redis-cli -h redis`
