# Utilidades y Flujos en Contenedor

Guía práctica de utilidades incluidas en las imágenes de `devcontainer-cli` para operar procesos persistentes, sesiones interactivas y tareas en segundo plano dentro del contenedor.

---

## 1. Procesos Interactivos y TUI en Segundo Plano con Zellij

Las herramientas interactivas de terminal (TUI), como `claude --remote-control <nombre>`, requieren una pseudo-terminal (PTY) real para funcionar. Si se lanzan con `nohup ... < /dev/null &`, fallan de inmediato por ausencia de TTY; si se ejecutan en una shell normal, se cierran al ejecutar `exit`.

Como **Zellij** viene preinstalado en todas las imágenes (`zellij` es un módulo siempre activo), puede usarse para crear sesiones desacopladas con una PTY persistente, independientes de su sesión de shell y del proceso PID 1 (`sshd`) del contenedor.

### Crear y lanzar un proceso en segundo plano

Desde el directorio del workspace dentro del contenedor (o directamente desde el host usando `devcontainer-cli agent exec -w`):

```bash
# 1. Crear una sesión de Zellij desacoplada en segundo plano
zellij attach --create-background claude

# 2. Ejecutar el comando dentro de esa sesión
zellij --session claude run -- claude --remote-control claude-docker-16gb
```

También puede lanzarse directamente desde el host sin abrir una shell interactiva:

```bash
devcontainer-cli agent exec -w -- zellij attach --create-background claude
devcontainer-cli agent exec -w -- zellij --session claude run -- claude --remote-control claude-docker-16gb
```

### Verificar, reconectarse y salir

```bash
# Listar sesiones activas de Zellij
zellij list-sessions

# Volver a conectarse a la interfaz en vivo
zellij attach claude
# (o desde el host)
devcontainer-cli access shell -- zellij attach claude
```

- **Desacoplar sin cerrar el proceso**: presione `Ctrl+O` y luego `D`.
- **Salir del contenedor**: escriba `exit`; la sesión de Zellij y el proceso seguirán activos en segundo plano mientras el contenedor esté en ejecución.

---

## 2. Procesos No Interactivos en Segundo Plano (`nohup` + `disown`)

Para comandos que **no** requieren una interfaz TUI ni PTY (por ejemplo, servidores HTTP o workers simples), basta con redirigir entrada/salida y desvincular el trabajo de la shell:

```bash
nohup mi-comando > /tmp/mi-comando.log 2>&1 < /dev/null &
disown
exit
```

Para inspeccionar la salida posteriormente:

```bash
tail -f /tmp/mi-comando.log
```
