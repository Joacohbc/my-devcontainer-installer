# Configuración de Cloudflare Tunnel (Red Privada)

Esta guía explica cómo configurar una **Red Privada** utilizando Cloudflare Tunnel y Docker. Esto permite acceder a tu entorno de desarrollo (`devcontainer-ssh`) y otros servicios de forma segura desde cualquier lugar utilizando el cliente WARP, sin exponer puertos a internet.

## 1. Configuración de Red (Opcional)

El proyecto ya viene preconfigurado para utilizar una subred específica para Docker. Si necesitas cambiar el rango de red por defecto (por ejemplo, para evitar conflictos con tu red local), puedes hacerlo mediante el archivo `.env`.

### Crear archivo `.env`

Crea un archivo `.env` en la raíz de tu proyecto con el siguiente contenido:

```bash
TUNNEL_TOKEN=tu_token_aqui
DOCKER_SUBNET=172.25.0.0/24
```

*   **TUNNEL_TOKEN**: Tu token de Cloudflare Tunnel.
*   **DOCKER_SUBNET**: El rango de red que utilizará Docker (por defecto `172.25.0.0/24` si no se especifica).

> **Nota:** Si cambias la subred después de haber iniciado los contenedores, deberás recrearlos con: `docker compose up -d --force-recreate`.

## 2. Configurar Cloudflare Zero Trust

Una vez definida tu red (o usando el valor por defecto `172.25.0.0/24`), debes indicar a Cloudflare que enrute el tráfico hacia allí.

1.  Ve al **Zero Trust Dashboard** -> **Networks** -> **Tunnels / Connectors**.
2.  Busca tu túnel y selecciona **Configure**.
3.  Ve a la pestaña **Private Network**.
4.  Añade el CIDR que estás utilizando (ej: **`172.25.0.0/24`**).
5.  Guarda los cambios.

## 3. Configurar Cliente WARP (Split Tunnels)

Para que tu dispositivo (PC, Móvil) sepa que debe enviar el tráfico de esa IP a través de WARP:

1.  En el Dashboard de Zero Trust, ve a **Team & Resources** -> **Devices** -> **Device Profiles**.
2.  Edita tu perfil activo.
3.  En la sección **Split Tunnels**, asegúrate de que el rango `172.25.0.0/24` (o el que hayas elegido):
    *   Esté **Incluido** (si usas modo "Include IPs").
    *   O **No esté Excluido** (si usas modo "Exclude IPs", que es el predeterminado).

## 4. Acceso y Uso

Docker asignará una dirección IP automáticamente a cada contenedor dentro del rango definido.

### Obtener la IP del contenedor

Usa el comando `docker inspect` para ver la IP asignada actualmente al servicio que te interesa.

**Para el entorno SSH (`devcontainer-ssh`):**

```bash
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' devcontainer-ssh
```

**Para Bases de Datos:**

```bash
# PostgreSQL
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' postgres

# MongoDB
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' mongo
```

### Conexión SSH

Una vez obtengas la IP (por ejemplo, `172.25.0.X`) y con WARP activo en tu dispositivo cliente:

```bash
ssh devuser@<IP_OBTENIDA>
```
