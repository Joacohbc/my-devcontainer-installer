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

2. **Iniciar el entorno:**

    > **Nota para Cloudflare Tunnel:** Si deseas habilitar el acceso remoto seguro, crea el archivo `.env` siguiendo las instrucciones de [CLOUDFLARE_TUNNEL.md](CLOUDFLARE_TUNNEL.md) **antes** de ejecutar el siguiente comando. Y ejecutar: `docker compose --env-file .env up -d --build`

    ```bash
    docker compose up -d --build
    ```

## Acceso y Uso

### 1. Obtener Credenciales

Al iniciar por primera vez, se generan contraseñas aleatorias para `root` y `devuser`. Consúltalas en los logs:

```bash
docker compose logs devcontainer-ssh | tail -n 3
# Busca líneas como: "devuser password: <password>"
```

### 2. Conexión SSH

Para facilitar la conexión cómoda (sin contraseña) y compatible con **VS Code Remote - SSH**, se recomienda configurar el acceso mediante claves SSH.

#### Pasos Comunes (Obligatorio)

Realiza estos pasos preliminares antes de configurar tu conexión específica.

1. **Verificar credenciales (para tener la contraseña a mano):**

    ```bash
    docker logs devcontainer-ssh | tail -n 3
    ```

    Este paso siempre debe hacerse **en el dispositivo host** (donde corre Docker). Deberás usar la contraseña temporal para la primera conexión SSH (sea para el
    acceso local o remoto).

2. **Generar claves SSH:**

    ```bash
    ssh-keygen -t ed25519 -f ~/.ssh/id_devcontainer -N "" -q
    ```

    > _**Nota:** Este paso debe realizarse en el **dispositivo desde el que te conectarás** (puede ser el host local o tu laptop)._

#### Tipos de Configuración

Existen dos escenarios de conexión principales. Elige el tuyo:

**A. Acceso Local (Tu PC es el Host)**
Este método se usa cuando Docker corre en la misma máquina desde la que trabajas.

1. **Copiar Clave:**

    ```bash
    IP_SSH=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' devcontainer-ssh)
    ssh-copy-id -i ~/.ssh/id_devcontainer.pub devuser@$IP_SSH
    ```

2. **Configurar Alias de Conexión:** Agrega la configuración al archivo `~/.ssh/config` ejecutando:

    ```bash
    cat <<EOF >> ~/.ssh/config
    Host devcontainer-ssh
        HostName $IP_SSH
        IdentityFile ~/.ssh/id_devcontainer
        User devuser
    EOF
    ```

**B. Acceso Remoto (Desde otra PC a través de la red)**
Si el contenedor corre en un servidor (ej. una Raspberry Pi, Mini PC, etc) y te conectas desde tu laptop.

> **Importante:** En los siguientes comandos, sustituye `usuario@host` por la conexión real a tu servidor (puede ser `tu_usuario@ip` o un alias de SSH si ya lo tienes configurado).

1. **Instalar Clave Remotamente (desde tu laptop):**

    ```bash
    cat ~/.ssh/id_devcontainer.pub | ssh usuario@host "docker exec -i -u devuser devcontainer-ssh sh -c 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys'"
    ```

2. **ProxyCommand:** Agrega la configuración al archivo `~/.ssh/config` ejecutando:

    ```bash
    cat <<'EOF' >> ~/.ssh/config
    Host dev-rbpi
        User devuser
        IdentityFile ~/.ssh/id_devcontainer
        ProxyCommand ssh usuario@host "nc -q0 \$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' devcontainer-ssh) 22"
    EOF
    ```

#### ¿Cómo funciona? (Detalle Técnico)

1. **Aislamiento de Claves:** Se utiliza una clave específica (`id_devcontainer`) en lugar de tu clave personal predeterminada para mayor seguridad y segmentación.
2. **Resolución de IP:** Los contenedores tienen IPs dinámicas o privadas.
    * En **local**, se busca la IP con `docker inspect` y se configura estáticamente (o se actualiza).
    * En **remoto**, la IP del contenedor no es alcanzable directamente desde tu laptop. El `ProxyCommand` establece una conexión SSH al servidor físico, y desde allí abre un túnel directo al puerto 22 del contenedor usando su IP interna. Esto permite conectar VS Code o tu terminal como si el contenedor estuviera en tu red local.

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

Para configurar el acceso remoto seguro mediante Cloudflare Tunnel (Red Privada), consulta la guía: [CLOUDFLARE_TUNNEL.md](CLOUDFLARE_TUNNEL.md).

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

> _**Nota:** PostgreSQL solicitará la contraseña `devpass` interactivamente._

**Credenciales:**

* **Usuario:** `devuser`
* **Contraseña:** `devpass`
* **Base de datos:** `devdb` (PostgreSQL)
* **Root/Admin:** `devuser:devpass` (MongoDB)
