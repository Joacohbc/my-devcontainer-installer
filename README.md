# my-devcontainer-installer

Este repositorio proporciona un entorno de desarrollo completo y remoto basado en Docker. Está diseñado para simplificar la configuración de proyectos que requieren múltiples tecnologías (bases de datos, lenguajes, herramientas) y una conectividad de red avanzada.

El entorno ofrece un contenedor principal (`devcontainer-ssh`) accesible vía SSH, con acceso directo a bases de datos y herramientas de desarrollo preinstaladas, simulando una máquina virtual ligera pero con la flexibilidad de Docker.

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
  * **Go:** Última versión (instalada vía script utilitario).
  * **SQLite:** sqlite3.
  * **Herramientas CLI:** `git`, `gh` (GitHub CLI), `docker-ce-cli` (Docker outside Docker), `nano`, `wget`, `jq`.
* **Red Avanzada (IP dedicada):** Opción de configurar una IP estática dentro de tu red local (modo `ipvlan`), permitiendo tratar al contenedor como un dispositivo físico más en tu red.
* **Terminal Mejorada:** ZSH preconfigurado con frameworks y plugins útiles.

## Requisitos Previos

* Docker y Docker Compose.
* Conocimiento básico de terminal y redes (si usas el modo de red avanzada).
* Acceso a tu router/red local para asignar una IP estática (opcional, para modo `ipvlan`).

## Instalación y Configuración

1. **Clonar el repositorio:**

    ```bash
    git clone https://github.com/Joacohbc/my-devcontainer-installer.git .dev-env && cd .dev-env
    ```

2. **Configurar variables de entorno (`.env`):**

    Crea un archivo `.env` basado en la configuración de tu red.

    ```ini
    # Configuración de Red (Modo ipvlan)
    DEVCONTAINER_SSH_IP=192.168.1.150  # IP libre en tu red local
    NETWORK_RANGE=192.168.1.0/24       # Rango de tu red (CIDR)
    GATEWAY_IP=192.168.1.1             # IP de tu Router
    HOST_INTERFACE=eth0                # Nombre de tu interfaz de red física (ej: eth0, wlan0)
    ```

    > **Nota:** Para saber el nombre de tu interfaz, usa el comando `ip a` o `ifconfig`.

3. **Iniciar el entorno:**

    ```bash
    docker compose --env-file .env up -d --build
    ```

## Acceso y Uso

### 1. Obtener Credenciales

Al iniciar por primera vez, se generan contraseñas aleatorias para `root` y `devuser`. Consúltalas en los logs:

```bash
docker compose logs devcontainer-ssh
# Busca líneas como: "devuser password: <password>"
```

### 2. Conexión SSH

* **Usuario:** `devuser` (recomendado) o `root`.
* **Host:** La IP configurada en `DEVCONTAINER_SSH_IP` (o `localhost` si modificaste `docker-compose.yml` para usar puertos).

```bash
ssh devuser@<DEVCONTAINER_SSH_IP>
```

### 3. Pasos Post-Instalación (Recomendados)

Una vez dentro del contenedor, puedes realizar configuraciones adicionales.

* **Configurar GitHub CLI:**
    Ejecuta el script incluido para loguearte y configurar git automáticamente:

    ```bash
    /workspace/login-github-cli.sh
    ```

* **Actualizar Go:**
    Para actualizar la versión de Go en el futuro:

    ```bash
    /workspace/update_golang.sh
    ```

Para más detalles sobre pasos posteriores (Firebase, Gemini CLI, etc.), consulta el archivo [POST_INSTALL_STEPS.md](POST_INSTALL_STEPS.md).

## Estructura del Proyecto

* `Dockerfile`: Configuración de la imagen base, instalación de paquetes y usuarios.
* `docker-compose.yml`: Orquestación de servicios (contenedor SSH + Bases de datos).
* `zsh-installer.sh`: Script de configuración de ZSH (invocado durante la construcción).
* `golang_utils.sh` / `update_golang.sh`: Utilidades para gestionar la instalación de Go.
* `login-github-cli.sh`: Script auxiliar para autenticación con GitHub.

## Docker-outside-Docker (DooD)

El contenedor tiene acceso al socket de Docker del host. Esto te permite ejecutar comandos como `docker ps`, `docker build`, etc., desde dentro del contenedor SSH, pero afectando a tu Docker local.

* Para deshabilitarlo, comenta el montaje de `/var/run/docker.sock` en `docker-compose.yml`.

## Bases de Datos

Las bases de datos están expuestas en los puertos estándar y accesibles desde el contenedor SSH. Puedes conectarte directamente usando los siguientes comandos:

### MongoDB (Puerto 27017)

```bash
mongosh --host mongo -u devuser -p devpass --authenticationDatabase admin
```

### Redis (Puerto 6379)

```bash
redis-cli -h redis
```

### PostgreSQL (Puerto 5432)

```bash
psql -h postgres -U devuser -d devdb
```

> **Nota:** PostgreSQL solicitará la contraseña `devpass` interactivamente.

**Credenciales:**
* **Usuario:** `devuser`
* **Contraseña:** `devpass`
* **Base de datos:** `devdb` (PostgreSQL)
* **Root/Admin:** `devuser:devpass` (MongoDB)
