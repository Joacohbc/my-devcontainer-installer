# 📝 TODO: Mejoras para devcontainer-cli

Este documento consolida todas las oportunidades de mejora identificadas en el código, UX, arquitectura y tests. Los elementos están ordenados por prioridad y esfuerzo.

---

## 🔴 Prioridad Alta - Quick Wins (Bajo Esfuerzo)
*Estas tareas aportan mucho valor y son rápidas de implementar.*

### Nuevos Comandos Esenciales
- [x] Implementar `devcontainer-cli shell [-w workspace] [--user root] [-- command args...]`: Atajo para `docker exec -it <container> <shell>`. (Gap de UX masivo). → `shell.go`.
- [x] Implementar `devcontainer-cli logs [-w workspace] [--follow] [--tail 100] [service]`: Wrapper para `docker compose logs`. → `logs.go`.

### Bugs y UX Críticos
- [x] Decidir comportamiento de `down.go --yes`: hoy `--yes` implica borrar volúmenes (`down.go` setea `removeVolumes=true`). Falta decidir si `--yes` debería solo saltar la confirmación sin borrar volúmenes (más seguro, requiere `-v` explícito). → `--yes` ahora solo salta el prompt; los volúmenes se borran únicamente con `-v/--volumes` explícito (`resolveRemoveVolumes`).
- [x] Arreglar `prune.go`: En modo non-interactive sin `--yes`, elimina imágenes huérfanas sin confirmación (peligroso). Debería fallar pidiendo `--yes`. → ya falla con error en `prune.go`.

### Nuevos Módulos (Lenguajes muy demandados)
- [ ] Añadir módulo `rust` (vía `rustup`).
- [ ] Añadir módulo `ruby` (vía `rbenv` o `rvm`).
- [ ] Añadir módulo `deno`.
- [ ] Añadir módulo `php` (vía PPA `ondrej/php`).

### Nuevos Servicios Compose (Bases de datos)
- [ ] Añadir servicio `mysql` / `mariadb`.
- [ ] Añadir health checks a los servicios existentes `mongo` y `redis` (Postgres ya lo tiene). Requiere añadir campo `HealthCheck` a `ServiceDef` (`compose/types.go`).

### Refactoring Rápido
- [x] ~~Eliminar `exec.Command("docker", ...)` en `setup_ssh.go`~~: ya no hay shell-outs a `docker` por `exec.Command`; las llamadas restantes son a `ssh`/`ssh-keygen` (fuera del alcance de `infra/docker`).
- [x] Unificar `contains()` (`port_forward.go`) y `containsString()` (`setup_ssh.go`) usando `slices.Contains`. → ya usan `slices.Contains`/`strings.Contains` de la stdlib.
- [x] Añadir soporte para completion en el shell `fish` en los scripts de instalación. → `install.sh` instala completion `fish` (ps1 no instala completion por diseño).

---

## 🟡 Prioridad Media - Mejoras de UX (Esfuerzo Medio)
*Mejoras significativas en la experiencia de usuario y automatización.*

### Flujo Interactivo (Prompts)
- [ ] Permitir "volver atrás" (Go Back) en el wizard interactivo (`generate_prompts.go`) en caso de que el usuario necesite modificar una selección anterior sin reiniciar todo el proceso.

### Comandos de Visibilidad y Diagnóstico
- [x] Implementar `devcontainer-cli status [--all]`: Muestra una tabla con todos los workspaces, su estado (Running/Stopped), modo e imagen. → `status.go`.
- [ ] Implementar `devcontainer-cli doctor`: Diagnóstico de Docker, daemon, disco, configs SSH y puertos.

### Configuración de Servicios y Entornos
- [x] Permitir versiones configurables en servicios compose (ej. `mongo:7`, `postgres:16`). → `mongo`/`postgres`/`redis` exponen opción de versión.
- [ ] Hacer configurables las credenciales hardcodeadas (`root/root`, `devuser/devpass`).
- [ ] Hacer el puerto SSH `2222` configurable.
- [ ] Modo `--json` para output programático (útil para `status` y `config`).
- [ ] Modo `--quiet` para suprimir output decorativo en scripts.

### Portabilidad y Compartición
- [ ] Implementar export/import de config (`config export > file.yml`, `config import file.yml`).
- [ ] Implementar *Presets* (ej. `--preset fullstack-node` que seleccione node, postgres, redis automáticamente).

### Estilización, Temas y Logging
- [ ] Configurar temas visuales y estilización consistente de la interfaz de la CLI usando [lipgloss](https://github.com/charmbracelet/lipgloss).
- [ ] Implementar la creación de logs estructurados y un sistema de logging avanzado con [log](https://github.com/charmbracelet/log) para facilitar la depuración y el diagnóstico de la CLI.

### Instaladores y Seguridad
- [x] Añadir verificación de checksums (`.sha256`) en los scripts `install.sh` e `install.ps1`. → ambos descargan y verifican `<asset>.sha256` antes de instalar; abortan si falta o no coincide.
- [x] Añadir indicador de progreso durante la descarga de binarios en `upgrade-cli`. → descarga en streaming con `progressWriter` (porcentaje + bytes a stderr).
- [x] Añadir mecanismo de rollback en `upgrade-cli` en caso de fallos. → `swapBinary` restaura el binario original si el reemplazo falla (path Windows con rollback de `<exe>.old`).

---

## 🟣 Prioridad Estratégica (Esfuerzo Alto)
*Cambios profundos que mejorarán la robustez y extensibilidad a largo plazo.*

### Arquitectura de Generación
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
- [x] Tests para `infra/assets/assets.go` (`Preflight`, `ValidateRequiredFiles`, etc.). → `assets_test.go`.
- [x] Tests para `infra/sshdefaults/sshdefaults.go` (lógica de bloques SSH config). → `sshdefaults_test.go`.
- [x] Tests faltantes en `infra/docker/docker.go` (funciones `DockerCapture`, `DockerCompose`). → `docker_test.go` (mock por args para separar la sonda `version`).

### Domain (Edge Cases y Helpers)
- [x] Tests para `domain/config.go` (migraciones y I/O de configs de proyecto). → `config_test.go` (round-trip, migración de modos legacy, JSON inválido).
- [x] Tests para `domain/docker_conflicts.go` (Detección de conflictos, mock de CaptureFunc). → `docker_conflicts_test.go`.
- [x] Tests unitarios directos para los métodos `Render()` de los módulos Dockerfile y Compose (actualmente solo testeados integración-style). → `modules/dockerfile/render_test.go` y `modules/compose/render_test.go`.
- [x] Fuzz tests para validadores. → `validators_fuzz_test.go` (las funciones reales son `SanitizeDockerName`/`IsValidDockerName`/`IsValidImageName`/`IsValidCidr`; `ValidateWorkspaceName`/`ValidateBaseImage` no existen).

### CLI y UI
- [x] Tests para lógica de parsing en `cli/pick/pick.go` (parsePSLines, ContainerWorkspace). → `pick_test.go`.
- [x] Integrar runners Windows/macOS en `.github/workflows/cli-tests.yml`. → job `test` con matriz `ubuntu/windows/macos`; `lint` (gofmt+vet) queda en Linux.
- [x] Evitar que los jobs de variant build en CI comiencen si el base build falla. → `build-variants` con `needs: build-base`.

---

## 🗑️ Limpieza Menor / Code Smells
- [x] Arreglar `update.go`: si se usa `--all` y fallan algunos proyectos, actualmente siempre devuelve exit code 0. Debería fallar si hubo errores. → `updateAll` ahora retorna error si `failCount > 0` (con tests).
- [x] Quitar doble llamada a `CollectRequiredCopyFiles()` en `root.go`. → se calcula una sola vez y se reutiliza.
- [x] Manejar explícitamente los errores ignorados de `os.Getwd()` en toda la aplicación. → helper `currentDir()` que envuelve el error; call sites propagan o degradan (completions).
- [x] Actualizar fallback de Go en `golang_utils.sh` (actualmente anclado a un obsoleto `1.20`). → fallback ahora `go1.25.0`.
- [x] Unificar el idioma de los mensajes en `sshhelp.go` (hay una frase en español colada). → mensaje final ahora en inglés.
