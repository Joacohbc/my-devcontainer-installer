# Arquitectura y Funcionamiento de SSH (`devcontainer-cli`)

Guía detallada sobre cómo gestiona `devcontainer-cli` la clave SSH compartida, la configuración guardada (modo persistente) y las conexiones sin rastro (modo efímero).

---

## Resumen rápido: ¿qué modo usar?

| Modo | Comando principal | ¿Escribe archivos SSH en el host? | Cuándo usarlo |
|---|---|---|---|
| **Guardado (Persistente)** | `devcontainer-cli access ssh` | Sí, de forma aislada (`~/.ssh/devcontainer-cli.config` + `known_hosts` de la CLI) | Uso diario, `ssh <workspace>`, **VS Code Remote - SSH**, JetBrains Gateway |
| **Efímero (`--ephemeral`)** | `devcontainer-cli access ssh --ephemeral` (por defecto en `agent ssh` y `agent forward`) | **No** (cero modificaciones en `~/.ssh/`) | Agentes autónomos de IA, CI/CD, scripts, contenedores temporales |
| **Shell directa (`docker exec`)** | `devcontainer-cli access shell` / `agent exec` | No usa SSH | Comandos puntuales o terminal rápida sin necesitar túneles ni `ssh-agent` |

---

## 1. La clave SSH compartida

En lugar de crear una clave distinta por cada proyecto, `devcontainer-cli` utiliza **una única clave SSH compartida** reutilizada en todos tus contenedores (locales y remotos):

- **Ruta por defecto**: `~/.config/devcontainer-cli/ssh/id_devcontainer` (y su clave pública `.pub`).
- **Algoritmo**: `ed25519` sin frase de paso (`ssh-keygen -t ed25519`).
- **Creación automática**: se genera la primera vez que ejecutas `access ssh` (o `access ssh --ephemeral` / `agent ssh`), o bien manualmente con:
  ```bash
  devcontainer-cli config ssh-key --generate
  ```

### Usar tu propia clave y precedencia

Si ya tienes una clave personal (por ejemplo `~/.ssh/id_ed25519`), puedes configurar que la CLI la use como predeterminada en `~/.config/devcontainer-cli/config.json`:

```bash
devcontainer-cli config ssh-key                      # Ver la ruta actual y si existe
devcontainer-cli config ssh-key ~/.ssh/id_ed25519    # Usar tu propia clave por defecto
devcontainer-cli config ssh-key --unset              # Volver a ~/.config/devcontainer-cli/ssh/id_devcontainer
```

Al conectar, la clave se resuelve en este orden de precedencia:
1. Bandera explícita `--key <ruta>`.
2. Clave configurada con `config ssh-key`.
3. Clave compartida gestionada (`~/.config/devcontainer-cli/ssh/id_devcontainer`).

### Cómo se autoriza dentro del contenedor

Cada vez que se configura SSH (y también al abrir una conexión efímera), la CLI envía la clave pública por `docker exec -i` hacia `~/.ssh/authorized_keys` del usuario objetivo (`devuser` por defecto):
- Resuelve el directorio personal real del usuario en vivo mediante `getent passwd` (por si el entrypoint remapeó el UID/GID de `devuser` al arrancar).
- Agrega la clave pública de forma idempotente (`sort -u` para evitar duplicados) y fija permisos estrictos (`700` en `~/.ssh`, `600` en `authorized_keys`).

> La clave privada **nunca** se copia dentro del contenedor.

### Exportar acceso a otra máquina (`--export`)

Si el contenedor corre en un servidor y quieres conectarte desde otro equipo sin copiar archivos a mano, ejecuta en el servidor:

```bash
devcontainer-cli config ssh-key --export usuario@servidor
```

Imprime un bloque autocontenido que configura `~/.ssh/id_devcontainer` y el `Host` con `ProxyCommand` en la máquina cliente. Como incluye la clave privada, trátalo como un secreto.

---

## 2. Modo Guardado / Persistente (`devcontainer-cli access ssh`)

Diseñado para que `ssh <workspace>` y **VS Code Remote - SSH** funcionen siempre sin contaminar tus archivos personales ni mostrar alertas al reconstruir imágenes.

### Archivos que toca y aislamiento

1. **`~/.ssh/devcontainer-cli.config`** *(personalizable con `devcontainer-cli config ssh-config-file <ruta>`)*:
   Aquí escribe la CLI todos sus bloques `Host <alias>`, cada uno precedido por un marcador estructurado:
   ```text
   # devcontainer-cli:managed v=1 kind=workspace ref=mi-proyecto alias=mi-proyecto
   Host mi-proyecto
       HostName 172.25.0.14
       User devuser
       IdentityFile /home/tu-usuario/.config/devcontainer-cli/ssh/id_devcontainer
       IdentitiesOnly yes
       UserKnownHostsFile /home/tu-usuario/.config/devcontainer-cli/ssh/known_hosts
       StrictHostKeyChecking accept-new
       HashKnownHosts no
   ```
2. **`~/.ssh/config`**:
   La CLI **nunca reescribe tu `~/.ssh/config`**. Solo agrega una única directiva al inicio:
   ```text
   Include devcontainer-cli.config
   ```
   *(Si tenías bloques antiguos dentro de `~/.ssh/config` de versiones previas, los migra automáticamente a `devcontainer-cli.config`).*
3. **`~/.config/devcontainer-cli/ssh/known_hosts`**:
   Archivo `known_hosts` propio y aislado del `~/.ssh/known_hosts` del usuario (no es sobrescribible por bandera).

### ¿Por qué un `known_hosts` propio? (*Host-Key Pinning*)

Cuando reconstruyes una imagen Docker, el contenedor nuevo regenera sus claves de host SSH mientras suele conservar la misma IP interna (por ejemplo `172.25.0.14`). Si esas claves vivieran en `~/.ssh/known_hosts`, SSH abortaría con `REMOTE HOST IDENTIFICATION HAS CHANGED`.

Al aislarlas en `~/.config/devcontainer-cli/ssh/known_hosts` con `HashKnownHosts no`, la CLI lee las claves actuales del contenedor mediante Docker y las **re-fija automáticamente** antes de cada conexión, sin tocar jamás tu `~/.ssh/known_hosts`.

### Anclaje por nombre, no por IP

- El bloque guardado identifica el objetivo por **nombre de contenedor** (`<workspace>-devcontainer-ssh`, o el pasado con `--container`).
- Si el contenedor se recrea con otra IP privada, `access ssh` actualiza el `HostName` en su sitio antes de abrir la sesión.
- Si el contenedor fue eliminado, falla de inmediato informando que el alias está obsoleto en lugar de colgarse esperando un timeout SSH.
- Con `--setup` puedes forzar la regeneración del bloque y la reinstalación de la clave pública antes de conectar.

### Modo remoto guardado (`--via`)

```bash
devcontainer-cli access ssh --via usuario@servidor --container mi-proyecto-devcontainer-ssh
```

Guarda un bloque `Host` con `ProxyCommand ssh usuario@servidor "nc -q0 $(docker inspect ...) 22"`, resolviendo la IP del contenedor en vivo en el servidor remoto al conectar. Solo necesitas pasar `--via` la primera vez: las siguientes conexiones con `access ssh --container ...` o `ssh <alias>` recuerdan el salto y fijan la clave bajo el alias.

### Limpieza

- `devcontainer-cli workspace destroy` elimina el bloque `Host` y las claves fijadas del proyecto en `known_hosts`.
- `devcontainer-cli config clean ssh` detecta y limpia entradas huérfanas de contenedores o proyectos eliminados (incluidas las creadas con `--via` cuando el servidor remoto es accesible).

---

## 3. Modo Efímero (`--ephemeral`)

```bash
devcontainer-cli access ssh --ephemeral
devcontainer-cli access ssh --ephemeral --via usuario@servidor -c mi-proyecto-devcontainer-ssh
```

Es además el comportamiento **predeterminado** de `devcontainer-cli agent ssh` y `devcontainer-cli agent forward` (desactivable si se pasa explícitamente `--ephemeral=false` donde aplique).

### Qué hace internamente

1. **Cero escritura en `~/.ssh/`**: no lee ni modifica `~/.ssh/config`, `~/.ssh/devcontainer-cli.config` ni ningún `known_hosts`.
2. **Garantiza la clave en el contenedor**: aunque no escribe bloques `Host`, sí asegura que la clave compartida exista en el host e instala su mitad pública en `~/.ssh/authorized_keys` del contenedor (de lo contrario un contenedor recién creado sin `access ssh` previo devolvería `Permission denied (publickey)`).
3. **Resolución en vivo y conexión directa**:
   - Consulta al daemon Docker (local o vía `--via`) la IP privada actual del contenedor (con fallback a `127.0.0.1` si no tiene red).
   - Ejecuta `ssh` directo al **puerto 22** (usando `-J <via>` si se indicó `--via`) con:
     - `-i <clave>`
     - `-p 22`
     - `-o StrictHostKeyChecking=no`
     - `-o UserKnownHostsFile=/dev/null`
     - `-o LogLevel=ERROR`

### Por qué existe este modo

- Permite a **agentes de IA**, pipelines de **CI/CD** y scripts conectarse por SSH o abrir túneles (`agent forward`) contra cualquier contenedor —incluidos los temporales de `agent run`— sin dejar bloques `Host` ni claves huérfanas que limpiar después.
- Evita conflictos con alias existentes en `~/.ssh/config` durante ejecuciones desatendidas.
