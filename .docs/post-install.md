# Pasos Posteriores a la Instalación

Acciones y comandos para conectar y trabajar dentro del contenedor una vez iniciado el entorno de desarrollo.

> [!TIP]
> Antes de realizar inicios de sesión manuales: si activó el volumen de configuración compartida, ejecute `devcontainer-cli config shared sync` en la máquina anfitriona (*host*) para sembrar las credenciales y sesiones que ya posee (GitHub CLI, Claude Code, Codex, Antigravity). De este modo, los contenedores arrancan ya autenticados.

---

## Acceso al DevContainer (SSH y CLI Shell)

Es posible conectarse al contenedor desde el anfitrión mediante `devcontainer-cli` o mediante un cliente OpenSSH estándar, tanto en modo **interactivo** como **no interactivo**:

### 1. Vía CLI Shell (`devcontainer-cli shell`)

Utiliza `docker exec` internamente, resolviendo automáticamente el contenedor del espacio de trabajo actual:

- **Modo Interactivo (predeterminado)**: Abre una terminal interactiva (Zsh) con soporte completo de pseudo-terminal (TTY).
  ```bash
  devcontainer-cli shell                       # Entra a la shell interactiva del devcontainer
  devcontainer-cli shell -- bash               # Abre la shell especificando bash
  devcontainer-cli shell -c <contenedor>       # Entra a un contenedor específico del stack (ej. postgres)
  ```
- **Modo No Interactivo (`-T` / `--no-tty`)**: Ejecuta comandos aislados o canalizaciones (*pipes* / redirecciones de archivos) desactivando la asignación de TTY para evitar la corrupción de datos binarios (como retornos de carro `\r`).
  ```bash
  devcontainer-cli shell -- uname -a           # Ejecuta un comando puntual y retorna la salida
  devcontainer-cli shell -T -- pg_dump -U devuser devdb > backup.sql # Generación limpia de volcados de BD
  cat script.sh | devcontainer-cli shell -T -- bash                 # Envío de un script local al contenedor
  ```

### 2. Vía SSH (`devcontainer-cli ssh` / cliente `ssh`)

Proporciona acceso mediante OpenSSH, idóneo para **VS Code Remote - SSH** o sesiones multiplexadas con **Zellij**:

- **Modo Persistente (predeterminado)**:
  ```bash
  devcontainer-cli ssh                         # Abre sesión SSH (ejecuta configuración automática la primera vez)
  devcontainer-cli ssh --remote user@server    # Conecta por SSH a un devcontainer en un servidor remoto
  ssh <workspace>                              # Conexión directa mediante el alias registrado por la CLI
  ```
  El alias reside en `~/.ssh/devcontainer-cli.config` (archivo propio de la CLI, incluido desde `~/.ssh/config` con una directiva `Include`).

- **Modo Efímero (`--ephemeral`)**:
  ```bash
  devcontainer-cli ssh --ephemeral             # Conecta directamente a la IP sin tocar archivos ~/.ssh/
  ```
  Ideal para scripts de automatización, agentes de IA y CI/CD donde no se desea registrar alias ni modificar configuraciones locales en el host.

- **Modo No Interactivo**:
  ```bash
  ssh <workspace> "ls -la /workspaces"         # Ejecuta un comando remoto por SSH de forma no interactiva
  cat script_local.sh | ssh <workspace> "bash" # Transmite y ejecuta un script local mediante SSH
  ```

---

## Scripts Embebidos en `~/post-script/`

La CLI incluye scripts dentro de la imagen según los módulos seleccionados:

```bash
~/post-script/login-github-cli.sh    # Inicio de sesión de GitHub CLI (módulo github-cli)
sudo ~/post-script/update_golang.sh  # Actualiza Go a la última versión estable (módulo go; requiere sudo)
```

### Herramientas de IA (módulos `ai-clis`)

Los instaladores no interactivos se ejecutan automáticamente en segundo plano al iniciar el contenedor, una sola vez por ciclo de vida (un centinela en `~/.post-script-state/<script>.done` evita reinstalaciones tras `stop`/`start`):

```bash
~/post-script/start.d/50-install-claude-code.sh      # Claude Code
~/post-script/start.d/50-install-opencode.sh         # OpenCode
~/post-script/start.d/50-install-antigravity.sh      # Antigravity CLI
~/post-script/start.d/50-install-copilot.sh          # GitHub Copilot
~/post-script/start.d/90-install-graphify.sh         # Graphify
~/post-script/start.d/90-install-caveman.sh          # Caveman
~/post-script/start.d/90-install-browser-harness.sh  # Browser Harness
```

Si no incluyó un módulo en la imagen inicial, puede materializar cualquiera de estos instaladores en un contenedor en ejecución con:
```bash
devcontainer-cli copy --asset <nombre>
```

---

## Gestión de Bases de Datos

Los siguientes comandos se ejecutan desde el anfitrión utilizando el prefijo del espacio de trabajo (`<workspace>-postgres`):

### Reiniciar la Base de Datos (Reset)
Elimina y recrea el esquema `public` de PostgreSQL:
```bash
devcontainer-cli shell -c <workspace>-postgres -- psql -U devuser -d devdb -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
```

### Exportar la Base de Datos (Backup)
Utilice el parámetro `-T` para no corromper el volcado con caracteres de control de terminal:
```bash
devcontainer-cli shell -c <workspace>-postgres -T -- pg_dump -U devuser --no-owner --no-acl devdb > backup.sql
```
