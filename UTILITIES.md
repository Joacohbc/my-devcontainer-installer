# Utilidades en Contenedor

Guía práctica de utilidades incluidas en las imágenes de `devcontainer-cli` para mantener procesos interactivos persistentes dentro del contenedor.

---

## Procesos Interactivos y TUI en Segundo Plano con Zellij

Las aplicaciones interactivas de terminal (TUI), CLIs de agentes o procesos de larga duración requieren una pseudo-terminal (PTY) real para funcionar. Si se lanzan con `nohup ... < /dev/null &`, fallan de inmediato por ausencia de TTY; si se ejecutan en una shell normal, se cierran al ejecutar `exit`.

Como **Zellij** viene preinstalado en todas las imágenes (`zellij` es un módulo siempre activo), permite crear sesiones desacopladas con una PTY persistente, independientes de tu sesión de shell y del proceso PID 1 (`sshd`) del contenedor.

### Crear y lanzar un proceso en segundo plano

Desde el directorio del workspace dentro del contenedor:

```bash
# 1. Crear una sesión de Zellij desacoplada en segundo plano
zellij attach --create-background <sesion>

# 2. Ejecutar tu comando dentro de esa sesión
zellij --session <sesion> run -- <comando>
```

También puedes lanzarlo directamente desde el host sin abrir una shell interactiva:

```bash
devcontainer-cli agent exec -w -- zellij attach --create-background <sesion>
devcontainer-cli agent exec -w -- zellij --session <sesion> run -- <comando>
```

### Verificar, reconectarse y salir

```bash
# Listar sesiones activas de Zellij
zellij list-sessions

# Volver a conectarse a la interfaz en vivo
zellij attach <sesion>
# (o desde el host)
devcontainer-cli access shell -- zellij attach <sesion>
```

- **Desacoplar sin cerrar el proceso**: presione `Ctrl+O` y luego `D`.
- **Salir del contenedor**: escriba `exit`; la sesión de Zellij y el proceso seguirán activos en segundo plano mientras el contenedor esté en ejecución.

---

## 2. Procesos No Interactivos en Segundo Plano (`nohup` + `disown`)

Para comandos que **no** requieren una interfaz TUI ni PTY (por ejemplo, servidores HTTP o workers simples), basta con redirigir entrada/salida y desvincular el trabajo de la shell:

```bash
nohup mi-comando > /tmp/mi-comando.log 2>&1 < /dev/null &
disown
exit
```

Para inspeccionar la salida posteriormente:

```bash
tail -f /tmp/mi-comando.log
```

---

## 3. Acceso Privado y Túneles Remotos (Cloudflare Tunnel y Tailscale)

Para conectarse a devcontainers o publicar puertos de forma segura desde otros dispositivos sin exponer puertos en `0.0.0.0`, utilice una de estas dos estrategias según dónde desee ejecutar el cliente de red:

| Método | Dónde corre | Ideal cuando... | Guía completa en el repo | Documentación oficial |
|---|---|---|---|---|
| **Tailscale (en el host)** | Servidor anfitrión (*host*) | Administra el servidor Docker; compatible con `custom`, `profiles`, `image` y `run` sin modificar contenedores y con HTTPS directo vía `tailscale serve` | [`.docs/tailscale.md`](.docs/tailscale.md) | [Linux](https://tailscale.com/docs/install/linux) · [Serve](https://tailscale.com/kb/1312/serve) · [Docker](https://tailscale.com/docs/features/containers/docker) |
| **Tailscale Gateway en Docker (`access network connect`)** | Contenedor `tailscale/tailscale` independiente unido a cada proyecto | No quiere instalar Tailscale en el host y prefiere enrutar la red interna Docker (`DOCKER_SUBNET`) bajo demanda con `access network connect` | [`.docs/tailscale.md#5-alternativa-en-docker-contenedor-gateway--access-network-connect`](.docs/tailscale.md#5-alternativa-en-docker-contenedor-gateway--access-network-connect) | [Docker](https://tailscale.com/docs/features/containers/docker) |
| **Cloudflare Tunnel (`cloudflared`)** | Dentro del devcontainer (módulo `cloudflared`) | No puede/quiere instalar nada en el host ni usar `/dev/net/tun`, y prefiere acceso Zero Trust por IP privada (`DOCKER_SUBNET`) con WARP | [`.docs/cloudflare-tunnel.md`](.docs/cloudflare-tunnel.md) | [Private Networks](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/private-net/) |

### Resumen rápido: Tailscale (Host o Contenedor Gateway)

- **SSH / VS Code remoto (Host)** desde el cliente:
  ```bash
  devcontainer-cli access ssh --via <usuario>@<ip-o-magicdns-tailscale> --container <workspace>-devcontainer-ssh
  ```
- **Web en escritorio (*Secure Context* en `localhost`)**: ejecute `devcontainer-cli access forward 3000 --alias <workspace>-devcontainer-ssh` en el cliente (o `access forward 3000` en el host + `ssh -L 3000:127.0.0.1:3000 <usuario>@<ip-o-magicdns-tailscale>`).
- **Web móvil / PWA con HTTPS**: `devcontainer-cli access forward 3000` (en `127.0.0.1`) + `sudo tailscale serve --bg 3000` en el host.
- **API en IP Tailscale** *(nunca `0.0.0.0`)*: `devcontainer-cli access forward <ip_tailscale>:8000:8000`.
- **Contenedor Gateway en Docker** *(sin instalar Tailscale en el host)*: conecte su contenedor `tailscale-gateway` (*Subnet Router* con `TS_ROUTES`) a cada proyecto mediante `devcontainer-cli access network connect tailscale-gateway` y conéctese a `ssh devuser@<DEVCONTAINER_IP>` (recuerde ejecutar `devcontainer-cli access network disconnect tailscale-gateway` antes de `workspace down` / `agent destroy`).

### Resumen rápido: Cloudflare Tunnel en el contenedor

1. Incluya el módulo `cloudflared` al generar el proyecto:
   ```bash
   devcontainer-cli agent create --modules <sus-módulos>,cloudflared
   ```
2. Inicie el túnel dentro del contenedor (para dejarlo en segundo plano sin TUI, aplique `nohup` + `disown` de la **Sección 2**):
   ```bash
   devcontainer-cli access shell -- cloudflared tunnel run --token <SU_TOKEN>
   ```
3. Publique la subred `DOCKER_SUBNET` del `.env` en Cloudflare Zero Trust (*Private Network* + *WARP Split Tunnels*) y conéctese por `ssh devuser@<DEVCONTAINER_IP>`.

> Consulte todos los detalles de configuración, seguridad (*Secure Contexts*) y pasos de Zero Trust en [`.docs/tailscale.md`](.docs/tailscale.md) y [`.docs/cloudflare-tunnel.md`](.docs/cloudflare-tunnel.md).
