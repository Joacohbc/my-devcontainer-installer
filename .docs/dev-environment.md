# Entorno de Desarrollo, CI y Publicación

Este documento detalla el toolchain local, los flujos de integración continua en GitHub Actions, el proceso de lanzamiento con GoReleaser y el mecanismo de auto-actualización del binario.

---

## 1. Toolchain y Comandos de Desarrollo

El código fuente de la CLI reside exclusivamente en el directorio `cli/` y utiliza **Go 1.26**.

Todos los comandos de desarrollo deben ejecutarse desde la carpeta `cli/`:

| Tarea | Comando | Invariante / Verificación |
|---|---|---|
| **Compilar** | `go build ./...` | Compilación sin advertencias de todos los paquetes. |
| **Ejecutar Pruebas** | `go test ./...` | Ejecución de la suite completa de pruebas unitarias y de integración en memoria. |
| **Análisis Estático** | `go vet ./...` | Detección de errores y construcciones sospechosas; debe salir limpio. |
| **Verificar Formato** | `gofmt -l .` | Debe imprimir una salida completamente vacía. Un archivo no formateado rompe CI. |
| **Corregir Formato** | `gofmt -w .` | Aplica el formato estándar de Go a todos los archivos modificados. |
| **Ejecutar en Desarrollo** | `go run ./cmd/devcontainer-cli [args]` | Ejecuta el comando deseado directamente con el código actual. |
| **Verificación de Release** | `goreleaser check` | Valida la configuración del manifiesto de GoReleaser. |
| **Simulación de Release** | `goreleaser release --snapshot --clean` | Genera los binarios de snapshot locales sin publicar artefactos. |

Si Go no está instalado en el anfitrión, estos comandos se ejecutan dentro del devcontainer del proyecto con `devcontainer-cli agent exec -w -- sh -c 'cd cli && …'`. Si ese contenedor está detenido, `agent exec` falla: primero se inicia con `devcontainer-cli workspace up`.

---

## 2. Flujos de Trabajo en GitHub Actions (`.github/workflows/`)

### 1. `cli-tests.yml`
- **Disparador**: Cada `push` o `pull_request` que afecte archivos bajo `cli/**`.
- **Trabajos**: Ejecuta de forma estricta `gofmt`, `go vet`, `go build` y `go test`. Cualquier fallo en una de las cuatro etapas bloquea la integración.

### 2. `docker-image.yml`
- **Disparador**: Cambios en definiciones de módulos Dockerfile o imágenes base.
- **Estructura en Cascada**:
  1. `build-utils`: Publica la imagen utilitaria ligera (`images/utils/Dockerfile`: Node.js, Git y utilidades de shell) para tareas auxiliares de la CLI.
  2. `build-base-cache`: Genera la imagen del perfil `base` (capas comunes + GitHub CLI) y la publica como `devcontainer-base:latest`, con caché de capas en el registro (`devcontainer-cache:base`, `mode=max`).
  3. `build-single`: Matriz que compila las variantes remotas de un solo lenguaje consumiendo la caché de `devcontainer-base` (`cache-from`).
  4. `build-combo`: Matriz que compila las variantes remotas combinadas; depende de `build-single`.

  Entre ambas matrices cubren `types.RemoteVariants`, salvo `base`, que ya publica `build-base-cache`. No existe una imagen completa con todos los módulos: la antigua variante `ssh` (`devcontainer-ssh:latest`) se retiró y la CLI la rechaza como perfil desconocido.

### 3. `skills-refs.yml`
- **Disparador**: Modificaciones en el catálogo de habilidades `internal/domain/modules/skills/skills.yml`.
- **Propósito**: Valida que todas las referencias Git y repositorios de habilidades sean accesibles y sintácticamente válidos.

### 4. `cli-release.yml`
- **Disparador**: Evento de publicación de una Release en GitHub (`release: published`).
- **Propósito**: Ejecuta `goreleaser release` para compilar los binarios de producción, calcular los hashes SHA-256 y adjuntarlos a la versión publicada.

---

## 3. Versionado, GoReleaser y Artefactos de Lanzamiento

### Nombres Normalizados de Artefactos (Tripletes)
Los ejecutables se distribuyen como binarios individuales sin empaquetado zip ni tar:
`devcontainer-cli-<triplet>` acompañado de su suma de verificación `devcontainer-cli-<triplet>.sha256`.

Los tripletes soportados son:
- `linux-x64`
- `linux-arm64`
- `darwin-x64`
- `darwin-arm64`

### Inyección de Versión en Compilación
La versión se inyecta en `cmd/devcontainer-cli/main.go` mediante banderas del enlazador:
`-ldflags "-X main.version={{ .Version }}"`

Si el binario se compila localmente sin inyección, el valor por defecto de `version` es `dev`.

### Mecanismo de Auto-Actualización (`devcontainer-cli config upgrade-cli`)
- Consulta la API de lanzamientos de GitHub (`/repos/<owner>/<repo>/releases`), respetando `GITHUB_TOKEN` para evitar límites de tasa.
- Compara versiones semánticas y selecciona la versión estable más reciente (o pre-lanzamientos si se pasa `--pre-release`).
- Descarga el binario correspondiente al triplete de la máquina actual y su hash `.sha256`.
- Valida la integridad mediante comparación en tiempo constante (`subtle.ConstantTimeCompare`).
- Reemplaza el binario en ejecución mediante una operación atómica de renombramiento en el sistema de archivos.

---

## 4. Scripts de Instalación para el Usuario

Ubicados en `cli/install.sh` y `cli/uninstall.sh`:
- Ruta de instalación estándar: `$HOME/.local/share/devcontainer-cli/`
- Enlace en PATH: `$HOME/.local/bin/devcontainer-cli`
- Autocompletado: Generación automática de scripts de autocompletado para `zsh` y `bash` en los archivos de inicio de sesión del usuario.
