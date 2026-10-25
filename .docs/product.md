# Visión y Modelo de Producto

`devcontainer-cli` es una herramienta CLI en Go diseñada para crear, gestionar y operar entornos de desarrollo reproducibles, aislados y desechables en Docker. Su propósito fundamental es eliminar la fricción de configuración de herramientas en la máquina anfitriona (*host*) y ofrecer tanto a desarrolladores humanos como a agentes de inteligencia artificial un entorno seguro y predecible.

---

## 1. Actores y Personas del Sistema

### Actor 1: Desarrollador Humano
- **Objetivo**: Iniciar proyectos rápidamente en múltiples stacks (Node.js, Go, Python, Java, Rust) sin instalar compiladores, runtimes o bases de datos en su sistema operativo.
- **Canal de Interacción**: Asistente interactivo en terminal (`generate`), comandos de ciclo de vida (`up`, `down`, `start`, `stop`, `restart`), conexión SSH nativa para VS Code Remote o Neovim, y multiplexación de terminal con Zellij.
- **Expectativa**: Cero conflictos de dependencias entre proyectos, conservación de configuraciones personales (alias, claves SSH, tokens de Git/IA) y mínima latencia al iniciar contenedores mediante imágenes precompiladas o caché de capas inteligente.

### Actor 2: Agente Autónomo de Inteligencia Artificial (Antigravity, Claude Code, Codex, Copilot)
- **Objetivo**: Ejecutar compilaciones, pruebas, formateo y diagnósticos de forma desatendida dentro del contenedor, sin tocar el sistema de archivos del anfitrión ni arriesgar la estabilidad del host.
- **Canal de Interacción**: Fachada `devcontainer-cli agent` (`agent create`, `agent exec`, `agent forward`, `agent context`, `agent clean`) y lectura del inventario dinámico en `~/CONTEXT.md` / `get-devcontainer-context --json`.
- **Expectativa**: Comportamiento determinista, salida limpia, ejecución obligatoria como usuario regular `devuser` (para no dejar archivos propiedad de `root` en el workspace), fallos inmediatos y explícitos en modo no interactivo (sin bloqueos por confirmaciones pendientes), e instrucciones precisas de recuperación.

---

## 2. Flujos Funcionales de Extremo a Extremo

### Flujo 1: Generación y Configuración del Entorno

1. **Detección o Selección de Stack**:
   - Vía interactiva: el usuario ejecuta `devcontainer-cli` y navega el asistente para elegir módulos del Dockerfile, servicios de base de datos (PostgreSQL, Redis, MongoDB), puertos y habilidades de agente.
   - Vía plantilla/perfil: selección de perfiles predefinidos (`nodejs`, `python`, `go`, `bun`, etc.) o perfiles personalizados en `~/.devcontainer-cli/profiles/`.
   - Vía automática: `devcontainer-cli init` inspecciona los archivos del proyecto (ej. `package.json`, `go.mod`, `requirements.txt`, `pom.xml`) para seleccionar los módulos requeridos automáticamente.
2. **Materialización del Directorio `.dc_<workspace>/`**:
   - Se crea el directorio de configuración del proyecto con `docker-compose.yml`, `Dockerfile` (en modo `custom`), `.env` y el directorio `build/` con los scripts necesarios.
3. **Cálculo de Huella Digital y Caché**:
   - En modo `custom`, la CLI calcula el hash SHA-256 de las capas resultantes. Si ya existe una imagen local con dicha huella, se omite el proceso de compilación (`docker build`) y se reutiliza al instante.
   - En modo `profiles`, se omite la compilación local y se descarga directamente la imagen publicada en GitHub Container Registry.

### Flujo 2: Conexión y Operación Diaria

1. **Acceso por Shell Directo**:
   - `devcontainer-cli shell`: invoca `docker exec` asignando un pseudo-terminal TTY y Zsh con temas y alias preconfigurados.
   - `devcontainer-cli shell -T -- <comando>`: ejecución no interactiva para scripts y pipelines.
2. **Acceso por SSH Nativo**:
   - `devcontainer-cli ssh`: gestiona el ciclo de vida de claves y alias en `~/.ssh/devcontainer-cli.config` y abre una sesión OpenSSH contra el contenedor en el puerto 2222.
   - Ideal para editores remotos (VS Code Remote - SSH, JetBrains Gateway, editores en terminal).
3. **Reenvío de Puertos y Túneles**:
   - `devcontainer-cli port-forward`: expone puertos del contenedor en el loopback privado del host (`127.0.0.1`), o crea túneles inversos (`reverse:PORT`) para que el contenedor consuma servicios que residen en la máquina física.

### Flujo 3: Aprovisionamiento y Colaboración de Agentes de IA

1. **Sembrado de Credenciales (`config shared sync`)**:
   - Las sesiones de GitHub CLI, Claude Code, Antigravity y OpenAI autenticadas en el host se sincronizan en el volumen persistente `devcontainer-shared-config`.
2. **Aislamiento de Sesiones por Proyecto**:
   - Cada proyecto monta su propio workspace en `/workspaces/<workspace>`. Los agentes inician sus sesiones en este directorio, impidiendo que el historial de chat o contexto de un repositorio se filtre a otro.
3. **Orientación Autocontenida**:
   - Al iniciar sesión, el agente lee `~/CONTEXT.md`, generado automáticamente para describir con precisión exacta las herramientas y credenciales disponibles en ese contenedor específico.

### Flujo 4: Limpieza y Eliminación Segura

1. **Detención**: `devcontainer-cli down` detiene y remueve los contenedores y redes del proyecto actual.
2. **Destrucción Completa**: `devcontainer-cli destroy` detiene el stack, remueve volúmenes asociados, elimina el bloque de alias en la configuración SSH de la CLI, desasigna claves en `known_hosts` y borra el directorio `.dc_<workspace>/`.
3. **Recolección de Basura Global**: `devcontainer-cli clean` ofrece subcomandos especializados para limpiar imágenes huérfanas, contenedores detenidos, redes en desuso y bloques SSH obsoletos.
