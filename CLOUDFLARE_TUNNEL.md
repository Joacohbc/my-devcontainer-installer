# Configuración de Cloudflare Tunnel (Red Privada)

Esta guía explica cómo configurar una **Red Privada** utilizando Cloudflare Tunnel y Docker. Esto permite acceder a tu entorno de desarrollo (`devcontainer-ssh`) y otros servicios de forma segura desde cualquier lugar utilizando el cliente WARP, sin exponer puertos a internet.

## 1. Configuración de Red (Opcional)

La CLI elige automáticamente una subred libre (preferentemente `172.25.0.0/28`) y la persiste en el `.env` generado. Si necesitas cambiar el rango después, edita el `.env` antes de iniciar los contenedores.

### Archivo `.env` generado

La CLI escribe algo como:

```bash
DOCKER_SUBNET=172.25.0.0/28
DEVCONTAINER_IP=172.25.0.14   # último host válido de la subred
TUNNEL_TOKEN=tu_token_aqui    # solo si seleccionaste el servicio 'tunnel'
```

*   **TUNNEL_TOKEN**: Tu token de Cloudflare Tunnel (la CLI lo pregunta si seleccionas `--service tunnel`).
*   **DOCKER_SUBNET**: Rango que usará Docker. Default `172.25.0.0/28` (16 IPs). La CLI detecta colisiones y sugiere otra subred libre si hace falta.
*   **DEVCONTAINER_IP**: IP fija reservada para el contenedor SSH, derivada de la subred. Útil porque queda estable entre reinicios.

> **Nota:** Si cambias la subred después de haber iniciado los contenedores, deberás recrearlos con: `docker compose up -d --force-recreate`.

## 2. Configurar Cloudflare Zero Trust

Una vez definida tu red (o usando el valor por defecto `172.25.0.0/28`), debes indicar a Cloudflare que enrute el tráfico hacia allí.

1.  Ve al **Zero Trust Dashboard** -> **Networks** -> **Tunnels / Connectors**.
2.  Busca tu túnel y selecciona **Configure**.
3.  Ve a la pestaña **Private Network**.
4.  Añade el CIDR exacto que aparece en tu `.env` (ej: **`172.25.0.0/28`**).
5.  Guarda los cambios.

## 3. Configurar Cliente WARP (Split Tunnels)

Para que tu dispositivo (PC, Móvil) sepa que debe enviar el tráfico de esa IP a través de WARP:

1.  En el Dashboard de Zero Trust, ve a **Team & Resources** -> **Devices** -> **Device Profiles**.
2.  Edita tu perfil activo.
3.  En la sección **Split Tunnels**, asegúrate de que el rango configurado (default `172.25.0.0/28`, o el que hayas elegido):
    *   Esté **Incluido** (si usas modo "Include IPs").
    *   O **No esté Excluido** (si usas modo "Exclude IPs", que es el predeterminado).

## 4. Acceso y Uso

El contenedor SSH recibe una IP **fija** (`DEVCONTAINER_IP` del `.env`, derivada del último host de la subred). El resto de los servicios recibe una IP dinámica dentro del rango definido.

### Obtener la IP del contenedor

Para el contenedor SSH normalmente alcanza con leer `DEVCONTAINER_IP` del `.env`. Si necesitas la IP runtime, usa `docker inspect`. Los contenedores se prefijan con el `workspace` configurado en la CLI, así que sustituye `<workspace>` por ese nombre (o consulta `docker ps`):

> Si un contenedor está conectado a varias redes Docker, `inspect` devuelve
> una IP por línea; `head -n1` se queda con la primera.

**Para el entorno SSH (`<workspace>-devcontainer-ssh`):**

```bash
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{"\n"}}{{end}}' <workspace>-devcontainer-ssh | head -n1
```

**Para Bases de Datos:**

```bash
# PostgreSQL
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{"\n"}}{{end}}' <workspace>-postgres | head -n1

# MongoDB
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{"\n"}}{{end}}' <workspace>-mongo | head -n1
```

### Conexión SSH

Una vez obtengas la IP (típicamente la `DEVCONTAINER_IP` del `.env`) y con WARP activo en tu dispositivo cliente:

```bash
ssh devuser@<IP_OBTENIDA>
```
