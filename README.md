# my-devcontainer-installer

Este repositorio contiene la configuración para un entorno de desarrollo remoto usando Docker, ideal para proyectos que involucran múltiples bases de datos y requieren una configuración de red específica. El contenedor proporciona un entorno aislado y consistente, y la configuración de red permite el acceso directo a las bases de datos desde tu red local.

## Características

- **Sistema Operativo:** Ubuntu 22.04.
- **Acceso SSH:**  Conexión remota segura al contenedor mediante OpenSSH.
- **Sincronización de Archivos:**  Monta el directorio del proyecto en `/workspace` dentro del contenedor, permitiendo la sincronización bidireccional de archivos.
- **Bases de Datos Preconfiguradas:** (se puede cambiar la version desde el `docker-compose.yml`)
  - MySQL 8.0
  - MongoDB 6.0
  - Redis 7.0
  - PostgreSQL 15
- **Lenguajes de Programación:**  Instala Python (latest), Java/JDK Temurin (11 & 17), NVM (Node.js a necesidad), y Go (latest).
- **Red Personalizada (ipvlan):** Permite asignar una IP estática al contenedor SSH desde tu red local, facilitando la conexión.
- **ZSH con Powerlevel10k:**  Shell ZSH preconfigurada con un tema atractivo y plugins útiles (ver sección ZSH más abajo).
- **Volúmenes Persistentes (Bind Mounts):** Los datos de las bases de datos se almacenan en un directorio *específico* de tu host, garantizando la persistencia incluso si los contenedores se eliminan.
- **Docker-outside-Docker (DooD):** Permite utilizar el CLI de Docker dentro del contenedor `devcontainer-ssh` para gestionar contenedores en el host Docker.

## Requisitos Previos

- Docker y Docker Compose instalados en tu sistema.
- Conocimiento básico de la línea de comandos.
- Acceso a una red local (para la configuración de la red `ipvlan`).
- (Opcional, pero recomendado) Un cliente SSH.

## Instalación y Uso

1. **Clona el repositorio:**

    ```bash
    git clone [https://github.com/Joacohbc/my-devcontainer-installer.git](https://github.com/Joacohbc/my-devcontainer-installer.git)
    cd my-devcontainer-installer
    ```

2. **Crea el archivo `.env` y configura las variables:**

    Crea un archivo llamado `.env` en el mismo directorio que `docker-compose.yml`.  Añade las siguientes variables, ajustando los valores según tu configuración:

    ```.env
    DEVCONTAINER_SSH_IP=192.168.X.X  # IP estática para el contenedor SSH
    NETWORK_RANGE=192.168.X.X/24       # Rango de tu red local
    GATEWAY_IP=192.168.0.X            # IP de tu router
    MOUNT_VOLUMES=/home/user/docker_volumenes # Ruta ABSOLUTA donde se guardarán los datos
    ```

    - **`DEVCONTAINER_SSH_IP`**: Elige una IP *libre* dentro de tu red local que no esté en uso por otro dispositivo.
    - **`NETWORK_RANGE` y `GATEWAY_IP`**:  Deben coincidir con la configuración de tu red local.
    - **`MOUNT_VOLUMES`**:  *Debes* reemplazar `/home/user/docker_volumenes` con la ruta *absoluta* al directorio donde quieres que se almacenen los datos de las bases de datos.  *Antes* de ejecutar `docker-compose up`, crea esta estructura de directorios:

   ```bash
   mkdir -p /home/user/docker_volumenes/{mysql,mongo,redis,postgres}
   sudo chown -R $USER:$USER /home/user/docker_volumenes # Otorga permisos a tu usuario
   ```

   Asegúrate de reemplazar la ruta `/home/user/docker_volumenes` por la correcta.

3. **Modifica `docker-compose.yml` (si es necesario):**

    - En la sección `networks` -> `local-network` -> `driver_opts`,  verifica que `parent: eth0` use el nombre correcto de tu interfaz de red.  Si no es `eth0`, cámbialo (usa `ip link show` para ver tus interfaces).

4. **Inicia los contenedores:**

    ```bash
    docker compose --env-file .env up -d
    ```

    La primera vez, Docker descargará las imágenes, lo que puede tardar.

5. **Conéctate al contenedor por SSH:**

    ```bash
    ssh root@<DEVCONTAINER_SSH_IP>
    ```

    Reemplaza `<DEVCONTAINER_SSH_IP>` con la IP que configuraste en el archivo `.env`.  La contraseña del usuario `root` es `rootpass`.

6. **Ejecutar scripts de inicio (opcional):**

    Si deseas configurar ZSH y las herramientas de desarrollo, ejecuta los scripts `zsh-installer.sh` y `setup.sh` *dentro del contenedor* (en ese orden):

    ```bash
    ./zsh-installer.sh
    ./setup.sh
    ```

7. **Detener y eliminar los contenedores (y volúmenes):**

    Cuando hayas terminado, para detener los contenedores *y eliminar los volúmenes nombrados* (pero no los datos en tu SSD, que están seguros gracias a los bind mounts), usa:

    ```bash
    docker-compose down -v
    ```

    El `-v` es importante para eliminar los volúmenes nombrados internos de Docker Compose, que *no* son los que contienen tus datos persistentes.

## Estructura de Archivos

- **`Dockerfile`:**  Define la imagen base del contenedor de desarrollo (Ubuntu 22.04, SSH, herramientas básicas, Docker CLI).
- **`docker-compose.yml`:**  Define los servicios (contenedor SSH, bases de datos), la red, y los volúmenes. Incluye el montaje del socket de Docker para DooD.
- **`entrypoint.sh`:**  Script que se ejecuta al iniciar el contenedor SSH; configura el servicio SSH y añade el usuario `devuser` al grupo `docker` para DooD.
- **`.env`:**  Archivo de configuración para variables de entorno (IP, rutas, etc.).
- **`zsh-installer.sh`:** (Opcional) Instala y configura ZSH, Oh My Zsh, Powerlevel10k y plugins.
- **`setup.sh`:** (Opcional) Script para instalar herramientas de desarrollo adicionales.

## Conexión a las Bases de Datos

Desde el contenedor SSH (y también desde tu host, gracias a la configuración de red), puedes conectarte a las bases de datos usando las siguientes cadenas de conexión:

- **MySQL:** `mysql://devuser:devpass@mysql:3306/devdb`
- **MongoDB:** `mongodb://devuser:devpass@mongo:27017`  (Nota: el usuario/contraseña se configuran en el archivo `.env` o en `docker-compose.yml`, si no usas un archivo .env).
- **Redis:** `redis://redis:6379` (Redis por defecto no tiene autenticación; puedes configurarla si lo necesitas).
- **PostgreSQL:** `postgresql://devuser:devpass@postgres:5432/devdb`

## ZSH

El contenedor viene con ZSH preinstalado. Si ejecutas `zsh-installer.sh`, se configurará lo siguiente:

- **Oh My Zsh:** Framework para gestionar la configuración de ZSH.
- **Powerlevel10k:** Tema de ZSH rápido y personalizable.
- **Plugins:**
  - `zsh-syntax-highlighting`: Resaltado de sintaxis.
  - `zsh-history-substring-search`: Búsqueda en el historial.
  - `zsh-autosuggestions`: Sugerencias de comandos.
- **Fuentes:** Instala las fuentes MesloLGS NF necesarias para Powerlevel10k.

Al finalizar la ejecución de `zsh-installer.sh`, se te guiará a través del asistente de configuración de Powerlevel10k. Sigue las instrucciones para personalizar la apariencia de tu prompt.

## Docker-outside-Docker (DooD)

Esta configuración incluye la funcionalidad Docker-outside-Docker (DooD), que te permite ejecutar comandos Docker (como `docker ps`, `docker run`, etc.) desde dentro del contenedor `devcontainer-ssh`. Estos comandos interactuarán con el daemon de Docker que se ejecuta en tu máquina host.

Esto se logra mediante:

1. **Montaje del Socket de Docker:** El archivo `docker-compose.yml` monta el socket de Docker del host (`/var/run/docker.sock`) dentro del contenedor `devcontainer-ssh`.
2. **Instalación del Docker CLI:** El `Dockerfile` instala el cliente de línea de comandos de Docker (`docker-ce-cli`) dentro del contenedor. No se instala el motor completo de Docker, solo el cliente.
3. **Permisos de Usuario:** El script `entrypoint.sh` añade el usuario `devuser` al grupo `docker` dentro del contenedor, lo que le otorga los permisos necesarios para interactuar con el socket de Docker.

### Deshabilitar Docker-outside-Docker

Si no necesitas esta funcionalidad, puedes deshabilitarla comentando las siguientes líneas en los archivos correspondientes (busca los comentarios que indican cómo hacerlo):

- **`docker-compose.yml`**: La línea que monta `/var/run/docker.sock`.
- **`Dockerfile`**: Las líneas que instalan `ca-certificates`, `curl`, `gnupg`, `lsb-release` (si no son necesarias para otros paquetes), las líneas que configuran la GPG key y el repositorio de Docker, y la línea que instala `docker-ce-cli`.
- **`entrypoint.sh`**: La línea que añade `devuser` al grupo `docker`.

## Notas Importantes

- **Persistencia de Datos:** Los datos de las bases de datos se almacenan en el directorio que especificaste en `MOUNT_VOLUMES` en tu *host*.  Estos datos *no* se eliminan cuando usas `docker-compose down -v`.
- **Puertos:**  Los puertos de las bases de datos están expuestos en tu host, lo que te permite conectarte a ellas desde aplicaciones en tu máquina local.
- **Red ipvlan:**  La red `ipvlan` permite que el contenedor SSH tenga una IP directamente accesible desde tu red local.  Esto facilita la conexión SSH y el acceso a las bases de datos.
- **Usuario y contraseña**: Las contraseñas que se encuentran el el docker-compose.yml son solo a modo de ejemplo, se recomienda cambiarlas por cuestiones de seguridad.
