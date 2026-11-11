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
2. **Prohibición de Docker y Procesos**: Ningún comando bajo `internal/cli/commands` puede importar `internal/infra/docker`, `internal/infra/git` ni `os/exec`. `TestCommandsDoNotImportProcessOrInfraAdapters` lo verifica.
3. **Reutilización de Ayudantes de Banderas**: Utilizar siempre los ayudantes definidos en `flags.go` (`addYesFlag`, `addInteractiveFlag`, `addWorkspaceFlags`) para mantener descripciones y nombres de banderas unificados.
4. **Soporte No Interactivo**: Nuevos subcomandos heredan o reciben `--no-interactive` como bandera persistente desde la raíz (no se duplica ni se admite `--non-interactive`). Ante `--no-interactive`, cualquier parámetro obligatorio no provisto debe retornar un error inmediato sin detenerse a consultar. En comandos que efectúan descubrimiento múltiple (como `config skill add <source>`), se exige el uso explícito de `--all` o `--pick <elementos>`.
5. **Composición de Perfiles en `--profile`**: En modo `custom`, la bandera `--profile` acepta listas separadas por comas (`--profile a,b`) con autocompletado mediante `completeCSV`. En `mode=profiles` y en el comando `run`, debe aceptarse estrictamente un único perfil remoto, rechazando cualquier valor CSV con un error explícito y no interactivo.
6. **Escritura de Definiciones Versionadas**: Los comandos nunca escriben ni borran perfiles, definiciones de imagen o habilidades de usuario por su cuenta; delegan en `service.ProfileStore`, `service.ImageService` o `service.ConfigService`, que preparan el repositorio de configuración antes de escribir y registran un commit Conventional por operación (D19).
7. **Invocar un Comando desde Otro**: Cuando un flujo interactivo debe abrir una sesión de otro comando (el menú de acceso de `workspace init`, D23), se construye el comando, se analizan sus banderas con `ParseFlags` y se llama a su `RunE`. Nunca se usa `Execute()`: con argumentos nulos cobra recurre a `os.Args` y la sesión recibiría los argumentos de `init`.
8. **Autocompletado de Nombres**: Toda bandera o argumento que recibe un nombre de imagen o de perfil registra autocompletado (`completeImageArg`, `completeImageNames`, `profileIDs`). Los nombres de imagen se leen de disco con `ImageService.Names`, nunca mediante Docker, y se filtran por origen cuando el comando solo acepta uno (D26).

### Tabla de Banderas y Atajos Canónicos

Las siguientes banderas y reglas de nomenclatura son obligatorias en todos los comandos de la CLI:

| Bandera | Atajo | Semántica y Reglas de Uso |
|---|---|---|
| `--container` | `-c` | Contenedor de destino (siempre registrado mediante el ayudante `addContainerFlag`). |
| `--yes` | `-y` | Confirma una acción destructiva (la ÚNICA bandera canónica de confirmación). |
| `--force` | *(sin atajo)* | Sobrescribe archivos existentes únicamente; nunca debe emplearse como sinónimo de `--yes`. |
| `--no-interactive` | *(sin atajo)* | Registrada una sola vez como bandera persistente en la raíz; nunca duplicada como `--non-interactive`. |
| `--all` | `-a` | Incluye todos los elementos; el atajo `-a` está reservado exclusivamente para `--all`. |
| `--include-active` | *(sin atajo)* | Incluye recursos activos o en ejecución en operaciones de `clean`. |
| `--dry-run` | *(sin atajo)* | Previsualiza operaciones destructivas sin mutar el estado. |
| `--services`, `--skills`, `--modules`, `--ports`, `--volumes`, `--forward-ports` | *(sin atajo)* | Banderas de lista CSV en plural (las formas singulares `--service`, `--skill` y `--with` quedan terminantemente prohibidas). |
| `--x` / `--x=false` | *(según aplique)* | Banderas booleanas canónicas; se prohíben parejas tri-state (`--x` / `--no-x`), excepto `--no-interactive`, `--no-tty` y `--no-up`. |
| `--follow` | `-f` | Atajo `-f` reservado exclusivamente para seguimiento de flujos (ej. logs). |
| `--workdir` | `-w` | Bandera booleana para indicar el directorio de montaje del workspace. |
| `--version` | *(sin atajo)* | Imprime la versión de la CLI (`-v` está reservado exclusivamente para `down -v` / `--volumes`). |

### Jerarquía de Comandos en 6 Dominios y Fachada de Agente

La CLI estructura todas sus operaciones en 6 dominios canónicos orientados a recursos, junto con una fachada `agent` especializada:

1. **`workspace`**: Ciclo de vida completo del devcontainer asociado al proyecto (`init` con wizard, `auto-init` con detección de stack sin prompts, `up`, `down`, `start`, `stop`, `restart`, `destroy`, `compose`, `export`, `import`).
2. **`access`**: Sesiones interactivas y conectividad de red con el contenedor (`shell`, `ssh`, `forward`, `route`, `network`, `copy`, `password`).
3. **`inspect`**: Observabilidad, inspección y telemetría de recursos (`status`, `info`, `logs`, `context`, `ls`).
4. **`profile`**: Gestión, catálogo y autoría de perfiles y plantillas modulares (`list`, `info`, `create`, `copy`, `remove`).
5. **`image`**: Ciclo de vida y registro de imágenes independientes de proyecto (`list`, `info`, `create`, `build`, `pull`, `export`, `remove`, `update`).
6. **`config`**: Parámetros globales y mantenimiento (`registry`, `git-init`, `db-user`, `db-password`, `ssh-key`, `ssh-config-file`, `alias`, `shared`, `skill`, `clean`, `upgrade-cli`).

**Utilidad independiente en la raíz**:
- **`run`**: `devcontainer-cli run` inicia un contenedor desechable directamente a partir de la imagen publicada de un perfil (`--profile`) o de una definición de imagen (`--image`, D27), sin asociarse a ningún workspace de proyecto ni generar archivos en disco.

**Fachada `agent`**:
- Contiene exclusivamente comandos que presentan valores predeterminados y semántica inherentemente desatendida para agentes de IA: `exec`, `ssh`, `forward`, `create`, `run`, `destroy`, `list`, `cli-info`.
- Se prohíbe crear clones 1:1 de comandos humanos en la fachada `agent`.
- Se prohíbe mantener alias retrocompatibles que dupliquen rutas de invocación entre dominios.

---

## 3. Manejo de Errores y Códigos de Salida

| Condición | Mecanismo | Código de Salida |
|---|---|---|
| Éxito | Retorno `nil` de `RunE` | `0` |
| Error de ejecución ordinario | Retorno de `fmt.Errorf(...)` | `1` |
| Cancelación de usuario (Ctrl+C o escape en prompt) | Retorno de `prompt.ErrCancelled` o captura de `SIGINT`/`SIGTERM` | `130` |

### Regla de Oro en Sugerencias de Recuperación (*Recovery Hints*)
Los mensajes de error que instruyen al usuario o al agente sobre qué comando ejecutar a continuación representan la documentación de mayor valor en tiempo de ejecución.
- **Regla Inviolable**: Las sugerencias de recuperación deben nombrar comandos que **no abran prompts interactivos** (ej. sugerir `devcontainer-cli workspace up`, `devcontainer-cli workspace start` o `devcontainer-cli agent create`).
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
