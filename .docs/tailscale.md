# Configuración de Acceso Privado con Tailscale

Esta guía explica cómo conectarse de forma privada a sus devcontainers desde cualquier dispositivo de su *tailnet* (**SSH**, **VS Code Remote - SSH**, aplicaciones web en navegador/móvil y APIs REST) usando el patrón principal con **Tailscale en el servidor anfitrión (*host*)** —o su alternativa con **un contenedor gateway en Docker conectado mediante `access network connect`**— sin modificar los contenedores ni abrir puertos hacia `0.0.0.0`.

> [!NOTE]
> **Documentación oficial de referencia (Tailscale)**:
> - Instalación y puesta en marcha en Linux / host: <https://tailscale.com/docs/install/linux> (<https://tailscale.com/kb/1031/install-linux>)
> - Publicación HTTPS privada dentro de la tailnet (`tailscale serve`): <https://tailscale.com/kb/1312/serve>
> - Referencia oficial de Tailscale en Docker (`tailscale/tailscale`): <https://tailscale.com/docs/features/containers/docker>

---

## 0. Modelo recomendado y preparación del host

`devcontainer-cli` adopta **Tailscale en el host** (decisión [`D29`](architecture.md#d29-tailscale-en-el-host-o-gateway-desacoplado-frente-a-sidecar-embebido)): el demonio de Tailscale corre en el servidor anfitrión donde reside Docker, no dentro de cada devcontainer. Esto garantiza compatibilidad idéntica con los tres modos de compilación (`custom`, `profiles`, `image`) y con contenedores temporales (`run`), sin requerir `/dev/net/tun` ni capacidades privilegiadas (`NET_ADMIN`) en los contenedores.

En el servidor Linux anfitrión (consulte la guía oficial para su distribución):

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
```

Para comprobar la IP asignada en su tailnet (`100.x.y.z`):

```bash
tailscale ip -4
```

---

## 1. Acceso SSH y VS Code Remote - SSH (`--via`)

Desde su equipo cliente (laptop o estación de trabajo conectada a la misma tailnet), registre el túnel SSH hacia el contenedor remoto utilizando `--via` sobre la identidad Tailscale del servidor:

```bash
devcontainer-cli access ssh --via <usuario>@<ip-o-magicdns-tailscale> --container <workspace>-devcontainer-ssh
```

Una vez registrado:

```bash
ssh <workspace>-devcontainer-ssh
```

- Puede seleccionar el alias `<workspace>-devcontainer-ssh` directamente en **VS Code Remote - SSH**.
- Su clave privada permanece en el cliente; el puerto `22` del contenedor nunca se expone hacia interfaces públicas ni a la tailnet.

---

## 2. Exponer puertos web evitando fallos de *Secure Context*

> [!WARNING]
> Al abrir una aplicación web por HTTP plano sobre una IP remota (`http://100.x.y.z:3000`), los navegadores desactivan por seguridad los **Contextos Seguros (*Secure Contexts*)**, lo que rompe APIs como `crypto.randomUUID()`, WebCrypto y Service Workers (PWA). Elija el patrón adecuado según el cliente.

### Opción A: Escritorio / Laptop mediante túnel SSH local (recomendado, cero cambios en el proyecto)

Tras configurar `--via`, cree el túnel en `127.0.0.1` desde su propio equipo cliente:

```bash
devcontainer-cli access forward 3000 --alias <workspace>-devcontainer-ssh
```

O bien, ejecutando el forward en el servidor y encadenando con `ssh -L` desde el cliente:

```bash
# En el servidor (host):
devcontainer-cli access forward 3000

# En su laptop (cliente):
ssh -L 3000:127.0.0.1:3000 <usuario>@<ip-o-magicdns-tailscale>
```

Abra `http://localhost:3000` en su navegador local (`localhost` es *Secure Context* nativo y Vite lo acepta sin ajustes adicionales).

### Opción B: Móvil / PWA / Multi-dispositivo con HTTPS automático (`tailscale serve`)

Requiere tener **MagicDNS** y **HTTPS Certificates** habilitados en la consola DNS de Tailscale (véase <https://tailscale.com/kb/1312/serve>).

1. En el servidor anfitrión, reenvíe el puerto del contenedor **solo hacia loopback (`127.0.0.1`)**:
   ```bash
   devcontainer-cli access forward 3000
   ```
2. En otra terminal del servidor, publique ese puerto local con TLS automático dentro de su tailnet:
   ```bash
   sudo tailscale serve --bg 3000
   ```
3. Acceda desde cualquier dispositivo de su tailnet a `https://<nombre-host>.<nombre-tailnet>.ts.net`.
4. Para verificar o retirar la publicación cuando termine:
   ```bash
   tailscale serve status
   tailscale serve reset
   ```

> [!TIP]
> En proyectos con **Vite 6+**, añada su dominio `*.ts.net` (o el hostname exacto) en `server.allowedHosts` dentro de `vite.config.ts` para evitar el bloqueo de *DNS rebinding* al entrar por HTTPS.

### Opción C: APIs REST y servicios sin frontend interactivo

Para endpoints HTTP/TCP consumidos por `curl`, clientes móviles nativos o herramientas backend donde no aplica *Secure Context*, puede enlazar el túnel **únicamente** a la IP privada de Tailscale del host:

```bash
# Humano (o guardando el alias):
devcontainer-cli access forward 100.x.y.z:8000:8000

# Agente (modo efímero):
devcontainer-cli agent forward 100.x.y.z:8000:8000
```

> [!CAUTION]
> **Nunca** utilice `0.0.0.0:<puerto>:<puerto>` por defecto. Enlace siempre a `127.0.0.1` (default) o explícitamente a la IP `100.x.y.z` de Tailscale.

---

## 3. Reenvío automático de múltiples puertos (`--forward-ports`)

Para evitar escribir túneles uno por uno en cada sesión, declare los puertos en el proyecto al generarlo (o en el campo `forward_ports:` de un perfil):

```bash
devcontainer-cli agent create --modules nodejs,pnpm --forward-ports 3000,8000
```

Luego, dentro del directorio del proyecto, ejecute sin argumentos para abrir todos los túneles en paralelo:

```bash
devcontainer-cli access forward
```

---

## 4. Alcanzar servicios de su tailnet desde dentro del contenedor (`reverse:`)

Si necesita que una aplicación dentro del devcontainer consuma una base de datos o API privada ubicada en otra máquina de su tailnet (o en el propio host):

```bash
devcontainer-cli access forward reverse:5432:<ip-o-magicdns-tailnet>:5432
```

*Nota: si especifica un puerto de escucha distinto dentro del contenedor (p. ej. `reverse:80:100.x.y.z:8080`), el puerto del contenedor debe ser `1024` o superior porque la sesión SSH corre como `devuser`.*

---

## 5. Alternativa en Docker: Contenedor Gateway + `access network connect`

Si **no desea o no puede instalar Tailscale en el sistema operativo del host**, puede ejecutar **un único contenedor `tailscale/tailscale`** independiente en Docker actuando como *Subnet Router* y conectarlo bajo demanda a la red interna de cualquier proyecto mediante `devcontainer-cli access network connect`.

De este modo:
- No se modifican el `Dockerfile` ni el `docker-compose.yml` generados (funciona con `custom`, `profiles`, `image` y `run`).
- Un único nodo Tailscale ve por red interna Docker las IPs y servicios de todos los proyectos a los que lo conecte.

### Puesta en marcha del contenedor gateway

Despliegue el contenedor gateway fuera de los directorios de proyectos (requiere modo kernel con `/dev/net/tun`, `NET_ADMIN`, `NET_RAW` e `ip_forward`; consulte <https://tailscale.com/docs/features/containers/docker>):

```yaml
# docker-compose.yml independiente (infraestructura Tailscale en el host Docker)
services:
  tailscale-gateway:
    image: tailscale/tailscale:latest
    container_name: tailscale-gateway
    hostname: devcontainer-gateway
    restart: unless-stopped
    devices:
      - /dev/net/tun:/dev/net/tun
    cap_add:
      - NET_ADMIN
      - NET_RAW
    sysctls:
      - net.ipv4.ip_forward=1
    environment:
      - TS_AUTHKEY=${TS_AUTHKEY}
      - TS_STATE_DIR=/var/lib/tailscale
      - TS_AUTH_ONCE=true
      - TS_USERSPACE=false
      - TS_ACCEPT_DNS=false
      # Rango por defecto que asigna devcontainer-cli (172.25.0.0/28, .16/28, ...),
      # o sustituya por los CIDR exactos (DOCKER_SUBNET) de sus proyectos activos:
      - TS_ROUTES=172.25.0.0/16
    volumes:
      - tailscale-gateway-state:/var/lib/tailscale

volumes:
  tailscale-gateway-state:
```

> [!IMPORTANT]
> Tras levantar `tailscale-gateway`, **apruebe la ruta anunciada** (`172.25.0.0/16` o las subredes `/28` concretas) en la consola de administración de Tailscale (*Machines* -> *Edit route settings*, o automáticamente mediante `autoApprovers` en su política ACL).

### Conectar y operar con sus devcontainers

1. Desde el directorio de cada proyecto ya levantado (`devcontainer-cli workspace up`), conecte el gateway a su red Docker aislada:
   ```bash
   devcontainer-cli access network connect tailscale-gateway
   ```
2. Consulte en `.dc_<workspace>/build/.env` la IP fija asignada al contenedor (`DEVCONTAINER_IP`, por ejemplo `172.25.0.14`) y conéctese directamente desde cualquier equipo de su tailnet:
   ```bash
   ssh devuser@172.25.0.14
   ```
3. Para abrir un frontend en navegador manteniendo **Secure Context (`localhost`)**, encadene un túnel SSH local hacia esa IP interna:
   ```bash
   ssh -L 3000:127.0.0.1:3000 devuser@172.25.0.14
   ```
4. **Antes de apagar o destruir el proyecto** (`workspace down` o `agent destroy`), desconecte el gateway para liberar la red del workspace:
   ```bash
   devcontainer-cli access network disconnect tailscale-gateway
   ```

### Cuándo elegir Tailscale en Host vs Contenedor Gateway (`access network connect`)

| Aspecto | Tailscale en el Host *(Recomendado)* | Contenedor Gateway + `access network connect` |
|---|---|---|
| **Instalación en el SO anfitrión** | Sí (`tailscale` en el host) | No (solo un contenedor `tailscale/tailscale`) |
| **Requisitos del kernel/Docker** | Ninguno especial en contenedores | Exige `/dev/net/tun`, `NET_ADMIN` e `ip_forward=1` |
| **HTTPS nativo (`tailscale serve`)** | Directo desde el host tras `access forward` | No directo entre contenedores (`tailscale serve` solo acepta `localhost`/`127.0.0.1` como destino) |
| **Aprobación en Tailscale Admin** | No requiere | Sí requiere aprobar `TS_ROUTES` (o ACL `autoApprovers`) |
| **Colisiones entre múltiples servidores** | Ninguna | Posible si varios servidores anuncian `172.25.0.0/16`; use `DOCKER_SUBNET` específicos por servidor si comparte tailnet |

*(Nota: la alternativa de un sidecar dedicado por proyecto con `network_mode: service:tailscale` sigue documentada en [`D29`](architecture.md#d29-tailscale-en-el-host-o-gateway-desacoplado-frente-a-sidecar-embebido) y no forma parte del stack estándar generado por la CLI).*
