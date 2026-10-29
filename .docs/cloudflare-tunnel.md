# Configuración de Cloudflare Tunnel (Red Privada)

Esta guía explica cómo configurar una **Red Privada** utilizando Cloudflare Tunnel y Docker. Esto permite acceder a su entorno de desarrollo (`devcontainer-ssh`) y otros servicios de forma segura desde cualquier lugar utilizando el cliente WARP, sin exponer puertos a internet.

---

## 0. Instalar cloudflared en el Contenedor

`cloudflared` es un **módulo del Dockerfile** (`cloudflared`): se instala **dentro** del devcontainer, no como contenedor hermano. Selecciónelo en el asistente interactivo (categoría *Dev Tools*) o mediante:

```bash
devcontainer-cli --with cloudflared
```

Existen tres formas de iniciar un túnel, de menor a mayor grado de configuración:

1. **Quick tunnel (gratuito, sin cuenta y sin inicio de sesión)**:
   No requiere token ni inicio de sesión. Desde dentro del contenedor:
   ```bash
   cloudflared tunnel --url http://localhost:8080
   ```
   Imprime una URL `https://<algo>.trycloudflare.com` y comienza a servir dicho puerto inmediatamente. La URL es **efímera**: se destruye con el proceso y cambia en cada ejecución; sirve para demostraciones puntuales o validación de webhooks, no como dirección estable.

2. **Túnel con nombre y token de connector**:
   Pase el token al comando dentro del contenedor:
   ```bash
   cloudflared tunnel run --token <token>
   ```
   El connector queda autorizado contra el túnel creado en el panel de control. Es el método recomendado para Red Privada + WARP, pues ofrece una dirección fija. La CLI no solicita dicho token en la generación; se provee en la sesión o se exporta manualmente como variable `TUNNEL_TOKEN`.

3. **Túnel con nombre sin token**:
   Inicie sesión desde dentro del contenedor con `cloudflared tunnel login` (el certificado se almacena en `~/.cloudflared/cert.pem`). Es el único método que autoriza comandos administrativos como `cloudflared tunnel route ip add`.

> [!WARNING]
> Los tres métodos exponen el puerto sin autenticación previa salvo que se configure una política de Access en Zero Trust.

---

## 1. Configuración de Red

La CLI selecciona automáticamente una subred libre (por defecto `172.25.0.0/28`) y la persiste en el archivo `.env` generado:

```bash
DOCKER_SUBNET=172.25.0.0/28
DEVCONTAINER_IP=172.25.0.14   # último host válido de la subred
```

- **`DOCKER_SUBNET`**: Rango utilizado por Docker (16 IPs).
- **`DEVCONTAINER_IP`**: IP fija reservada para el contenedor SSH, derivada de la subred para mantenerse estable entre reinicios.

---

## 2. Configurar Cloudflare Zero Trust

1. Acceda al **Zero Trust Dashboard** -> **Networks** -> **Tunnels / Connectors**.
2. Seleccione su túnel y haga clic en **Configure**.
3. Diríjase a la pestaña **Private Network**.
4. Añada el CIDR exacto que aparece en su archivo `.env` (ej. `172.25.0.0/28`).
5. Guarde los cambios sin asignar ningún *Public Hostname* al túnel para mantener el tráfico estrictamente privado.

### Alternativa por Línea de Comandos (`route ip add`)
```bash
devcontainer-cli agent exec -w -- \
  cloudflared tunnel route ip add 172.25.0.0/28 <tunnel-name-o-uuid>
```

---

## 3. Configurar el Cliente WARP (Split Tunnels)

En el panel de Zero Trust (**Team & Resources** -> **Devices** -> **Device Profiles** -> perfil activo):
- En la sección **Split Tunnels**, asegúrese de que el rango (`172.25.0.0/28`) esté **Incluido** (en modo Include IPs) o **No esté Excluido** (en modo Exclude IPs).

---

## 4. Conexión SSH a través de la Red Privada

Con el cliente WARP activo en su equipo anfitrión:

```bash
ssh devuser@172.25.0.14
```
