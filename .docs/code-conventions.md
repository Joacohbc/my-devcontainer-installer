# Convenciones de Código y Diseño

Este documento establece las directivas de desarrollo, patrones arquitectónicos y estándares de calidad para el código de `devcontainer-cli`.

---

## 1. Directivas de Diseño de Código (`hbc-code-design`)

### Código Auto-Documentado (Cero Comentarios Explicativos)
- El código debe comunicar su intención mediante nombres expresivos de funciones, tipos y variables.
- Quedan prohibidos los comentarios redundantes que describen lo que el código hace línea por línea.
- Solo se admiten comentarios para documentar razones no evidentes impuestas por sistemas externos (ej. comportamiento peculiar de una llamada al kernel, orden de capas en Dockerfile para preservar caché, o peculiaridades de PEP 668).

### Flujo de Control Plano (Cláusulas de Guarda)
- Máximo 2 niveles de anidamiento dentro de cualquier bloque de función.
- Aplicar cláusulas de guarda (*guard clauses*) inmediatamente al inicio de funciones y métodos para validar precondiciones, manejando errores temprano y manteniendo el camino feliz (*happy path*) sin sangrado excesivo.

### Inyección de Dependencias Mediante Interfaces
- Todo servicio que requiera interacción con el usuario o salida a terminal debe recibir interfaces `service.Reporter` y `service.Prompter` como campos de su estructura.
- Las funciones de dominio que consultan Docker deben recibir `domain.CaptureFunc`.
- La capa de infraestructura Docker desacopla la ejecución de procesos mediante la interfaz `docker.Runner`, permitiendo sustitución en pruebas (`docker.SetRunner(fake)`).

### Unidades Explícitas en Identificadores
- Los identificadores que representan duraciones, bytes o límites numéricos deben incluir la unidad en el nombre: `timeoutMs`, `maxSizeBytes`, `retryCount`.

---

## 2. Patrón de Autoría de Comandos Cobra

Cada comando se define en un archivo dedicado dentro de `internal/cli/commands/<nombre>.go` y se auto-registra en tiempo de inicio:

```go
func init() { register(newDownCommand()) }

func newDownCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:          "down",
        Short:        "Stop and remove containers for the current project",
        SilenceUsage: true,
        RunE:         runDown,
    }
    addYesFlag(cmd)          // -y/--yes para confirmaciones destructivas
    addInteractiveFlag(cmd)  // --no-interactive
    return cmd
}

func runDown(cmd *cobra.Command, _ []string) error {
    svc := service.LifecycleService{Report: ui.Console{}}
    return svc.Down(composeFile, workspace, removeVolumes)
}
```

### Reglas Estrictas para Comandos
1. **Delegación Total al Servicio**: `RunE` solo procesa argumentos, resuelve banderas de Cobra, instancia el servicio con `ui.Console{}` e invoca el método correspondiente.
2. **Prohibición de Docker y Procesos**: Ningún comando bajo `internal/cli/commands` puede importar `internal/infra/docker` ni `os/exec`.
3. **Reutilización de Ayudantes de Banderas**: Utilizar siempre los ayudantes definidos en `flags.go` (`addYesFlag`, `addInteractiveFlag`, `addWorkspaceFlags`) para mantener descripciones y nombres de banderas unificados.
4. **Soporte No Interactivo**: Nuevos subcomandos deben aceptar `--no-interactive` (no `--non-interactive`; la raíz acepta ambos por retrocompatibilidad). Ante `--no-interactive`, cualquier parámetro obligatorio no provisto debe retornar un error inmediato sin detenerse a consultar.

---

## 3. Manejo de Errores y Códigos de Salida

| Condición | Mecanismo | Código de Salida |
|---|---|---|
| Éxito | Retorno `nil` de `RunE` | `0` |
| Error de ejecución ordinario | Retorno de `fmt.Errorf(...)` | `1` |
| Cancelación de usuario (Ctrl+C o escape en prompt) | Retorno de `prompt.ErrCancelled` o captura de `SIGINT`/`SIGTERM` | `130` |

### Regla de Oro en Sugerencias de Recuperación (*Recovery Hints*)
Los mensajes de error que instruyen al usuario o al agente sobre qué comando ejecutar a continuación representan la documentación de mayor valor en tiempo de ejecución.
- **Regla Inviolable**: Las sugerencias de recuperación deben nombrar comandos que **no abran prompts interactivos** (ej. sugerir `devcontainer-cli up`, `devcontainer-cli start` o `devcontainer-cli agent create`).
- **Prohibido**: Sugerir el comando raíz interactivo `devcontainer-cli`, ya que provocaría que un agente o script desatendido se bloquee indefinidamente sin capacidad de respuesta.
- La suite de pruebas `TestErrorMessagesDoNotPointAtTheWizard` (`commands/recovery_hints_test.go`) valida automáticamente que ninguna cadena de error sugiera la invocación del wizard interactivo desnudo.

---

## 4. Matriz de Lenguajes del Repositorio

| Entorno / Archivo | Idioma Obligatorio | Detalle y Restricciones |
|---|---|---|
| Código fuente Go, identificadores, tipos, paquetes | **Inglés** | Nombres idiomáticos, limpios y descriptivos. |
| Commits de Git | **Inglés** | Formato Conventional Commits (`feat`, `fix`, `refactor`, `test`, `docs`, `chore`) explicando el motivo (*why*). |
| `AGENTS.md` (Constitución raíz) | **Inglés** | Directo, conciso, enfocado en reglas e invariantes del sistema. |
| Habilidades de Agente (`.agents/skills/*/SKILL.md`) | **Inglés** | Definiciones de acciones, triggers e instrucciones de paso a paso. |
| Subagentes (`.agents/agents/*.md`) | **Inglés** | Nombre, descripción, herramientas asignadas y formatos de salida. |
| Documentación técnica (`.docs/*.md`, `README.md`) | **Español Neutro Estándar** | Sin regionalismos ni voseo (prohibido *tenés, podés, hacé, fijate, borrá, agregá, acá*). |
| Interacción y conversación con el usuario | **Español Neutro Estándar** | Respuestas claras, directas y profesionales. |
