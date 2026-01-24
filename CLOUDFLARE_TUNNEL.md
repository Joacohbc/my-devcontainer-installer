# Configuración de Cloudflare Tunnel (Red Privada)

Esta guía explica cómo configurar una **Red Privada** utilizando Cloudflare Tunnel y Docker. Esto permite acceder a tu entorno de desarrollo (`devcontainer-ssh`) y otros servicios de forma segura desde cualquier lugar utilizando el cliente WARP, sin exponer puertos a internet.

## 1. Preparar `docker-compose.yml`

Para que el túnel funcione correctamente con la función "Private Network" de Cloudflare, necesitamos definir una subred conocida para Docker. Usaremos una variable de entorno para definir este rango (`DOCKER_SUBNET`).

### A. Crear archivo `.env`
Crea o edita el archivo `.env` en la raíz de tu proyecto y define tu rango de red preferido:

```bash
TUNNEL_TOKEN=tu_token_aqui
DOCKER_SUBNET=172.25.0.0/24
```

### B. Editar `docker-compose.yml`
Edita tu archivo `docker-compose.yml` para configurar la red `local-network` usando esa variable:

```yaml
version: '3.8'

services:
# ... (servicios)

networks:
  local-network:
    driver: bridge
    ipam:
      config:
        - subnet: ${DOCKER_SUBNET:-172.25.0.0/24}  # <--- Usamos la variable o un valor por defecto
```

> **Nota:** Al aplicar este cambio, deberás recrear los contenedores: `docker compose up -d --force-recreate`.

## 2. Configurar Cloudflare Zero Trust

Una vez que tu red Docker tiene un rango conocido (el valor de `DOCKER_SUBNET`, ej. `172.25.0.0/24`), debemos indicarle a Cloudflare que enrute el tráfico hacia allí a través del túnel.

1.  Ve al **Zero Trust Dashboard** -> **Networks** -> **Tunnels**.
2.  Busca tu túnel y entra en **Configure**.
3.  Ve a la pestaña **Private Network**.
4.  Añade el CIDR que definiste en tu variable (ej: **`172.25.0.0/24`**).
5.  Guarda los cambios.

## 3. Configurar Cliente WARP (Split Tunnels)

Para que tu dispositivo (PC, Móvil) sepa que debe enviar el tráfico de esa IP a través de WARP:

1.  En el Dashboard de Zero Trust, ve a **Settings** -> **WARP Client** -> **Device Profiles**.
2.  Edita tu perfil activo.
3.  En la sección **Split Tunnels**, asegúrate de que el rango `172.25.0.0/24` (o el que hayas elegido):
    *   Esté **Incluido** (si usas modo "Include IPs").
    *   O **No esté Excluido** (si usas modo "Exclude IPs", que es el predeterminado).
    *   Generalmente, las reglas "Exclude" por defecto ignoran las IPs privadas (como 172.16.0.0/12). Deberás **eliminar** la exclusión de `172.16.0.0/12` o agregar explícitamente tu subred a las inclusiones para que el tráfico pase por el túnel.

## 4. Acceso y Uso

Como no hemos fijado IPs manualmente, Docker asignará una dirección automáticamente a cada contenedor dentro del rango definido (ej. `172.25.0.0/24`). Para conectarte, primero necesitas averiguar qué IP se le asignó al servicio que te interesa.

### Pasos para conectarte:

1.  **Obtener la IP del contenedor:**
    Usa el comando `docker inspect` para ver la IP asignada actualmente.

    *   **Para el entorno SSH (`devcontainer-ssh`):**
        ```bash
        docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' devcontainer-ssh
        ```
    *   **Para PostgreSQL:**
        ```bash
        docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' postgres
        ```
    *   **Para MongoDB:**
        ```bash
        docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' mongo
        ```

2.  **Conexión SSH:**
    Una vez obtengas la IP (por ejemplo, `172.25.0.X`) y con WARP activo:
    ```bash
    ssh devuser@<IP_OBTENIDA>
    ```

> **Nota:** Aunque la IP es dinámica, al fijar la subred, las IPs siempre estarán dentro de ese rango conocido por el túnel.

## Comandos de Verificación
Si necesitas validar qué rango de red se está usando realmente:

Si necesitas inspeccionar las IPs asignadas actualmente:

**Obtener la IP del contenedor SSH:**
```bash
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' devcontainer-ssh
```

**Obtener información de la subred de Docker:**
```bash
docker network inspect my-devcontainer-installer_local-network | grep Subnet
```
*(Nota: El nombre de la red puede variar según el nombre del directorio padre, usa `docker network ls` para verificar).*
