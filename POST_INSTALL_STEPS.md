# Post Installation Steps

## Obtener la contraseña del DevUser

```bash
docker compose logs devcontainer-ssh
# >>> devuser password: <password>
```

## Conectarse

```bash
ssh devuser@localhost -p 2222
```

## Cambiar la contraseña (Opcional)

```bash
sudo passwd
```

## Loguearse con Github

```bash
chmod +x /workspace/login-github-cli.sh
/workspace/login-github-cli.sh
```

## Instalar Firebase Tools (Opcional)

```bash
pnpm install -g firebase-tools
```

```bash
firebase login
```

## Instalar Gemini CLI y Jules CLI (Opcional)

### 1. Instalar Gemini CLI

Gemini CLI te permite usar los modelos de IA de Google directamente desde tu terminal para tareas de codificación, refactorización y chat.

Comando de instalación global:

```bash
pnpm install -g @google/gemini-cli
```

Cómo iniciar: Una vez instalado, simplemente ejecuta:

```bash
gemini
```

(La primera vez te pedirá autenticarte con tu cuenta de Google).

### 2. Instalar Jules CLI

Jules es un "agente asíncrono" (sidekick) que trabaja en segundo plano (generalmente gestionando Pull Requests o tareas largas) y se integra con Gemini CLI.

Comando de instalación global:

```bash
pnpm install -g @google/jules
```

Cómo iniciar: Para autenticarte y vincularlo, usa:

```bash
jules login
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
