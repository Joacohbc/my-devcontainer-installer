# Utilidades en Contenedor

Guía práctica de utilidades incluidas en las imágenes de `devcontainer-cli` para mantener procesos interactivos persistentes dentro del contenedor.

---

## Procesos Interactivos y TUI en Segundo Plano con Zellij

Las aplicaciones interactivas de terminal (TUI), CLIs de agentes o procesos de larga duración requieren una pseudo-terminal (PTY) real para funcionar. Si se lanzan con `nohup ... < /dev/null &`, fallan de inmediato por ausencia de TTY; si se ejecutan en una shell normal, se cierran al ejecutar `exit`.

Como **Zellij** viene preinstalado en todas las imágenes (`zellij` es un módulo siempre activo), permite crear sesiones desacopladas con una PTY persistente, independientes de tu sesión de shell y del proceso PID 1 (`sshd`) del contenedor.

### Crear y lanzar un proceso en segundo plano

Desde el directorio del workspace dentro del contenedor:

```bash
# 1. Crear una sesión de Zellij desacoplada en segundo plano
zellij attach --create-background <sesion>

# 2. Ejecutar tu comando dentro de esa sesión
zellij --session <sesion> run -- <comando>
```

También puedes lanzarlo directamente desde el host sin abrir una shell interactiva:

```bash
devcontainer-cli agent exec -w -- zellij attach --create-background <sesion>
devcontainer-cli agent exec -w -- zellij --session <sesion> run -- <comando>
```

### Verificar, reconectarse y salir

```bash
# Listar sesiones activas de Zellij
zellij list-sessions

# Volver a conectarse a la interfaz en vivo
zellij attach <sesion>
# (o desde el host)
devcontainer-cli access shell -- zellij attach <sesion>
```

- **Desacoplar sin cerrar el proceso**: presiona `Ctrl+O` y luego `D`.
- **Salir del contenedor**: escribe `exit`; la sesión de Zellij y el proceso seguirán activos en segundo plano mientras el contenedor esté en ejecución.
