# 📝 TODO: Mejoras para devcontainer-cli

Este documento consolida todas las oportunidades de mejora identificadas en el código, UX, arquitectura y tests. Los elementos están ordenados por prioridad y esfuerzo.

---

## 🔴 Prioridad Alta - Quick Wins (Bajo Esfuerzo)
*Estas tareas aportan mucho valor y son rápidas de implementar.*

### Nuevos Comandos Esenciales
- [ ] Implementar `devcontainer-cli shell [-w workspace] [--user root] [-- command args...]`: Atajo para `docker exec -it <container> <shell>`. (Gap de UX masivo).
- [ ] Implementar `devcontainer-cli logs [-w workspace] [--follow] [--tail 100] [service]`: Wrapper para `docker compose logs`.

### Bugs y UX Críticos
- [ ] Arreglar `down.go`: El flag `--yes` no confirma la eliminación de volúmenes, solo salta la pregunta.
- [ ] Arreglar `prune.go`: En modo non-interactive sin `--yes`, elimina imágenes huérfanas sin confirmación (peligroso). Debería fallar pidiendo `--yes`.

### Nuevos Módulos (Lenguajes muy demandados)
- [ ] Añadir módulo `rust` (vía `rustup`).
- [ ] Añadir módulo `ruby` (vía `rbenv` o `rvm`).
- [ ] Añadir módulo `deno`.
- [ ] Añadir módulo `php` (vía PPA `ondrej/php`).

### Nuevos Servicios Compose (Bases de datos)
- [ ] Añadir servicio `mysql` / `mariadb`.
- [ ] Añadir health checks a los servicios existentes `mongo` y `redis` (Postgres ya lo tiene).

### Refactoring Rápido
- [ ] Eliminar `exec.Command("docker", ...)` en `setup_ssh.go` y usar la abstracción `infra/docker`.
- [ ] Unificar `contains()` (`port_forward.go`) y `containsString()` (`setup_ssh.go`) usando `slices.Contains`.
- [ ] Añadir soporte para completion en el shell `fish` en los scripts de instalación.

---

## 🟡 Prioridad Media - Mejoras de UX (Esfuerzo Medio)
*Mejoras significativas en la experiencia de usuario y automatización.*

### Flujo Interactivo (Prompts)
- [ ] Permitir "volver atrás" (Go Back) en el wizard interactivo (`generate_prompts.go`) en caso de que el usuario necesite modificar una selección anterior sin reiniciar todo el proceso.

### Comandos de Visibilidad y Diagnóstico
- [ ] Implementar `devcontainer-cli status [--all]`: Muestra una tabla con todos los workspaces, su estado (Running/Stopped), modo e imagen.
- [ ] Implementar `devcontainer-cli doctor`: Diagnóstico de Docker, daemon, disco, configs SSH y puertos.

### Configuración de Servicios y Entornos
- [ ] Permitir versiones configurables en servicios compose (ej. `mongo:7`, `postgres:16`).
- [ ] Hacer configurables las credenciales hardcodeadas (`root/root`, `devuser/devpass`).
- [ ] Hacer el puerto SSH `2222` configurable.
- [ ] Modo `--json` para output programático (útil para `status` y `config`).
- [ ] Modo `--quiet` para suprimir output decorativo en scripts.

### Portabilidad y Compartición
- [ ] Implementar export/import de config (`config export > file.yml`, `config import file.yml`).
- [ ] Implementar *Presets* (ej. `--preset fullstack-node` que seleccione node, postgres, redis automáticamente).

### Instaladores y Seguridad
- [ ] Añadir verificación de checksums (`.sha256`) en los scripts `install.sh` e `install.ps1`.
- [ ] Añadir indicador de progreso durante la descarga de binarios en `upgrade-cli`.
- [ ] Añadir mecanismo de rollback en `upgrade-cli` en caso de fallos.

---

## 🟣 Prioridad Estratégica (Esfuerzo Alto)
*Cambios profundos que mejorarán la robustez y extensibilidad a largo plazo.*

### Arquitectura de Generación
- [ ] **Reemplazar generación YAML manual**: Cambiar la concatenación de strings en los métodos `Render()` de compose por el uso de `gopkg.in/yaml.v3`. Elimina bugs de indentación.
- [ ] **Contexto en Docker**: Propagar `context.Context` a través de `infra/docker` para permitir cancelación (Ctrl+C limpio) y timeouts.
- [ ] **Fingerprint Completo**: El fingerprint actual solo mira el Dockerfile. Debería incluir también la configuración de Compose para recrear contenedores si cambian los servicios.

### Ecosistema y Performance
- [ ] **Compatibilidad devcontainer.json**: Permitir leer/escribir el estándar de VS Code para interoperabilidad.
- [ ] **Sistema de Plugins**: Permitir a los usuarios definir módulos/servicios custom sin modificar el binario del CLI.
- [ ] **Caché BuildKit**: Usar `--mount=type=cache,target=/var/cache/apt` en módulos base para acelerar builds iterativos.

---

## 🧪 Deuda Técnica: Testing (Para Desarrolladores)

Actualmente hay buena cobertura en `domain`, pero áreas clave están en blanco:

### Infraestructura (Crítico)
- [ ] Tests para `infra/assets/assets.go` (`Preflight`, `ValidateRequiredFiles`, etc.).
- [ ] Tests para `infra/sshdefaults/sshdefaults.go` (lógica de bloques SSH config).
- [ ] Tests faltantes en `infra/docker/docker.go` (funciones `DockerCapture`, `DockerCompose`).

### Domain (Edge Cases y Helpers)
- [ ] Tests para `domain/config.go` (migraciones y I/O de configs de proyecto).
- [ ] Tests para `domain/docker_conflicts.go` (Detección de conflictos, mock de CaptureFunc).
- [ ] Tests unitarios directos para los métodos `Render()` de los módulos Dockerfile y Compose (actualmente solo testeados integración-style).
- [ ] Fuzz tests para validadores (`ValidateWorkspaceName`, `ValidateBaseImage`).

### CLI y UI
- [ ] Tests para lógica de parsing en `cli/pick/pick.go` (parsePSLines, ContainerWorkspace).
- [ ] Integrar runners Windows/macOS en `.github/workflows/cli-tests.yml`.
- [ ] Evitar que los jobs de variant build en CI comiencen si el base build falla.

---

## 🗑️ Limpieza Menor / Code Smells
- [ ] Arreglar `update.go`: si se usa `--all` y fallan algunos proyectos, actualmente siempre devuelve exit code 0. Debería fallar si hubo errores.
- [ ] Quitar doble llamada a `CollectRequiredCopyFiles()` en `root.go`.
- [ ] Manejar explícitamente los errores ignorados de `os.Getwd()` en toda la aplicación.
- [ ] Actualizar fallback de Go en `golang_utils.sh` (actualmente anclado a un obsoleto `1.20`).
- [ ] Unificar el idioma de los mensajes en `sshhelp.go` (hay una frase en español colada).
