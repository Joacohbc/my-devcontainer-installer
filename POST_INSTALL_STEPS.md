# Pasos Post-Instalación

Acciones que se ejecutan **dentro del contenedor** una vez levantado. Para instalar la CLI y configurar el acceso SSH, ver [README.md](README.md).

> 💡 **Antes de hacer logins a mano:** si activaste el volumen de config compartida, ejecutá `devcontainer-cli sync-config` en el **host** para sembrar los logins/sesiones que ya tenés (GitHub CLI, Claude, Codex, Gemini, Antigravity) — así los contenedores arrancan ya autenticados. Ver [README.md → Sembrar logins desde el host](README.md#sembrar-logins-desde-el-host-sync-config). Los pasos manuales de abajo solo hacen falta para lo que `sync-config` no cubre.

## Cambiar la contraseña del DevUser (opcional)

La contraseña temporal del primer login se obtiene en el host con `docker compose logs devcontainer-ssh | grep 'devuser password' | tail -1`. Una vez dentro podés cambiarla:

```bash
sudo passwd
```

## Scripts horneados en `~/post-script/`

La CLI hornea estos scripts dentro de la imagen según los módulos elegidos:

```bash
~/post-script/login-github-cli.sh    # login de GitHub CLI (módulo github-cli) — o usá `sync-config gh` en el host
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

El progreso de cada uno queda en `~/.post-script-state/<script>.log`. Si querés reinstalar a mano, borrá el `.done` correspondiente y reiniciá el contenedor, o ejecutá el script directamente.

El instalador **interactivo** de Codex no se auto-ejecuta (lanza `npx @openai/codex` y requiere interacción); queda como script manual:

```bash
~/post-script/install-codex-cli.sh     # Codex CLI (usa npx, requiere interacción)
```

> 💡 **¿No incluiste el módulo?** No hace falta reinstalar la imagen: desde el host podés materializar cualquiera de estos instaladores en un contenedor en marcha con `devcontainer-cli copy --asset <nombre>` (ej. `copy --asset install-claude-code`) y luego ejecutarlo dentro. Ver [README.md → Copiar archivos y assets](README.md#copiar-archivos-y-assets-copy).

> Los logins de estas CLIs (Claude, Codex, Gemini, Antigravity) se siembran desde el host con `sync-config`; ver la nota al inicio de este documento.

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
