# Manual Práctico de Casos de Uso

Este manual proporciona una guía directa, exhaustiva y orientada a la práctica para operar `devcontainer-cli`. Está dividido en dos secciones complementarias: **Desarrolladores Humanos** (flujos interactivos, editores locales, bases de datos y redes) y **Agentes Autónomos de IA** (ejecución desatendida, aislamiento de procesos, auto-descubrimiento y entornos temporales).

---

## Filosofía Operativa

`devcontainer-cli` desacopla el código fuente de las herramientas requeridas para compilarlo y ejecutarlo. El principio rector es:
1. **La máquina física (*host*) permanece inmaculada**: no se instalan runtimes, compiladores ni bases de datos en el sistema operativo local.
2. **El contenedor es desechable y reproducible**: cualquier miembro del equipo o agente de IA puede recrear el entorno exacto en segundos.
3. **El workspace es soberano**: el repositorio se monta en `/workspaces/<workspace>`, garantizando que el trabajo persista tras reiniciar o destruir el contenedor.

---

# PARTE I: Casos de Uso para Desarrolladores Humanos

---

### Caso 1.1: Inicio Interactivo de un Nuevo Proyecto desde Cero

**Escenario**: Necesita comenzar un proyecto nuevo (ej. backend en Node.js con TypeScript, pnpm y utilidades de Git) sin configurar herramientas locales.

**Paso a paso**:
1. Cree y entre al directorio de su proyecto:
   ```bash
   mkdir api-pagos && cd api-pagos
   ```
2. Ejecute el asistente interactivo:
   ```bash
   devcontainer-cli
   ```
3. El asistente TUI (`charmbracelet/huh`) le guiará a través de las opciones:
   - **Modo**: Seleccione `custom` (para personalizar módulos a medida).
   - **Módulos Dockerfile**: Marque `nodejs`, `pnpm` y `github-cli`.
   - **Servicios Compose**: Seleccione ninguno o añada una base de datos si la requiere.
   - **Habilidades de Agente**: Responda `no` si solo trabajará usted.
4. Una vez completado, la CLI generará la carpeta oculta `.dc_api-pagos/` con el Dockerfile y compose.
5. Inicie el entorno:
   ```bash
   devcontainer-cli up
   ```
6. Conéctese a la terminal interactiva:
   ```bash
   devcontainer-cli shell
   ```

---

### Caso 1.2: Arranque Instantáneo con Perfiles Precompilados (`--profile`)

**Escenario**: Desea iniciar un entorno para Python o Go de inmediato sin esperar a que Docker compile capas localmente.

**Paso a paso**:
1. Ubíquese en el directorio de su proyecto:
   ```bash
   cd mi-servicio-python
   ```
2. Genere el entorno apuntando al perfil precompilado de GitHub Container Registry:
   ```bash
   devcontainer-cli --profile python
   ```
   *(Variantes disponibles: `nodejs`, `bun`, `python`, `go`, `java-temurin`, `node-go`, `node-python`, `node-java-temurin`, `bun-go`, `bun-python`, `bun-java-temurin`)*.
3. Inicie el contenedor (descargará la imagen precompilada en segundos sin compilar Dockerfile):
   ```bash
   devcontainer-cli up
   ```
4. Acceda por SSH o shell:
   ```bash
   devcontainer-cli ssh
   ```

---

### Caso 1.3: Desarrollo Fullstack con Bases de Datos (`postgres`, `redis`, `mongo`)

**Escenario**: Su aplicación requiere PostgreSQL y Redis locales para desarrollo. Desea que los datos persistan y poder consultar la base de datos tanto desde el contenedor como desde el anfitrión con herramientas gráficas (DBeaver, TablePlus).

**Paso a paso**:
1. Genere el devcontainer incluyendo los servicios deseados y los clientes de consola:
   ```bash
   devcontainer-cli --with nodejs,pnpm,dbclients --service postgres,redis
   devcontainer-cli up
   ```
2. **Conexión desde el código dentro del contenedor**:
   - Host de PostgreSQL: `postgres` (resuelto internamente por la red privada de Docker, **nunca** `localhost`).
   - Puerto: `5432` | Usuario: `devuser` | Contraseña: `devpass` | Base de datos: `devdb`.
   - Host de Redis: `redis` | Puerto: `6379`.
3. **Consulta interactiva desde la terminal del contenedor**:
   ```bash
   devcontainer-cli shell -- psql -h postgres -U devuser -d devdb
   devcontainer-cli shell -- redis-cli -h redis ping
   ```
4. **Conexión desde el Host (DBeaver, TablePlus, Navicat)**:
   - Dado que los puertos están publicados en el host, configure la conexión en su cliente local apuntando a:
     - Host: `127.0.0.1` | Puerto: `5432` | Usuario: `devuser` | Contraseña: `devpass`.
5. **Persistencia de Datos**:
   - Los datos se almacenan en volúmenes Docker independientes (`<workspace>-postgres-data` y `<workspace>-redis-data`). Un `devcontainer-cli down` o reinicio conserva toda su información intacta.

---

### Caso 1.4: Acceso al Contenedor: SSH Persistente, SSH Efímero y Shell Directa

**Escenario**: Desea conectarse al contenedor para desarrollar interactivamente con VS Code Remote, ejecutar comandos rápidos desde la terminal o trabajar con multiplexación de sesiones vía Zellij.

#### Comparativa de Métodos de Acceso

| Método | Comando | Mecanismo | Modifica `~/.ssh/` en Host | Caso de Uso Ideal |
|---|---|---|---|---|
| **SSH Persistente** | `devcontainer-cli ssh` | OpenSSH (puerto 2222) | **Sí** (registra bloque `Host` en `devcontainer-cli.config`) | Desarrollo interactivo diario, editores gráficos (VS Code Remote - SSH), alias corto `ssh <workspace>`. |
| **SSH Efímero** | `devcontainer-cli ssh --ephemeral` | OpenSSH (puerto 2222) | **No** (conecta directo a IP sin dejar rastro) | Tareas puntuales por SSH, scripts, automatizaciones o cuando no desea alterar su configuración SSH local. |
| **Shell Directa** | `devcontainer-cli shell` | `docker exec` | **No** (interactúa vía socket de Docker) | Entrada rápida a la terminal, pipes y redirecciones limpias (`-T`), acceso a BD auxiliares (`-c`). |

---

#### 1. Modo SSH Persistente y VS Code Remote - SSH
1. Ejecute la conexión por primera vez:
   ```bash
   devcontainer-cli ssh
   ```
   *La CLI genera automáticamente las claves SSH en `~/.config/devcontainer-cli/ssh/`, inyecta la clave pública en el contenedor y crea el bloque de conexión permanente en `~/.ssh/devcontainer-cli.config`.*
2. **Conexión desde VS Code**:
   - Abra VS Code en su máquina anfitriona.
   - Presione `F1` (o `Cmd+Shift+P` / `Ctrl+Shift+P`) y elija: **Remote-SSH: Connect to Host...**.
   - Seleccione el alias con el nombre de su proyecto (ej. `mi-proyecto`).
   - VS Code abrirá una ventana remota ejecutándose dentro del contenedor, con extensiones y terminal integradas.
3. **Conexión directa desde cualquier terminal**:
   ```bash
   ssh mi-proyecto
   ```

---

#### 2. Modo SSH Efímero (`--ephemeral`)
Conéctese directamente sin tocar su archivo de configuración SSH ni registrar alias:
```bash
devcontainer-cli ssh --ephemeral
```
*La CLI resuelve la IP del contenedor y utiliza la clave privada gestionada (`-i ~/.config/devcontainer-cli/ssh/id_devcontainer`) para abrir la sesión OpenSSH en el puerto 2222, garantizando **cero polución** en sus archivos de configuración locales.*

---

#### 3. Modo Shell Directa (`devcontainer-cli shell`)
Entrada inmediata sin necesidad de claves SSH ni daemon sshd:
```bash
# Terminal interactiva Zsh
devcontainer-cli shell

# Terminal Bash explícita
devcontainer-cli shell -- bash

# Acceso a contenedores secundarios del stack (ej. Postgres)
devcontainer-cli shell -c mi-proyecto-postgres

# Comandos no interactivos o pipes limpios (sin caracteres de control TTY)
devcontainer-cli shell -T -- uname -a
```

---

#### 4. Multiplexación de Terminal con Zellij
Si prefiere trabajar 100% en consola, inicie una sesión multiplexada dentro del contenedor:
```bash
devcontainer-cli shell -- zellij
```
*Puede dividir paneles, crear pestañas y desacoplarse (`Ctrl+o`, luego `d`) sin perder los procesos en ejecución.*

---

### Caso 1.5: Reenvío de Puertos y Túneles Inversos (`port-forward`)

**Escenario A (Forward directo)**: Desea probar una aplicación web iniciada dentro del contenedor en el puerto `3000` navegando desde el browser de su máquina anfitriona.
```bash
# Escucha en 127.0.0.1:3000 de su máquina física y reenvía hacia el puerto 3000 del contenedor
devcontainer-cli port-forward 3000
```

**Escenario B (Túnel Inverso - Host hacia Contenedor)**: Su contenedor necesita consumir un servicio que ya está corriendo en su máquina anfitriona (por ejemplo, una base de datos local en el host en el puerto `5432` o un mock API en el puerto `8080`).
```bash
# Abre el puerto 5432 en el loopback del contenedor y reenvía las peticiones hacia su máquina física
devcontainer-cli port-forward reverse:5432
```
*Dentro del contenedor, su aplicación podrá conectarse directamente a `localhost:5432` para alcanzar el servicio de su host.*

---

### Caso 1.6: Desarrollo Remoto en Servidores LAN (`--via`)

**Escenario**: El código y Docker residen en un servidor local (Raspberry Pi, Intel NUC, Home Server) accesible por SSH (`usuario@servidor-lan`), y usted desea programar desde su laptop sin instalar `devcontainer-cli` en el servidor.

**Paso a paso**:
1. Desde su laptop, conéctese al contenedor remoto a través del salto SSH:
   ```bash
   devcontainer-cli ssh --via usuario@servidor-lan --container <workspace>-devcontainer-ssh
   ```
2. La CLI utilizará la conexión SSH existente hacia el servidor como túnel de control (`DOCKER_HOST=ssh://...`), configurará la clave pública en el contenedor remoto y registrará el alias de conexión local.
3. Su clave privada **nunca abandona su laptop**.
4. Conéctese directamente en cualquier momento con:
   ```bash
   ssh <workspace>
   ```

---

### Caso 1.7: Acceso Privado Global con Cloudflare Zero Trust (WARP)

**Escenario**: Desea programar desde una cafetería o viaje conectándose a su devcontainer hogareño/oficina sin abrir puertos en el router ni configurar DDNS.

**Paso a paso**:
1. Genere el devcontainer con el módulo `cloudflared`:
   ```bash
   devcontainer-cli --with cloudflared
   devcontainer-cli up
   ```
2. Inicie el túnel dentro del contenedor con su token de conector obtenido en el panel de Cloudflare Zero Trust:
   ```bash
   devcontainer-cli shell -- cloudflared tunnel run --token <SU_TOKEN>
   ```
3. En el panel de Cloudflare Zero Trust (**Networks** -> **Tunnels** -> **Private Network**), agregue la subred que figura en su `.env` (ej. `172.25.0.0/28`).
4. Encienda la aplicación Cloudflare WARP en su laptop o móvil.
5. Conéctese por SSH directamente a la IP fija del contenedor (`DEVCONTAINER_IP` del `.env`):
   ```bash
   ssh devuser@172.25.0.14
   ```
*(Consulte [`cloudflare-tunnel.md`](cloudflare-tunnel.md) para detalles avanzados).*

---

### Caso 1.8: Personalización de Alias y Credenciales Compartidas

**Escenario**: Desea que todos sus contenedores compartan sus inicios de sesión (GitHub CLI, Claude Code, OpenAI) y sus alias de terminal preferidos sin tener que reconfigurarlos en cada proyecto.

**Paso a paso**:
1. **Sembrar inicios de sesión desde su máquina anfitriona**:
   ```bash
   devcontainer-cli config shared sync
   ```
   *Copia de forma segura las sesiones de `~/.config/gh`, `~/.claude`, etc., hacia el volumen persistente `devcontainer-shared-config`. Todo contenedor nuevo iniciará con sus herramientas ya autenticadas.*
2. **Definir alias de terminal globales**:
   ```bash
   devcontainer-cli config alias set gs "git status -sb"
   devcontainer-cli config alias set dcu "devcontainer-cli up"
   devcontainer-cli config alias sync
   ```
   *Los alias se aplican inmediatamente en la próxima shell de todos sus contenedores sin necesidad de recompilar imágenes.*

---

### Caso 1.9: Higiene y Limpieza del Sistema (`clean` / `destroy`)

**Escenario**: Ha terminado un proyecto o desea liberar espacio en disco eliminando recursos Docker huérfanos.

**Paso a paso**:
1. **Destruir el proyecto actual**:
   ```bash
   devcontainer-cli destroy
   ```
   *Detiene el stack, remueve volúmenes asociados, borra la carpeta `.dc_<workspace>/`, limpia el bloque en `~/.ssh/devcontainer-cli.config` y olvida la clave de host en `known_hosts`.*
2. **Limpiar bloques SSH obsoletos de proyectos ya eliminados**:
   ```bash
   devcontainer-cli clean ssh -y
   ```
3. **Limpieza integral del sistema Docker gestionado**:
   ```bash
   devcontainer-cli clean all
   ```

---

### Caso 1.10: Composición de Perfiles (Lenguaje Base + Perfil Personal de Herramientas)

**Escenario**: Dispone de un perfil personal con herramientas y scripts propios (por ejemplo, `my-ai-tools` con utilidades de SQLite, scripts de inicialización y habilidades de agente) y desea reutilizarlo sobre cualquier perfil de lenguaje (`python`, `nodejs` o `go`) sin duplicar definiciones de perfiles.

**Paso a paso**:
1. Cree su perfil personal de complementos en `~/.devcontainer-cli/profiles/my-ai-tools/`:
   - `profile.yml`:
     ```yaml
     id: my-ai-tools
     label: Mis Herramientas de IA
     modules:
       - sqlite
     scripts:
       - file: setup-tools.sh
         when: build
     skills:
       - agent-browser
     ports:
       - "8080:80"
     ```
   - Coloque el script `setup-tools.sh` en el mismo directorio.
2. Combine el perfil del lenguaje con su perfil de complementos pasando una lista separada por comas:
   ```bash
   devcontainer-cli agent create --profile python,my-ai-tools
   ```
   O mediante el comando directo:
   ```bash
   devcontainer-cli --profile python,my-ai-tools
   ```
3. El entorno resultante fusiona automáticamente:
   - **Módulos**: `github-cli`, `python`, `sqlite` (orden de primera aparición, sin duplicados).
   - **Scripts**: Ejecuta el script `setup-tools.sh` en la fase de construcción.
   - **Habilidades y Puertos**: Registra la habilidad `agent-browser` y publica el puerto `8080:80`.

---

# PARTE II: Casos de Uso para Agentes Autónomos de IA

---

### Invariantes Críticos para Agentes
Los agentes de IA que operan en proyectos bajo `devcontainer-cli` deben observar estas cuatro reglas fundamentales:
1. **Nunca tocar Docker directamente**: Queda terminantemente prohibido invocar `docker`, `docker compose` o `docker exec` desnudos. Toda interacción se realiza a través de `devcontainer-cli agent <verbo>`.
2. **Ejecución obligatoria como usuario regular (`devuser`)**: La fachada `agent exec` fuerza la ejecución como `devuser`, evitando que se creen archivos propiedad de `root` en el checkout del host.
3. **Cero prompts interactivos**: Los comandos del grupo `agent` tienen desactivada la interacción. Si falta un parámetro, fallan ruidosamente de inmediato en lugar de bloquear el proceso.
4. **Modo Efímero por Defecto en SSH y Puertos**: Las operaciones `agent ssh` y `agent forward` activan `--ephemeral=true` por defecto, conectando directamente por IP y clave privada sin crear bloques `Host` ni modificar `~/.ssh/config` ni `~/.ssh/devcontainer-cli.config`, garantizando cero polución en el host.

---

### Caso 2.1: Aprovisionamiento Autónomo y Desatendido (`agent create`)

**Escenario**: El agente necesita preparar un entorno completo para un proyecto sin intervención humana.

**Comando**:
```bash
devcontainer-cli agent create --profile node-python --service postgres --build
```

**Comportamiento**:
- Genera la configuración `.dc_<workspace>/`.
- Resuelve dependencias y compila o descarga la imagen sin solicitar confirmaciones.
- Levanta el stack completo (`docker compose up`).
- Si la operación tiene éxito, el entorno queda listo para ejecutar comandos inmediatamente.

---

### Caso 2.2: Orientación y Auto-Inspección de Contexto (`agent context`)

**Escenario**: Antes de ejecutar compilaciones, pruebas o migraciones, el agente debe saber con precisión matemática qué versiones de herramientas, rutas y credenciales existen en el contenedor.

**Paso a paso**:
1. El agente consulta el contexto dinámico en formato JSON:
   ```bash
   devcontainer-cli agent context --json
   ```
2. La salida entrega un inventario estructurado:
   - Versión exacta de Node.js, Python, Go, etc.
   - Rutas activas en `PATH`.
   - Servicios de base de datos alcanzables y sus puertos internos.
   - Workspace montado.
3. El agente lee el documento de orientación en el contenedor:
   ```bash
   devcontainer-cli agent exec -w -- cat ~/CONTEXT.md
   ```

---

### Caso 2.3: Ejecución de Compilaciones y Pruebas (`agent exec`)

**Escenario**: El agente debe ejecutar la suite de pruebas unitarias o de integración del proyecto de forma aislada.

**Comando estándar**:
```bash
devcontainer-cli agent exec -w -- pnpm test
```
*El parámetro `-w` sitúa la ejecución automáticamente en el directorio de trabajo del workspace (`/workspaces/<workspace>`).*

**Comandos encadenados o con variables de entorno**:
```bash
devcontainer-cli agent exec -w -- bash -lc 'uv pip install -e . && pytest -v'
```

**Ventajas operativas**:
- Si el comando falla, el código de salida devuelto refleja exactamente el estado de la prueba.
- Los archivos generados (como reportes de cobertura o binarios) quedan con permisos del usuario anfitrión.

---

### Caso 2.4: Aprovisionamiento de Habilidades con Alcance de Proyecto (`agent skill install`)

**Escenario**: El agente requiere herramientas o guías especializadas provistas por el ecosistema de Skills (ej. `playwright`, `security-auditor`), pero no debe contaminar el volumen compartido del usuario.

**Comando**:
```bash
devcontainer-cli agent skill install webapp-testing
```

**Mecanismo**:
- La CLI levanta un contenedor efímero con montaje del workspace.
- Ejecuta `npx skills add` dentro del directorio del repositorio.
- Las habilidades quedan registradas exclusivamente para este proyecto (`<workspace>/.agents/skills/`), sin alterar otros repositorios.

---

### Caso 2.5: Sesión SSH y Reenvío de Puertos Efímeros (`agent ssh` / `agent forward`)

**Escenario**: El agente requiere abrir una sesión SSH desatendida para herramientas que dependen de SSH, o necesita exponer un puerto web (ej. `8080`) para que una herramienta externa en el host (Playwright, Chrome DevTools) interactúe con la aplicación.

**Comandos**:
```bash
# Sesión SSH desatendida efímera (cero polución en ~/.ssh/config)
devcontainer-cli agent ssh

# Reenvío de puerto directo a 127.0.0.1:8080 en el host
devcontainer-cli agent forward 8080

# Túnel inverso efímero: permite al contenedor acceder al puerto 5432 del host
devcontainer-cli agent forward reverse:5432
```

**Propiedades Clave**:
- `--ephemeral=true` por defecto: la conexión se realiza directamente a la IP del contenedor con la clave privada, sin tocar `~/.ssh/config` ni `~/.ssh/devcontainer-cli.config`.
- Al finalizar el proceso del agente, no queda ningún alias o clave huérfana en la máquina del usuario.
- El reenvío a `127.0.0.1` asegura la navegación en un *Secure Context* local sin problemas de APIs Web bloqueadas.

---

### Caso 2.6: Entornos Desechables y Temporales (`--temporal`)

**Escenario**: El agente necesita clonar una librería externa, reproducir un fallo o ejecutar un script potencialmente riesgoso sin afectar ningún proyecto del usuario.

**Comando**:
```bash
devcontainer-cli agent create --temporal --profile python
```

**Comportamiento**:
- Se crea un directorio temporal aislado.
- Se levanta el contenedor con un tiempo de vida efímero.
- Al concluir la prueba, se invoca:
  ```bash
  devcontainer-cli agent clean
  ```
  *Todo rastro del entorno temporal, contenedores e imágenes huérfanas se elimina por completo.*

---

### Caso 2.7: Limpieza y Cierre de Tarea (`agent clean`)

**Escenario**: Al concluir un pipeline o sprint de trabajo autónomo, el agente debe liberar memoria RAM y almacenamiento.

**Comando**:
```bash
devcontainer-cli agent clean
```
- Detiene los contenedores del proyecto.
- Elimina la red y volúmenes temporales.
- Elimina la imagen Docker local específica construida para este proyecto si ya no se requiere.
