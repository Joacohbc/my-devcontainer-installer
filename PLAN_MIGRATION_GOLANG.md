# Plan de migración: `cli/` (TypeScript/Node) → Go (Cobra)

> Documento de planificación. Define el stack objetivo, la estructura de
> paquetes, las fases de trabajo y los contratos que **no** deben romperse al
> portar el CLI de TypeScript/Node a Go.

---

## 1. Contexto y alcance

El CLI actual (`cli/`) genera `Dockerfile` + `docker-compose` para devcontainers,
orquesta `docker`, gestiona configuración y se auto-actualiza. Está escrito en
TypeScript, corre sobre Node y se distribuye como **binario único Node SEA** con
los scripts `.sh` embebidos.

Magnitud a migrar:

| Métrica | Cantidad |
|---|---|
| Fuente TS (sin tests) | ~6.200 líneas |
| Tests (`node:test`) | ~2.000 líneas |
| Comandos | 15 |
| Módulos Dockerfile | 16 |
| Módulos compose (servicios) | 6 |
| Workflows CI | 3 (`cli-tests`, `cli-release`, `docker-image`) |
| Instaladores | 2 pares (`install`/`uninstall` × `sh`/`ps1`) |
| Scripts de build | 3 (`build.mjs`, `build-sea.mjs`, `build-binary.mjs`) |

### Por qué Go encaja mejor que el stack actual

El núcleo del CLI es: un **generador de texto** (Dockerfile/compose/.env), un
**wrapper de `docker`** (shell-out), un **self-updater** y un **store de
configuración**. Nada de eso está atado a Node, y Go resuelve nativamente cosas
que hoy van a contracorriente:

| Hoy (Node) | En Go | Ganancia |
|---|---|---|
| Node SEA (flag experimental + blob + Asset API) | binario estático nativo + `go:embed` | elimina todo el andamiaje SEA y los 3 `.mjs` |
| Matriz de release + `getTargetTriplet()` + renombrado `.exe.old` | cross-compile `GOOS/GOARCH` (goreleaser) | release mucho más simple; darwin-arm64 nativo (sin Rosetta) |
| `completion.ts` mantenido a mano | **Cobra genera completion** bash/zsh/fish | borra una fuente entera de deuda técnica |
| `chalk` / `enquirer` / `yaml` | `fatih/color` / `huh` / `goccy/go-yaml` | equivalentes maduros |
| arranque del runtime Node | binario nativo | startup más rápido |

---

## 2. Stack objetivo

### Núcleo del CLI (obligatorio)

| Pieza | Elección | En vez de | Razón |
|---|---|---|---|
| Framework de comandos | `spf13/cobra` | — | Estándar de facto; genera completion bash/zsh/fish (elimina `completion.ts`). |
| Prompts interactivos | `charmbracelet/huh` | `promptui` | promptui **no tiene multiselect**, que es el corazón del wizard. huh cubre `select`/`multiselect`/`input`/`confirm` + validación + modo accesible no-TTY. |
| Config (read/write) | `encoding/json` + `adrg/xdg` | `spf13/viper` | `config.json` es **un solo campo** (`registry`) e `images.json` es un **store mutable**, no config en capas. Viper es peso + estado global para nada. `adrg/xdg` resuelve `XDG`/`%APPDATA%` en una línea. |
| Generación YAML | `goccy/go-yaml` | `gopkg.in/yaml.v3` | v3 (`go-yaml/yaml`) está **archivado**. goccy es puro-Go, activo, trae `MapSlice` (orden fácil) y errores con línea/columna. |
| Modelado del compose | structs tipados + `MapSlice` + `Marshaler` custom | mapas sin tipar | Los structs preservan orden (campos por declaración) y dan seguridad de esquema; el `Marshaler` resuelve el `networks` polimórfico (lista U objeto). |
| Embeber assets `.sh` | `go:embed` | Node SEA Asset API | Nativo; sin flags experimentales ni baile del `.old` en Windows. |
| Color / output | `fatih/color` (o `charmbracelet/lipgloss`) | `chalk` | Equivalente directo del helper `ui`. |
| Inyección de versión | `-ldflags "-X main.version=…"` | `esbuild define` | Estándar Go. |
| Release | `goreleaser` | matriz manual + renombrado | Cross-compile + naming + checksums declarativos. |

### Testing

| Pieza | Elección | Notas |
|---|---|---|
| Runner de tests | stdlib `testing` (+ `testify/assert` opcional) | Reemplaza `node:test`. |
| Seam de Docker | interfaz `Runner` sobre `exec.CommandContext` | Equivalente del `spawner` actual; **imprescindible** para testear sin daemon. |
| Validación del YAML generado | `goccy` parse en memoria (string→struct) | El generador es puro string→string; no necesita FS. |
| Seam de filesystem | `spf13/afero` — **opcional** | Solo para testear la *capa de escritura* (`writeOutput`/`maybeOverwrite`) en memoria. Si alcanzan los temp dirs, no se incluye. |
| Validación autoritativa de compose | `compose-spec/compose-go` — **opcional, solo tests** | Carga el compose generado con la lib del propio Docker. Fuerte pero no obligatorio; arrastra yaml archivado transitivamente (aislado a tests). |

### Descartado / no recomendado

| Pieza | Veredicto |
|---|---|
| `spf13/viper` | Sobredimensionado: config de un campo + store mutable. Usar `encoding/json` + `adrg/xdg`. |
| `promptui` | Sin multiselect nativo; poco mantenido; flojo en no-TTY. |
| `gopkg.in/yaml.v3` | Repositorio archivado. |
| Validador de Dockerfile (BuildKit `frontend/dockerfile/parser`) | Existe y es autoritativo, pero arrastra un árbol de dependencias enorme. El Dockerfile es lineal, los fragment-tests lo cubren, y `docker build` (en `local-cached`) es el validador real y gratis. |
| `hadolint` | Es un *linter* de best-practices (binario Haskell externo), no un validador embebible. Otro objetivo. |

### Documentación

| Pieza | Elección | Notas |
|---|---|---|
| Sitio de docs | `withastro/starlight` | Buen DX + i18n de primera (docs en español). Reintroduce toolchain Node, pero **es independiente del binario**. Alternativa on-theme: **Hugo** (binario Go, cero `node_modules`). |

### Resumen en una línea

```
CLI:    cobra + huh + (encoding/json + adrg/xdg) + goccy/go-yaml + go:embed + fatih/color
Test:   testing + Runner interface (docker)  ·  [opcional] afero (FS) · compose-go (compose)
Release: goreleaser  ·  versión por -ldflags
Docs:   starlight   (o hugo si se quiere cero Node)
Fuera:  viper · promptui · yaml.v3 · validador de Dockerfile
```

---

## 3. Estrategia: track paralelo con cutover

No se puede migrar archivo por archivo dentro del mismo binario (lenguajes
distintos). Se construye en un directorio nuevo (`cli-go/`) mientras el CLI TS
sigue shippeando, y se hace el switch cuando haya **paridad de comportamiento
verificada**. Sin big-bang.

### Red de seguridad central — tests de paridad (golden files)

El CLI TS *ya funciona*. Para una matriz de configuraciones representativas:

1. Generar el output con el TS actual (Dockerfile/compose/.env) y congelarlo como **golden**.
2. El Go debe reproducirlo, normalizando solo el reordenamiento esperado de claves YAML.
3. Cualquier divergencia es un test rojo, no una revisión a ojo.

La matriz golden debe cubrir, como mínimo:

- Modo `local-cached` con cada runtime (node, python, go, java-temurin, bun) por separado y combinados.
- Modo `remote` con cada variante de `REMOTE_VARIANTS`.
- Cada servicio compose (postgres, redis, mongo, tunnel) y combinaciones.
- `dockerSocket`: `none` / `socket` / `dind` (incluyendo la auto-provisión del engine y la segmentación de red).
- Config vacía / por defecto.

---

## 4. Estructura de paquetes Go

Mapea 1:1 las capas documentadas en `CLAUDE.md` (responsabilidad, no feature).
`internal/` para que nada sea importable desde afuera del módulo.

```
cli-go/
  go.mod
  cmd/devcontainer-cli/
    main.go              # wiring: handlers de señal (SIGINT/SIGTERM → exit 130), dispatch raíz
  internal/
    core/                # datos puros, sin I/O
      types.go           # DevcontainerConfig, ComposeService, BuildMode, SCHEMA_VERSION,
                         #   REMOTE_VARIANTS, VARIANT_LABELS, BUILD_MODES
      labels.go          # LABEL_* + dockerfileLabelBlock / composeLabels
      registry.go        # catálogo de módulos y servicios (module-registry)
    modules/
      dockerfile/        # un archivo por módulo (base, nodejs, python, go, java-*, bun,
                         #   pnpm, sqlite, dbclients, github-cli, dod, ai-clis, tmux, cleanup, shell-init)
      compose/           # servicios (devcontainer, postgres, redis, mongo, dind-engine, tunnel)
    domain/              # lógica de negocio (decide qué pasa; delega I/O a infra)
      generator.go       # generateDockerfile (null en remote), generateCompose, resolveEnabledServices, fingerprint
      resolver.go        # requires/conflicts
      validators.go      # validación de input
      config.go          # carga/persistencia DevcontainerConfig + shims de compat de mode
      global-config.go   # ~/.config/.../config.json
      image-registry.go  # images.json + computeFingerprint
      subnet.go          # cálculo de subred del engine network
      docker-conflicts.go
      completion.go      # COMMAND_SPECS (lo que Cobra no deriva solo)
    infra/               # única capa que toca el mundo exterior
      docker/            # interfaz Runner sobre exec.CommandContext + ensureDocker/isDockerAvailable/dockerCompose...
      project/           # projectPaths(.dc_<ws>/...), resolveWorkspace
      ui/                # log/ok/warn/success/bar (fatih/color o lipgloss)
      prompt/            # select/multiselect/input/confirm sobre huh + PromptCancelledError
      config/            # helper encoding/json + adrg/xdg
      assets/            # go:embed FS de assets/*.sh + materialización (preflight)
    commands/            # un archivo por comando, cada uno expone su *cobra.Command
  assets/                # los .sh embebidos (entrypoint, install-*, golang_utils, zsh-installer, ...)
  .goreleaser.yaml
```

Convención de rutas de import: nombran la capa, igual que el alias `@/` actual
(`internal/domain/generator`, `internal/infra/docker`).

**Ubicación de tipos:** tipo usado por >1 módulo → `core/types.go`; tipo usado
por un solo comando/módulo → local a ese archivo (ej. `XxxFlags`).

---

## 5. Fases de trabajo

Cada fase indica **qué se porta**, el **entregable** y el **criterio de salida**.

### Fase 0 — Andamiaje

- `go mod init`, layout de carpetas, `cmd/.../main.go` con Cobra root vacío.
- Handlers de señal reproduciendo el contrato de `index.ts` (SIGINT/SIGTERM → restaura stdin → exit 130).
- Elegir/fijar versiones de las libs base.
- CI mínimo: `go test ./...`, `go vet`, `gofmt -l`, `golangci-lint`.
- **Salida:** `go build` produce un binario que imprime el help raíz.

### Fase 1 — Core + dominio puro (corazón, máximo valor)

Parte más valiosa y testeable: **no toca I/O**. Empezar acá.

- `core/types.go`, `core/labels.go`, `core/registry.go`.
- `modules/dockerfile/*` y `modules/compose/*`: un archivo por módulo con sus fragmentos.
- **`ComposeFile` modelado con structs tipados**; `networks` polimórfico con `Marshaler` custom de goccy.
- `domain/generator.go` (fingerprint SHA-256 de Dockerfile normalizado + copyFiles + module IDs ordenados, 12 chars; `generateDockerfile` → `null` en `remote`; auto-provisión dind; segmentación de red).
- `domain/resolver.go`, `validators.go`, `subnet.go`, `completion.go`.
- **Generador puro string→string** → validación sin FS.
- **Tests:** portar `generator.test.ts`, `resolver.test.ts`, `validators.test.ts` + montar **golden de paridad** contra el TS.
- **Salida:** para toda la matriz golden, el Go genera Dockerfile/compose/.env idénticos al TS (modulo orden YAML normalizado).

### Fase 2 — Infra (seams del mundo exterior)

- `infra/docker/`: **interfaz `Runner`** sobre `exec.CommandContext` + `isDockerAvailable`/`ensureDocker` (con cache), `dockerInherit`/`dockerCapture`/`dockerCompose`/`dockerComposeOrThrow`. **Único lugar que ejecuta `docker`.**
- `infra/config/`: `encoding/json` + `adrg/xdg`.
- `domain/global-config.go`, `image-registry.go`, `docker-conflicts.go` (con los shims `custom`/`standalone` → `local-cached`).
- `infra/prompt/`: 4 primitivas sobre huh; `huh.ErrUserAborted` → `PromptCancelledError` → exit 130.
- `infra/project/`, `infra/ui/`.
- **Tests:** mock del `Runner`; **afero opcional** para la capa de escritura.
- **Salida:** capa de I/O completa y mockeable.

### Fase 3 — Comandos (Cobra)

Orden: primero los puros/simples, luego los que tocan Docker.

1. Simples: `config` (+ subcomando `registry`), `cleanup-tips`, `completion` (**Cobra lo genera → borra `completion.ts`**).
2. Docker-wrappers: `down`, `start`/`stop`/`restart`, `prune`, `destroy`, `port-forward`, `run`, `update`, `setup-ssh`.
3. Default `generate` (prompts → flags → validate → write → build/pull) y `upgrade-cli`.
- Respetar `interactive`/`yes`/`no-interactive` y el contrato de errores (devolver `error`, no `os.Exit` en handlers).
- **Resolver la deuda KTD-1..8 de entrada:** todos pasan por `Runner`/`ui`/parser uniforme. No arrastrar inconsistencias.
- **Tests:** parse de flags + help por comando.
- **Salida:** paridad funcional de los 15 comandos.

### Fase 4 — Assets, build y self-update

- **Assets:** `go:embed` sobre `assets/*.sh` → reemplaza `sea-config.json` + `build-sea.mjs` + `build-binary.mjs` + Asset API. `preflight` materializa desde el `embed.FS` (mantener fallback a disco en dev no aplica: en Go el embed siempre está).
- **Versión:** `-ldflags "-X main.version=$VERSION"`.
- **Release:** `goreleaser` — targets `linux-x64/arm64`, `darwin-x64/arm64`, `windows-x64`; checksums; naming del triplet centralizado en config.
- **self-update:** `getTargetTriplet()`, reemplazo en-uso de Windows (`.old`), `cleanupStaleUpdate` en cada arranque. darwin-arm64 nativo (eliminar workaround Rosetta de `install.sh`).
- **Salida:** binarios por target con el naming que esperan los instaladores.

### Fase 5 — Instaladores, CI, docs y cutover

- **Instaladores:** `install.sh`/`install.ps1` + `uninstall.*` bajan el binario raw (formato sin archivar se mantiene). Revisar layout, PATH/rc, completion. **Mantener paridad sh↔ps1.**
- **Workflows:**
  - `cli-tests.yml` → `go test/vet/lint/fmt`.
  - `cli-release.yml` → goreleaser.
  - `docker-image.yml` → sigue invocando el CLI con `--with <module-ids>`; **el contrato de la lista `--with` (un id por módulo) se mantiene**, solo cambia el binario invocado.
- **Docs:** reescribir `CLAUDE.md`/`AGENTS.md`/`GEMINI.md` para Go (`go test`, layout de paquetes, helpers). Sitio **starlight** como track aparte (no bloquea cutover).
- **Cutover:** golden de paridad verdes + 15 comandos en paridad → mover `cli/` a `cli-ts-legacy/` (o borrar) y `cli-go/` → `cli/`. Tag de release con el binario Go.

---

## 6. Mapeo de comandos (TS → Go)

| `argv[0]` | Handler TS | `cobra.Command` Go | Notas de port |
|---|---|---|---|
| _(default)_ | `domain/generate.ts` | `generate` (root run) | Orquesta todo; cubrir gap de tests (`applyFlags`/`writeOutput`/`maybeOverwrite`). |
| `setup-ssh` | `setup-ssh.ts` | `setup-ssh` | **Migrar bien de entrada:** sin `spawnSync` directo (usar `Runner`), sin `log/ok/warn` locales (usar `ui`), sin `deriveWorkspace`/`defaultComposeFile` locales (usar `project`). Resuelve KTD-1/3/4/5. |
| `port-forward` | `port-forward.ts` | `port-forward` | Solo `--no-interactive` (no aceptar `--non-interactive`). |
| `run` | `quick-run.ts` | `run` | `docker run` desde imagen remota. |
| `down` | `down.ts` | `down` | `compose down [-v]`. |
| `destroy` | `destroy.ts` | `destroy` | down + borra `.dc_<ws>/` + config. Confirmación destructiva. |
| `start`/`stop`/`restart` | `lifecycle.ts` | `start`/`stop`/`restart` | Comparten lógica. |
| `prune` | `prune.ts` | `prune` | Borra imágenes huérfanas `devcontainer-cli/*`. |
| `update` | `update-images.ts` | `update` | `--all`; dispatch por modo (`local-cached`→build, `remote`→pull). |
| `upgrade-cli` | `self-update.ts` | `upgrade-cli` | `--check`/`--force`; contrato de naming del release. |
| `config` | `config-cmd.ts` | `config` (+ sub `registry`) | Subcomando como `cobra.Command` hijo. |
| `cleanup-tips` | `cleanup-instructions.ts` | `cleanup-tips` | Resuelve workspace y emite tips. |
| `completion` | `completion-cmd.ts` | `completion` | **Reemplazado por la generación nativa de Cobra.** |

Al registrar cada comando, recordar mantener sincronizado lo que hoy son 5
lugares (registry, dispatch, help raíz, `COMMAND_SPECS`, tests). En Go se reduce:
Cobra centraliza registro + help + completion; solo quedan los tests.

---

## 7. Mapeo de dependencias

| npm | Go | Uso |
|---|---|---|
| `enquirer` | `charmbracelet/huh` | prompts (`infra/prompt`) |
| `yaml` (stringify/parse) | `goccy/go-yaml` | generar/parsear compose |
| `chalk` | `fatih/color` o `charmbracelet/lipgloss` | `infra/ui` |
| `esbuild` (`define`) | `go build -ldflags` | inyección de versión |
| Node SEA Asset API | `go:embed` + `embed.FS` | assets `.sh` |
| `node:test` | `testing` (+ `testify` opcional) | tests |
| `child_process.spawnSync` | `os/exec` (`exec.CommandContext`) detrás de `Runner` | shell-out a docker |
| paths XDG/APPDATA a mano | `adrg/xdg` | dirs de config |
| — (validación tests) | `compose-spec/compose-go` (opcional) | validar compose generado |

---

## 8. Contratos que NO deben romperse

Duplicaciones/contratos que el `CLAUDE.md` marca como frágiles y deben portarse
intactos (y mantenerse sincronizados en el mismo PR que los toque):

1. **Variantes remotas** (`core/types.go` `REMOTE_VARIANTS`/`VARIANT_LABELS` ↔ `.github/workflows/docker-image.yml` matrix). Añadir variante = ambos lados + test en `generator`.
2. **Lista `--with` de la imagen full** (`docker-image.yml` job `build-base`): un id por cada módulo Dockerfile (excepto `base`, `cleanup`, y `java-openjdk` que excluye por conflicto con `java-temurin`). Los módulos compose no van.
3. **Naming del release-asset** (`devcontainer-cli-<triplet>[.exe]`): goreleaser ↔ `self-update.ts` (`getTargetTriplet`) ↔ `install.sh` ↔ `install.ps1`. Cambiar uno = cambiar los cuatro.
4. **Modos de build** (`local-cached`/`remote`): leídos en `generator`, `main`/preflight y `update`. Shim de compat `custom`/`standalone` → `local-cached` al cargar config.
5. **`dockerSocket`** (`none`/`socket`/`dind`): el modo es la única fuente de verdad del engine dind; `docker-dind` es `internal` + auto-provisionado; engine pineado (`docker:NN-dind-rootless`, `privileged`); red dedicada `*-engine-network` solo devcontainer+engine. Nunca auto-montar el socket del host (solo `socket`, con warning).
6. **Fingerprint** (`local-cached`): SHA-256 de Dockerfile normalizado + contenido de copyFiles + IDs de módulos ordenados, primeros 12 chars → `image = devcontainer-cli/<fp12>:latest`. Sale del **Dockerfile**, no del compose → el reordenamiento de claves YAML no afecta el sharing de imágenes.
7. **Paridad de instaladores** sh↔ps1: cualquier cambio en uno se replica en el otro (env overrides, layout, PATH/rc, naming). Diferencias de plataforma esperadas (completion solo en sh, etc.).
8. **Formato de release raw** (un binario por target, sin tar/zip): instaladores + `upgrade-cli` lo asumen.
9. **Contrato de exit codes:** error de usuario → 1; cancelación (`PromptCancelledError`) → 130; SIGINT/SIGTERM → 130. Handlers devuelven `error`, nunca `os.Exit` directo.
10. **Regla "módulo cambia → test":** todo módulo añadido/modificado tiene test espejo. CI bloquea merge.

---

## 9. Estrategia de testing detallada

- **Unit (paridad de patrones actuales):** parse de flags + help por comando; registry resoluble; output generado contiene los fragmentos esperados; requires/conflicts; validators válido/inválido.
- **Golden de paridad (la red central):** matriz de configs → output congelado del TS → el Go debe igualarlo. Normalizar solo orden de claves YAML.
- **Mock de Docker:** sustituir el `Runner` por uno fake que devuelve `{status, stdout, stderr}` predefinidos (equivalente a `spawner.spawnSync = ...`). Restaurar siempre.
- **I/O en memoria (opcional):** afero `NewMemMapFs` para `writeOutput`/`maybeOverwrite`/creación de `.dc_<ws>/`, cubriendo el gap actual de `generate.ts`.
- **Validación de output:** parse del string generado con goccy (bien formado); opcional `compose-go` (esquema compose autoritativo) en un test dedicado.
- **CI:** `go test ./...` + `go vet` + `gofmt -l` (debe estar vacío) + `golangci-lint`.

---

## 10. Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Regresión silenciosa en el output generado | **Golden de paridad** contra el TS. |
| Reordenamiento de claves YAML | Structs tipados (orden por declaración) + normalizar en el diff. Fingerprint sale del Dockerfile → no rompe sharing. |
| UX de prompts distinto (huh vs enquirer) | PoC temprana del `generate` interactivo antes de comprometer las fases. |
| Contratos duplicados (triplet, `--with`, variantes) | Centralizar naming en goreleaser; tests que asserten constantes. |
| Pérdida de cobertura | Portar la regla "módulo → test"; CI bloquea merge. |
| Dependencia pesada por validación | No incluir BuildKit; compose-go solo opcional en tests. |

---

## 11. Esfuerzo relativo

- **Fase 1** es la más pesada (lógica de generación + golden). El grueso del valor (correctitud) queda fijado al cerrarla.
- **Fases 0 y 4** son chicas.
- **Fases 2, 3 y 5** medianas.

El orden está pensado para que la correctitud del generador (lo más crítico) se
valide antes de invertir en distribución y comandos.

---

## 12. Checklist de cutover

- [ ] Golden de paridad verdes para toda la matriz.
- [ ] 15 comandos con paridad funcional + tests de flags/help.
- [ ] Assets embebidos y materializados correctamente desde el binario.
- [ ] `goreleaser` produce los 5 targets con el naming correcto.
- [ ] `upgrade-cli` actualiza desde un release real (incl. Windows `.old`).
- [ ] Instaladores sh/ps1 actualizados y en paridad.
- [ ] Workflows migrados (`cli-tests`, `cli-release`, `docker-image`).
- [ ] `CLAUDE.md`/`AGENTS.md`/`GEMINI.md` reescritos para Go.
- [ ] `cli/` (TS) movido a legacy o eliminado; `cli-go/` → `cli/`.
