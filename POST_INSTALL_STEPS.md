# Pasos Post-Instalación

Acciones y comandos para conectar y trabajar **dentro del contenedor** una vez levantado.

> 💡 **Antes de hacer logins a mano:** si activaste el volumen de config compartida, ejecutá `devcontainer-cli config shared sync` en el **host** para sembrar los logins/sesiones que ya tenés (GitHub CLI, Claude, Codex, Gemini, Antigravity) — así los contenedores arrancan ya autenticados. Ver [README.md → Sembrar logins desde el host](README.md#sembrar-logins-desde-el-host-config-shared-sync). Los pasos manuales de abajo solo hacen falta para lo que `config shared sync` no cubre.

## Acceso al DevContainer (SSH y CLI Shell)

Puedes conectarte al contenedor desde la máquina host mediante `devcontainer-cli` o mediante un cliente SSH directo, tanto en modo **interactivo** (para trabajar en la terminal) como **no interactivo** (para automatizaciones, comandos puntuales o redirección de archivos):

### 1. Vía CLI Shell (`devcontainer-cli shell`)

Utiliza `docker exec` por debajo resolviendo automáticamente el contenedor del workspace actual:

* **Modo Interactivo (predeterminado):** Abre una terminal interactiva (Zsh) con soporte TTY completo.
  ```bash
  devcontainer-cli shell                       # Entra a la shell interactiva del devcontainer
  devcontainer-cli shell -- bash               # Abre la shell especificando bash
  devcontainer-cli shell -c <contenedor>       # Entra a un contenedor específico del stack (ej. postgres)
  ```
* **Modo No Interactivo (`-T` / `--no-tty`):** Ejecuta comandos aislados o canalizaciones (pipes / redirecciones de archivos) desactivando la asignación de TTY para evitar la corrupción de datos (como caracteres `\r`).
  ```bash
  devcontainer-cli shell -- uname -a           # Ejecuta un comando puntual y devuelve la salida
  devcontainer-cli shell -T -- pg_dump -U devuser devdb > backup.sql # Generación limpia de dumps de BD
  cat script.sh | devcontainer-cli shell -T -- bash                 # Pipe de un script local al contenedor
  ```

### 2. Vía SSH (`devcontainer-cli ssh` / cliente `ssh`)

Proporciona acceso mediante OpenSSH, ideal para **VS Code Remote - SSH** o sesiones multiplexadas con **Zellij**:

* **Modo Interactivo:**
  ```bash
  devcontainer-cli ssh                         # Abre sesión SSH (ejecuta setup-ssh automáticamente si es la 1ª vez)
  devcontainer-cli ssh --remote user@server    # Conecta por SSH a un devcontainer en un servidor remoto
  ssh <workspace>                              # Conexión directa mediante el alias registrado por la CLI
  ```
  El alias vive en `~/.ssh/devcontainer-cli.config` (archivo propio de la CLI, incluido desde `~/.ssh/config` con una línea `Include`; se cambia con `devcontainer-cli config ssh-config-file`).

* **Modo No Interactivo:**
  ```bash
  ssh <workspace> "ls -la /workspace"          # Ejecuta un comando remoto por SSH de forma no interactiva
  cat script_local.sh | ssh <workspace> "bash" # Transmite y ejecuta un script local mediante SSH
  ```

## Cambiar la contraseña del DevUser (opcional)

La contraseña temporal del primer login se obtiene en el host con `docker compose logs devcontainer-ssh | grep 'devuser password' | tail -1`. Una vez dentro podés cambiarla:

```bash
sudo passwd
```

## Scripts embebidos en `~/post-script/`

La CLI incluye estos scripts dentro de la imagen según los módulos elegidos:

```bash
~/post-script/login-github-cli.sh    # login de GitHub CLI (módulo github-cli) — o usá `config shared sync gh` en el host
sudo ~/post-script/update_golang.sh  # actualiza Go a la última estable (módulo go; requiere root, escribe en /usr/local/go)
```

### CLIs de IA (módulos `claude-code`, `opencode`, `codex-cli`, `antigravity-cli`, `copilot-cli`, `caveman`, `graphify`)

Los instaladores **no interactivos** se ejecutan **automáticamente al arrancar el contenedor**, en segundo plano y **una sola vez por contenedor** (un sentinel en `~/.post-script-state/<script>.done` evita reinstalar tras un `stop`/`start`; un contenedor nuevo reinstala). El orden está fijado: primero los agentes y al final las herramientas que se cablean sobre ellos (Graphify/Caveman).

```bash
~/post-script/start.d/50-install-claude-code.sh   # Claude Code      (auto)
~/post-script/start.d/50-install-opencode.sh      # OpenCode         (auto)
~/post-script/start.d/50-install-antigravity.sh   # Antigravity CLI  (auto)
~/post-script/start.d/50-install-copilot.sh       # GitHub Copilot   (auto)
~/post-script/start.d/90-install-graphify.sh      # Graphify         (auto, al final)
~/post-script/start.d/90-install-caveman.sh       # Caveman          (auto, al final)
```

Cada instalador auto-start también queda accesible con su nombre llano en `~/post-script/` (un symlink a `start.d/NN-...`), así que podés ejecutarlo a mano cuando quieras:

```bash
~/post-script/install-claude-code.sh   # corre el instalador manualmente (symlink a start.d/)
```

El progreso de cada ejecución automática queda en `~/.post-script-state/<script>.log`. Si querés que se reinstale solo en el próximo arranque, borrá el `.done` correspondiente y reiniciá el contenedor.

El instalador **interactivo** de Codex no se auto-ejecuta (lanza `npx @openai/codex` y requiere interacción); queda como script manual:

```bash
~/post-script/install-codex-cli.sh     # Codex CLI (usa npx, requiere interacción)
```

> 💡 **¿No incluiste el módulo?** No hace falta reinstalar la imagen: desde el host podés materializar cualquiera de estos instaladores en un contenedor en marcha con `devcontainer-cli copy --asset <nombre>` (ej. `copy --asset install-claude-code`) y luego ejecutarlo dentro. Ver [README.md → Copiar archivos y assets](README.md#copiar-archivos-y-assets-copy).

> Los logins de estas CLIs (Claude, Codex, Gemini, Antigravity) se siembran desde el host con `config shared sync`; ver la nota al inicio de este documento.

## Bases de datos

> Requiere los servicios de DB y el módulo `dbclients` (clientes `psql`/`mongosh`/`redis-cli`). Credenciales y comandos de conexión en [README.md → Bases de Datos](README.md#bases-de-datos).

> Los siguientes comandos corren desde el **host** y usan el nombre real del contenedor, que lleva el prefijo del workspace: `<workspace>-postgres` (consultá `docker ps` si dudás del nombre).

### Resetear la base de datos

Elimina y recrea el schema `public` de PostgreSQL. Podés usar `devcontainer-cli shell` (atajo de `docker exec -it`) apuntando al contenedor de la DB:

```bash
devcontainer-cli shell -c <workspace>-postgres -- psql -U devuser -d devdb -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
```

> ⚠️ **Advertencia**: elimina **todos** los datos y tablas. Úsalo solo cuando necesites reiniciar el estado de la base de datos.

### Exportar la base de datos (backup)

Para el backup pasá `-T`/`--no-tty`, que ejecuta sin TTY (`docker exec -i`) para no corromper el dump redirigido a archivo:

```bash
devcontainer-cli shell -c <workspace>-postgres -T -- pg_dump -U devuser --no-owner --no-acl devdb > backup.sql
```

`--no-owner` y `--no-acl` omiten propietario y permisos (útil para portabilidad); `> backup.sql` guarda el dump en el directorio actual.

> ℹ️ El flag `-T` es clave acá: sin él, `shell` abre una TTY (`-it`) e inserta `\r` en el stream, corrompiendo el dump. Para el reset, que solo imprime en pantalla, no hace falta.
