# my-devcontainer-installer

Este repositorio contiene la configuración para un contenedor Docker que facilita el desarrollo remoto. El contenedor incluye las siguientes herramientas y características:

- **Ubuntu 22.04:** Sistema operativo base.
- **OpenSSH:** Permite la conexión remota segura al contenedor.
- **Sudo:** Brinda privilegios de administrador al usuario `devuser`.
- **Sincronización de archivos:** Los cambios realizados en tu máquina local se reflejarán en el contenedor y viceversa.
- **Conexión a bases de datos:** El contenedor puede conectarse a instancias de MySQL, MongoDB, Redis y PostgreSQL.

1. **Clona este repositorio:**
   ```bash
   git clone https://github.com/Joacohbc/my-devcontainer-installer.git
   cd my-devcontainer-installer
   ```

2. **Modifica el archivo `docker-compose.yml`:**
    - En la sección `networks` -> `local-network` -> `driver_opts`, reemplaza `"eth0"` con el nombre de la interfaz de red de tu PC
    - Ajusta el rango de IPs de tu red local en `subnet` y la IP del router en `gateway` si es necesario.

3. **Inicia el contenedor:**
   ```bash
   docker-compose up -d
   ```

4. **Conéctate al contenedor por SSH:**
   ```bash
   ssh devuser@<IP>
   ```
   La contraseña para el usuario `devuser` es `devuser`. La contraseña del usuario root es `rootpassword`.

5. **Comienza a desarrollar:**
   Tu directorio de trabajo actual estará disponible en `/workspace` dentro del contenedor. Cualquier cambio que realices en tu máquina local se reflejará en el contenedor y viceversa.

6. **Ejecutar scripts de inicio:** `zsh-installer.sh` y `setup.sh` (en ese orden)

## Descripción de los archivos

- **`Dockerfile`:** Define la imagen Docker que se utilizará para crear el contenedor.
- **`docker-compose.yml`:** Define los servicios que se ejecutarán en el contenedor, incluyendo las bases de datos y la configuración de red.
- **`entrypoint.sh`:** Script que se ejecuta al iniciar el contenedor y configura el servicio SSH.

## Conexión a bases de datos

El contenedor puede conectarse a las siguientes bases de datos:

- **MySQL:** `mysql://devuser:devpass@mysql:3306/devdb`
- **MongoDB:** `mongodb://mongo:27017`
- **Redis:** `redis://redis:6379`
- **PostgreSQL:** `postgresql://devuser:devpass@postgres:5432/devdb`

## ZSH
- Oh My Zsh: Un framework popular para gestionar tu configuración de Zsh, con una gran cantidad de temas y plugins.
- Powerlevel10k: Un tema de Zsh rápido y personalizable que ofrece una apariencia moderna y muchas opciones de configuración.

Plugins esenciales:
- zsh-syntax-highlighting: Resalta la sintaxis de los comandos mientras los escribes, ayudando a detectar errores.
- zsh-history-substring-search: Permite buscar en tu historial de comandos usando subcadenas, mejorando la navegación.
- zsh-autosuggestions: Sugiere comandos mientras escribes basándose en tu historial, ahorrando tiempo y pulsaciones de teclas.

Además, el script instala las fuentes MesloLGS NF necesarias para que Powerlevel10k se vea correctamente.

