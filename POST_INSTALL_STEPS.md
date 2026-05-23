# Post Installation Steps

## Obtener la contraseña del DevUser

```bash
docker compose logs devcontainer-ssh | grep 'devuser password' | tail -1
# >>> devuser password: <password>
```

## Cambiar la contraseña (Opcional)

```bash
sudo passwd
```

## Loguearse con Github

Los scripts post-instalación se hornean dentro de la imagen, accesibles en `~/post-script/`:

```bash
~/post-script/login-github-cli.sh
```

## Actualizar Go (si elegiste el módulo `go`)

Si generaste el entorno con `--with go`:

```bash
sudo ~/post-script/update_golang.sh
```

Actualiza la instalación de Go a la última versión estable (requiere root porque escribe en `/usr/local/go`).

## Instalar Firebase Tools (Opcional)

```bash
pnpm install -g firebase-tools
```

```bash
firebase login
```

## Instalar CLIs de IA (Opcional)

Si generaste el entorno con `--with ai-clis`, los scripts de instalación quedan disponibles en `~/post-script/` dentro del contenedor. Ejecutá el que necesites:

```bash
~/post-script/install-claude-code.sh   # Claude Code (@anthropic-ai/claude-code)
~/post-script/install-opencode.sh      # OpenCode (opencode.ai)
~/post-script/install-codex-cli.sh     # Codex CLI (usa npx, no requiere instalación global)
~/post-script/install-antigravity.sh   # Antigravity CLI
~/post-script/install-copilot.sh       # GitHub Copilot CLI
```

Si no incluiste el módulo `ai-clis`, podés instalarlas manualmente. Ejemplos:

### Claude Code

```bash
pnpm install -g @anthropic-ai/claude-code
claude   # primera vez te pedirá autenticarte
```

### Gemini CLI

Te permite usar los modelos de IA de Google directamente desde tu terminal para tareas de codificación, refactorización y chat.

```bash
pnpm install -g @google/gemini-cli
gemini   # primera vez te pedirá autenticarte con tu cuenta de Google
```

### OpenCode

```bash
curl -fsSL https://opencode.ai/install | bash
```

## Resetear la base de datos (Opcional)

Si necesitas limpiar completamente la base de datos y empezar desde cero, puedes eliminar y recrear el schema `public`:

```bash
docker exec -it postgres psql -U devuser -d devdb -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
```

> ⚠️ **Advertencia**: Este comando eliminará **todos** los datos y tablas de la base de datos. Úsalo solo cuando necesites reiniciar el estado de la base de datos.

## Exportar la base de datos (Backup)

Para crear un backup de la base de datos actual, ejecuta el siguiente comando:

```bash
docker exec -it postgres pg_dump -U devuser --no-owner --no-acl devdb > backup.sql
```

Este comando:

- `docker exec -it postgres`: Ejecuta un comando dentro del contenedor de PostgreSQL
- `pg_dump`: Herramienta de PostgreSQL para exportar bases de datos
- `-U devuser`: Usuario de la base de datos
- `--no-owner`: Omite los comandos de propietario (útil para portabilidad)
- `--no-acl`: Omite los permisos de acceso (útil para portabilidad)
- `devdb`: Nombre de la base de datos a exportar
- `> backup.sql`: Guarda el resultado en el archivo `backup.sql` en el directorio actual
