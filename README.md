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
- **Lenguajes de Programación:**  Instala Python (latest), Java/JDK Temurin (11 & 17), NVM (Node.js a necesidad), Go (latest), y SQLite.
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
    ```

    - **`DEVCONTAINER_SSH_IP`**: Elige una IP *libre* dentro de tu red local que no esté en uso por otro dispositivo.
    - **`NETWORK_RANGE` y `GATEWAY_IP`**:  Deben coincidir con la configuración de tu red local.

    Nota: Las bases de datos servidor (MySQL, PostgreSQL, etc.) utilizan volúmenes de Docker para la persistencia de datos (ej: `mysql_data:/var/lib/mysql`). Estos volúmenes son gestionados por Docker y sus datos persisten aunque los contenedores se eliminen. Las bases de datos SQLite, al ser archivos, se guardarán y persistirán dentro de tu directorio de proyecto (`/workspace` en el contenedor) si las creas allí, ya que este directorio se sincroniza con tu máquina host.

3. **Modifica `docker-compose.yml` (si es necesario):**

    - En la sección `networks` -> `local-network` -> `driver_opts`,  verifica que `parent: eth0` use el nombre correcto de tu interfaz de red.  Si no es `eth0`, cámbialo (usa `ip link show` para ver tus interfaces).

4. **Inicia los contenedores:**

    ```bash
    docker compose --env-file .env up -d
    ```

    La primera vez, Docker descargará las imágenes, lo que puede tardar.

5. **Conéctate al contenedor por SSH:**

    Una vez que los contenedores estén en funcionamiento, puedes conectarte por SSH.
    Reemplaza `<DEVCONTAINER_SSH_IP_OR_HOSTNAME>` en los comandos de ejemplo con:
    - La IP estática que configuraste para `DEVCONTAINER_SSH_IP` en tu archivo `.env` (si estás utilizando el modo de red bridge/ipvlan).
    - `localhost` si estás utilizando el modo de red local con reenvío de puertos (por ejemplo, si mapeaste el puerto 2222 del host al puerto 22 del contenedor, te conectarías a `localhost` en el puerto 2222).

    Las contraseñas para los usuarios `root` y `devuser` se generan aleatoriamente la primera vez que se inicia el contenedor `devcontainer-ssh`. Estas contraseñas se imprimen en los logs del contenedor. Para obtenerlas, ejecuta el siguiente comando en tu terminal del host después de que los contenedores hayan iniciado:

    ```bash
    docker logs devcontainer-ssh
    ```

    Busca en la salida las líneas que comienzan con `initial root password:` y `devuser password:`. Anota estas contraseñas, ya que las necesitarás para la conexión SSH.

    **Ejemplos de conexión SSH:**

    - Para conectar como `devuser` (recomendado para el desarrollo diario):

        ```bash
        ssh devuser@<DEVCONTAINER_SSH_IP_OR_HOSTNAME>
        # Ejemplo si usas modo local con puerto 2222 forwardeado a 22 del contenedor:
        # ssh -p 2222 devuser@localhost
        ```

    - Para conectar como `root`:

        ```bash
        ssh root@<DEVCONTAINER_SSH_IP_OR_HOSTNAME>
        # Ejemplo si usas modo local con puerto 2222 forwardeado a 22 del contenedor:
        # ssh -p 2222 root@localhost
        ```

    Se recomienda utilizar el usuario `devuser` para las tareas de desarrollo habituales.

6. **Ejecutar scripts de inicio (opcional):**

    Si deseas configurar ZSH y las herramientas de desarrollo, ejecuta los scripts `zsh-installer.sh`, `nvm-installer.sh` y `setup.sh` *dentro del contenedor* (en ese orden):

    ```bash
    # No debe ser con sudo, ya que se instalan $HOME del usuario
    ./zsh-installer.sh # Si deseas instalar ZSH, Oh My Zsh, Powerlevel10k y plugins
    # (Cerrar sesión y volver a entrar para que ZSH sea el shell por defecto)
    ./nvm-installer.sh  # Si deseas instalar NVM (Node.js) 
    sudo ./setup.sh # Si deseas instalar Java, Python, Go, etc.
    ```

7. **Detener y eliminar los contenedores:**

    Cuando hayas terminado, para detener los contenedores (*no elimina los volúmenes*), usa:

    ```bash
    docker-compose down
    ```

## Estructura de Archivos

- **`Dockerfile`:**  Define la imagen base del contenedor de desarrollo (Ubuntu 22.04, SSH, herramientas básicas, Docker CLI).
- **`docker-compose.yml`:**  Define los servicios (contenedor SSH, bases de datos), la red, y los volúmenes. Incluye el montaje del socket de Docker para DooD.
  - **Modos de Red para `devcontainer-ssh`:**
    - **Bridge (ipvlan):** Por defecto, el servicio `devcontainer-ssh` utiliza una red `ipvlan` (nombrada `local-network`) para obtener una IP directamente en tu red local. Esto se configura mediante la variable `DEVCONTAINER_SSH_IP` en el archivo `.env`.
    - **Local (Host Port Forwarding):** Si prefieres no usar `ipvlan` o necesitas una configuración más simple, puedes cambiar al modo local. Para ello:
      1. Comenta la sección `local-network` bajo el servicio `devcontainer-ssh` en `docker-compose.yml`.
      2. Descomenta la sección `ports` bajo el servicio `devcontainer-ssh` y ajusta el mapeo de puertos según sea necesario (por ejemplo, `"2222:22"` para mapear el puerto 2222 del host al puerto 22 del contenedor).
- **`entrypoint.sh`:**  Script que se ejecuta al iniciar el contenedor SSH; configura el servicio SSH y añade el usuario `devuser` al grupo `docker` para DooD.
- **`.env`:**  Archivo de configuración para variables de entorno (IP, rutas, etc.).
- **`zsh-installer.sh`:** (Opcional) Instala y configura ZSH, Oh My Zsh, Powerlevel10k y plugins.
- **`setup.sh`:** (Opcional) Script para instalar herramientas de desarrollo adicionales como Java, NVM (Node.js), Python, SQLite y Go.

## Conexión a las Bases de Datos

Desde el contenedor SSH (y también desde tu host, gracias a la configuración de red), puedes conectarte a las bases de datos usando las siguientes cadenas de conexión:

- **MySQL:** `mysql://devuser:devpass@mysql:3306/devdb`
- **MongoDB:** `mongodb://devuser:devpass@mongo:27017`  (Nota: el usuario/contraseña se configuran en el archivo `.env` o en `docker-compose.yml`, si no usas un archivo .env).
- **Redis:** `redis://redis:6379` (Redis por defecto no tiene autenticación; puedes configurarla si lo necesitas).
- **PostgreSQL:** `postgresql://devuser:devpass@postgres:5432/devdb`
- **SQLite:** Para SQLite, te conectarás directamente al archivo de la base de datos. Si el archivo está en tu proyecto (ej: `/workspace/mi_proyecto/datos.db`), usa `sqlite3 /workspace/mi_proyecto/datos.db`. Estos archivos se persisten como parte de tu proyecto sincronizado con el host.

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

- **Persistencia de Datos:** Los datos de las bases de datos servidor (MySQL, PostgreSQL, MongoDB, Redis) se almacenan en volúmenes de Docker definidos en `docker-compose.yml`. Estos datos *no* se eliminan cuando usas `docker-compose down -v` (a menos que elimines los volúmenes explícitamente con otros comandos de Docker). Los archivos de SQLite se persisten como parte de tu proyecto en `/workspace`.
- **Puertos:**  Los puertos de las bases de datos están expuestos en tu host, lo que te permite conectarte a ellas desde aplicaciones en tu máquina local.
- **Red ipvlan:**  La red `ipvlan` permite que el contenedor SSH tenga una IP directamente accesible desde tu red local.  Esto facilita la conexión SSH y el acceso a las bases de datos.
- **Usuario y contraseña**: Las contraseñas que se encuentran el el docker-compose.yml son solo a modo de ejemplo, se recomienda cambiarlas por cuestiones de seguridad.
