# Configuración de Cloudflare Tunnel (Red Privada)

Esta guía explica cómo configurar una **Red Privada** utilizando Cloudflare Tunnel y Docker. Esto permite acceder a tu entorno de desarrollo (`devcontainer-ssh`) y otros servicios de forma segura desde cualquier lugar utilizando el cliente WARP, sin exponer puertos a internet.

## 0. Instalar cloudflared en el contenedor

`cloudflared` es un **módulo del Dockerfile** (`cloudflared`), igual que `ngrok`:
se instala **dentro** del devcontainer, no como contenedor hermano. Selecciónalo
en el wizard (categoría *Dev Tools*) o con:

```bash
devcontainer-cli --with cloudflared
```

Hay **tres** formas de levantar un túnel, de menos a más configuración:

*   **Quick tunnel: gratis, sin cuenta y sin login.** No necesitas token ni
    hacer login: desde dentro del contenedor,

    ```bash
    cloudflared tunnel --url http://localhost:8080
    ```

    imprime una URL `https://<algo>.trycloudflare.com` y empieza a servir ese
    puerto al instante. La URL es **efímera**: se muere con el proceso y cambia
    en cada ejecución, así que sirve para una demo puntual o probar un webhook,
    no como dirección estable.
*   **Túnel con nombre, con token de connector**: pasa el token al comando
    dentro del contenedor,

    ```bash
    cloudflared tunnel run --token <token>
    ```

    y el connector queda autorizado contra el túnel que creaste en el dashboard.
    Es el único que da una URL fija y es el que usa el resto de esta guía (Red
    Privada + WARP). **La CLI no pregunta por ese token ni lo inyecta**: no hay
    variable de entorno que rellenar, el token se da aquí (o se exporta a mano
    como `TUNNEL_TOKEN` en la sesión, que es lo que lee `cloudflared` cuando
    omites la bandera).
*   **Túnel con nombre, sin token**: haz el login desde dentro del contenedor con
    `cloudflared tunnel login` (el certificado queda en `~/.cloudflared`). Es
    además el único camino que autoriza los comandos de cuenta como
    `cloudflared tunnel route ip add`.

> **⚠️ Los tres exponen el puerto a internet sin ninguna autenticación por
> delante**, quick tunnels incluidos: cualquiera con la URL llega a tu servicio.

> **Nota:** El túnel ahora termina **dentro** del devcontainer, así que alcanza
> los puertos en `localhost` directamente, sin saltos de red. Como contrapartida,
> el túnel solo corre mientras el contenedor esté levantado.
>
> Versiones anteriores lo ofrecían como servicio de compose (`--service tunnel`),
> que levantaba un contenedor `cloudflared` aparte. Ese servicio ya no existe: al
> abrir un proyecto viejo la CLI lo convierte sola al módulo. El `TUNNEL_TOKEN`
> que tuvieras en el `.env` sigue ahí y no se toca, pero ya **no** se pasa al
> contenedor: cópialo a `cloudflared tunnel run --token <token>`.

## 1. Configuración de Red (Opcional)

La CLI elige automáticamente una subred libre (preferentemente `172.25.0.0/28`) y la persiste en el `.env` generado. Si necesitas cambiar el rango después, edita el `.env` antes de iniciar los contenedores.

### Archivo `.env` generado

La CLI escribe algo como:

```bash
DOCKER_SUBNET=172.25.0.0/28
DEVCONTAINER_IP=172.25.0.14   # último host válido de la subred
```

> **Nota:** el `.env` de un proyecto viejo puede traer todavía un `TUNNEL_TOKEN`.
> La CLI ya no pregunta por él ni lo pasa al contenedor; queda ahí como dato
> tuyo, para copiarlo a `cloudflared tunnel run --token`.

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

> **No configures ningún *Public Hostname* en ese túnel.** Es lo único que lo
> mantiene fuera de internet: la Private Network no crea DNS ni ruta pública, y
> un Public Hostname sí (y queda abierto a cualquiera, salvo que le pongas una
> policy de Access delante).

### Alternativa: hacerlo por comando (`cloudflared tunnel route ip`)

Los pasos 1-5 tienen equivalente en CLI, así que la ruta se puede crear (y
verificar) sin abrir el dashboard. Como el módulo `cloudflared` vive **dentro**
del devcontainer, el comando se lanza a través de la CLI:

```bash
devcontainer-cli agent exec -w -- \
  cloudflared tunnel route ip add 172.25.0.0/28 <tunnel-name-o-uuid>
```

El CIDR es el `DOCKER_SUBNET` de tu `.env`; `<tunnel-name-o-uuid>` es el túnel
que ya creaste en el dashboard (o con `cloudflared tunnel create`).

El resto de subcomandos:

| Comando | Qué hace |
|---|---|
| `cloudflared tunnel route ip show` | Lista las rutas de la cuenta (alias: `list`) |
| `cloudflared tunnel route ip get <IP>` | Dice qué túnel sirve esa IP |
| `cloudflared tunnel route ip delete <CIDR>` | Borra la ruta |

Si usas *virtual networks* para separar subredes que se solapan entre proyectos,
todos aceptan `--vnet <nombre>`:

```bash
cloudflared tunnel route ip add --vnet proyecto-a 172.25.0.0/28 mi-tunel
```

#### Requiere el certificado de origen, no el token del connector

Esto es lo que más confunde: `route ip` es una operación de **gestión a nivel
cuenta**, y se autentica con el *origin certificate* (`~/.cloudflared/cert.pem`),
no con el token del connector. Ese token solo autoriza **correr** el túnel; con
él solo, `route ip add` falla por credenciales.

Para obtener el certificado, desde dentro del contenedor:

```bash
devcontainer-cli agent exec -w -- cloudflared tunnel login
```

Imprime una URL: ábrela en el navegador del host, elige la cuenta/zona, y el
certificado queda en `~/.cloudflared/cert.pem` **dentro del contenedor**. Si lo
tienes en otra ruta, pásalo con `--origincert <path>`.

> **⚠️ Ese certificado no sobrevive a un recreate.** `~/.cloudflared` no es una
> entrada del volumen de configuración compartida (`types.SharedConfigEntries`),
> así que vive en la capa escribible del contenedor: aguanta `stop`/`start`, pero
> `destroy` o cualquier rebuild lo borran y hay que repetir el login.

En la práctica eso importa menos de lo que parece, porque **la ruta es estado de
tu cuenta de Cloudflare, no del contenedor**: se crea una vez y sigue viva aunque
el `cert.pem` desaparezca, y el connector arranca con `tunnel run --token` sin
necesitar el certificado para nada. Si aun así quieres automatizarlo (recrear el
proyecto y que la ruta se registre sola), usa la API REST —
`POST /accounts/{account_id}/teamnet/routes` con un API token de cuenta con
permiso *Cloudflare Tunnel: Edit* — que no depende de ningún archivo dentro del
contenedor.

#### Un CIDR, un túnel

Cloudflare no admite dos rutas con el mismo CIDR en la misma cuenta (o en la
misma virtual network). La CLI elige una subred libre **en tu host**, no en tu
cuenta de Cloudflare, así que dos proyectos pueden salir ambos con
`172.25.0.0/28` y el segundo `route ip add` va a fallar. Fija un `DOCKER_SUBNET`
distinto en el `.env` de cada proyecto antes del primer `up`, o sepáralos con
`--vnet`.

#### Verificar

```bash
devcontainer-cli agent exec -w -- cloudflared tunnel route ip show
```

La ruta debe aparecer apuntando a tu túnel. Lo que mantiene el acceso **privado**
es que ese túnel no tenga ningún *Public Hostname* configurado: la Private
Network no crea DNS ni ruta pública, solo la alcanzan los dispositivos con WARP
enrolado en tu organización (paso 3).

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
