# Pasos Post-Instalación

Acciones que se ejecutan **dentro del contenedor** una vez levantado. Para instalar la CLI y configurar el acceso SSH, ver [README.md](README.md).

## Cambiar la contraseña del DevUser (opcional)

La contraseña temporal del primer login se obtiene en el host con `docker compose logs devcontainer-ssh | grep 'devuser password' | tail -1`. Una vez dentro podés cambiarla:

```bash
sudo passwd
```

## Scripts horneados en `~/post-script/`

La CLI hornea estos scripts dentro de la imagen según los módulos elegidos:

```bash
~/post-script/login-github-cli.sh   # login de GitHub CLI (módulo github-cli)
sudo ~/post-script/update_golang.sh  # actualiza Go a la última estable (módulo go; requiere root, escribe en /usr/local/go)
```

### CLIs de IA (módulos `claude-code`, `opencode`, `codex-cli`, `antigravity-cli`, `copilot-cli`)

Si generaste el entorno con alguno de los módulos de IA correspondientes, sus instaladores quedan disponibles. Ejecutá el que necesites:

```bash
~/post-script/install-claude-code.sh   # Claude Code (@anthropic-ai/claude-code)
~/post-script/install-opencode.sh      # OpenCode (opencode.ai)
~/post-script/install-codex-cli.sh     # Codex CLI (usa npx, no requiere instalación global)
~/post-script/install-antigravity.sh   # Antigravity CLI
~/post-script/install-copilot.sh       # GitHub Copilot CLI
```

Si no incluiste el módulo, podés instalarlas manualmente, por ejemplo:

```bash
pnpm install -g @anthropic-ai/claude-code   # Claude Code
pnpm install -g @google/gemini-cli          # Gemini CLI
curl -fsSL https://opencode.ai/install | bash  # OpenCode
```

## Instalar Firebase Tools (opcional)

```bash
pnpm install -g firebase-tools
firebase login
```

## Bases de datos

> Requiere los servicios de DB y el módulo `dbclients` (clientes `psql`/`mongosh`/`redis-cli`). Credenciales y comandos de conexión en [README.md → Bases de Datos](README.md#bases-de-datos).

### Resetear la base de datos

Elimina y recrea el schema `public` de PostgreSQL:

```bash
docker exec -it postgres psql -U devuser -d devdb -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
```

> ⚠️ **Advertencia**: elimina **todos** los datos y tablas. Úsalo solo cuando necesites reiniciar el estado de la base de datos.

### Exportar la base de datos (backup)

```bash
docker exec -it postgres pg_dump -U devuser --no-owner --no-acl devdb > backup.sql
```

`--no-owner` y `--no-acl` omiten propietario y permisos (útil para portabilidad); `> backup.sql` guarda el dump en el directorio actual.
