# Plan: autocompletado dinámico para zsh/bash

## Objetivo

Añadir autocompletado de shell a `devcontainer-cli` que complete:

- **Comandos** (`setup-ssh`, `port-forward`, `run`, `down`, ...).
- **Flags** de cada comando (`--variant`, `--with`, `--container`, ...).
- **Valores dinámicos**: contenedores en ejecución, alias SSH, módulos Dockerfile,
  servicios compose, variantes y modos.

## Enfoque: dinámico (delegación al binario)

Patrón estándar de `kubectl`/`gh`/cobra:

1. Un subcomando oculto **`__complete`** recibe la línea de comando actual y devuelve
   los candidatos válidos en esa posición, uno por línea, por `stdout`.
2. Un subcomando **`completion <bash|zsh>`** emite un script delgado que el usuario
   instala una sola vez. Ese script invoca `__complete` en cada `TAB` y pasa los
   candidatos al shell, que filtra por el prefijo ya escrito.

Ventaja: **una sola fuente de verdad en TypeScript**. Cada flag/comando nuevo se refleja
automáticamente; los valores dinámicos (contenedores, alias) se consultan en vivo.

### Protocolo `__complete`

```
devcontainer-cli __complete <palabra1> <palabra2> ... <palabra_actual>
```

- El shell pasa todas las palabras escritas tras el binario, hasta (e incluyendo) la
  palabra bajo el cursor (que puede ser `""`).
- El binario decide los candidatos mirando la palabra **anterior** y el primer token no-flag
  (= el subcomando). **No filtra por prefijo**: devuelve todos los candidatos válidos en la
  posición y deja que el shell filtre (`compgen` en bash, `compadd` en zsh).
- Reglas de decisión (en orden):
  1. Si la palabra anterior es un flag que espera valor → devolver los valores de ese flag.
  2. Si aún no hay subcomando (se está completando el primer token) → devolver nombres de
     comando + flags raíz de generación.
  3. Si hay subcomando → devolver los flags de ese comando + valores posicionales dinámicos.
- **Silencioso ante errores**: todo envuelto en `try/catch`; ante cualquier fallo no imprime
  nada y sale con código 0 (un error en `stdout` corrompería los candidatos).

## Archivos

### Nuevo: `cli/src/completion.ts`

Exporta:

- `completionCandidates(words: string[], providers?: CompletionProviders): string[]`
  — función **pura** (con proveedores inyectables) que implementa la lógica del protocolo.
  Es lo que se testea.
- `runCompleteHidden(argv: string[]): Promise<void>` — envuelve `completionCandidates` en
  `try/catch`, imprime candidatos (uno por línea). Handler de `__complete`.
- `runCompletion(argv: string[]): Promise<void>` — handler de `completion <shell>`; imprime
  `bashCompletionScript()` o `zshCompletionScript()`; error claro si falta/!= bash|zsh.
- `bashCompletionScript(): string` / `zshCompletionScript(): string`.

Proveedores inyectables (por defecto, las funciones reales; en tests, stubs):

```ts
interface CompletionProviders {
  containers: () => string[];   // listManagedContainers().map(c => c.name)
  sshAliases: () => string[];   // getSshAliases()
  modules:    () => string[];   // dockerfileModules.map(m => m.id)
  services:   () => string[];   // composeServices.map(s => s.id)
}
```

Cada proveedor real se envuelve para que **nunca lance** (p. ej. `listManagedContainers`
llama a `ensureDocker` que lanza si Docker no está): `try { ... } catch { return []; }`.

### Modificar: `cli/src/index.ts`

- Registrar en `COMMANDS`:
  ```ts
  completion: runCompletion,
  __complete: runCompleteHidden,
  ```
- `__complete` es oculto (prefijo `__`, no aparece en ayuda ni en la lista de comandos
  completables).

### Modificar: `cli/src/cli.ts`

- Añadir a `helpText()` la línea:
  ```
  cli completion <shell>    Print shell completion script (bash|zsh)
  ```

### Nuevo: `cli/src/__tests__/completion.test.ts`

Tests de `completionCandidates` con proveedores stub (ver sección Testing).

### Modificar (opcional): `cli/install.sh`

Tras instalar, detectar el shell e imprimir el one-liner exacto para activar el completado.
Activación automática (escribir en `fpath`/rc) sólo como opt-in con `INSTALL_COMPLETION=1`
(modificar archivos del usuario sin permiso es invasivo).

## Mapa comando → flags → valores

Fuente de verdad para la metadata estática dentro de `completion.ts`.

| Comando        | Flags                                                                 | Flags con valor dinámico / enumerado |
|----------------|-----------------------------------------------------------------------|--------------------------------------|
| _(raíz/generate)_ | `--mode --variant --registry --with --service --image --workspace --no-interactive --force-prompt --force --build --no-build -v --version -h --help` | `--mode`→BUILD_MODES, `--variant`→REMOTE_VARIANTS, `--with`→módulos, `--service`→servicios |
| `setup-ssh`    | `--remote --alias --key --port --mode --container --service --compose-file -f --user -y --yes -h --help` | `--mode`→`local\|windows\|remote`, `--container`→contenedores, `--alias`→alias SSH, `--service`→servicios |
| `port-forward` | `--alias --service --no-interactive --non-interactive -h --help`      | `--alias`→alias SSH, `--service`→servicios |
| `run`          | `--variant --name --volume --port --registry --no-interactive -h --help` | `--variant`→REMOTE_VARIANTS |
| `down`         | `-v --volumes -y --yes --no-interactive -h --help`                    | —                                    |
| `destroy`      | `-y --yes --no-interactive -h --help`                                 | —                                    |
| `prune`        | `--all -y --yes --no-interactive -h --help`                           | —                                    |
| `start`/`stop`/`restart` | `-h --help`                                                 | —                                    |
| `update`       | `--all --pull --rebuild -h --help`                                    | —                                    |
| `upgrade-cli`  | `-h --help`                                                           | —                                    |
| `cleanup-tips` | `-h --help`                                                           | —                                    |
| `config`       | posicional: `registry`; luego `--unset`                               | 1er posicional → `registry`          |
| `completion`   | posicional: `bash`, `zsh`                                             | 1er posicional → `bash zsh`          |

> Notas:
> - `cleanup-tips` se maneja aparte en `index.ts` (fuera de `COMMANDS`): hay que listarlo
>   manualmente en la lista de comandos completables.
> - Flags de texto libre (`--image --workspace --name --volume --registry --key --port
>   --user --remote --compose-file/-f`) → sin candidatos (devolver `[]`). Mejora futura:
>   sentinela para disparar completado de ficheros del shell.
> - `--with` y `--service` (raíz) aceptan **listas separadas por comas**: si la palabra
>   actual contiene `,`, completar el segmento tras la última coma y reanteponer lo ya escrito.

## Scripts de shell

### bash (`bashCompletionScript()`)

```bash
_devcontainer_cli_complete() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local args=("${COMP_WORDS[@]:1:COMP_CWORD-1}")   # palabras tras el binario, antes del cursor
  local candidates
  candidates="$(devcontainer-cli __complete "${args[@]}" "$cur" 2>/dev/null)"
  COMPREPLY=( $(compgen -W "${candidates}" -- "$cur") )
}
complete -F _devcontainer_cli_complete devcontainer-cli
```

### zsh (`zshCompletionScript()`)

```zsh
#compdef devcontainer-cli
_devcontainer_cli() {
  local current="${words[CURRENT]}"
  local args=("${words[2,CURRENT-1]}")             # words[1] es el binario en zsh
  local -a candidates
  candidates=("${(@f)$(devcontainer-cli __complete "${args[@]}" "$current" 2>/dev/null)}")
  compadd -- "${candidates[@]}"
}
_devcontainer_cli "$@"
```

> Mejora futura (zsh): emitir `valor\tdescripción` por línea y usar `_describe` para mostrar
> descripciones (p. ej. las de `VARIANT_LABELS`).

## Activación (instrucciones a documentar)

- **bash**:
  ```sh
  devcontainer-cli completion bash > ~/.local/share/bash-completion/completions/devcontainer-cli
  # o, en ~/.bashrc:  source <(devcontainer-cli completion bash)
  ```
- **zsh** (archivo en `$fpath`, idiomático):
  ```sh
  mkdir -p ~/.zsh/completions
  devcontainer-cli completion zsh > ~/.zsh/completions/_devcontainer-cli
  # en ~/.zshrc, antes de compinit:
  #   fpath+=(~/.zsh/completions)
  #   autoload -U compinit && compinit
  ```

## Casos borde

- Docker no disponible → proveedores dinámicos devuelven `[]` (sin throw, sin ruido en stderr).
- `__complete` con 0 args → tratar como completar el primer token (comandos + flags raíz).
- Nunca imprimir errores/avisos por `stdout` en `__complete`.
- `__complete` y `completion` excluidos de la lista que se autocompleta como comando
  (`__complete` siempre; `completion` se puede incluir para que `comp<TAB>` funcione — decisión: incluirlo).
- Invocación raíz (sin subcomando reconocido): si el token actual empieza por `-`, ofrecer
  los flags de generación raíz.

## Testing (`completion.test.ts`)

Con proveedores stub (`containers: () => ['app-dev','db-dev']`, etc.):

- `[]` o `['']` → incluye `setup-ssh`, `run`, `down`, ... (lista de comandos).
- `['run','--variant','']` → `REMOTE_VARIANTS`.
- `['--variant','']` (raíz) → `REMOTE_VARIANTS`.
- `['--mode','']` (raíz) → `BUILD_MODES`.
- `['setup-ssh','--mode','']` → `['local','windows','remote']`.
- `['setup-ssh','--container','']` → `['app-dev','db-dev']` (del stub).
- `['port-forward','--alias','']` → alias del stub.
- `['--with','']` → ids de módulos del stub.
- `['--service','']` → ids de servicios del stub.
- `['config','']` → `['registry']`.
- `['completion','']` → `['bash','zsh']`.
- `['down','']` → flags de `down` (`-v`,`--volumes`,`-y`,`--yes`,...).
- Flag de texto libre, p. ej. `['run','--name','']` → `[]`.
- Proveedor que lanza (simula Docker caído) → no propaga; devuelve `[]`.

## Pasos de implementación

1. Crear `cli/src/completion.ts` con metadata, `completionCandidates`, handlers y generadores
   de script.
2. Registrar `completion` y `__complete` en `cli/src/index.ts`.
3. Añadir la línea de `completion` a `helpText()` en `cli/src/cli.ts`.
4. Crear `cli/src/__tests__/completion.test.ts` y pasar `npm test`.
5. `npm run typecheck` + `npm run build`; probar manualmente:
   `source <(node dist/index.js completion bash)` y verificar TAB en bash; equivalente en zsh.
6. (Opcional) Ampliar `install.sh` con la sugerencia de activación.

## Build

No requiere cambios en `build.mjs`/SEA: `completion.ts` se incluye automáticamente al estar
importado desde `index.ts` (esbuild hace bundle desde `src/index.ts`).
