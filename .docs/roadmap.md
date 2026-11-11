# Hoja de Ruta y Tareas Pendientes (Roadmap)

Este documento es la **única lista viva de tareas** del proyecto. Describe exclusivamente el trabajo pendiente, las decisiones de diseño abiertas y la deuda técnica identificada. Cuando un elemento se completa en el código, debe eliminarse de este archivo en el mismo commit o pull request.

---

## 1. Decisiones Abiertas

Actualmente no hay decisiones abiertas que bloqueen las funcionalidades en producción.

### Decisiones Técnicas Resueltas
- **Desestimación de Alpine Linux para la Imagen Base**: Tras evaluar la reducción de tamaño frente al impacto en compatibilidad de desarrollo, se desestima el reemplazo de Ubuntu 24.04 por Alpine Linux debido a:
  1. Incompatibilidad de `musl libc` con binarios precompilados de asistentes de IA y el runtime `bun`.
  2. Ausencia de soporte de wheels binarios `manylinux` en Python, lo que forzaría la compilación local de paquetes y degradaría el tiempo de inicio.
  3. Necesidad de duplicar el mantenimiento de scripts de instalación de paquetes (`apk` frente a `apt`).
  La imagen base se mantiene estandarizada en Ubuntu 24.04 LTS para garantizar máxima estabilidad y paridad de herramientas.

---

## 2. Visión

- **Soporte Nativo de Windows (WSL2 / Hyper-V Engine)**: Evaluar la paridad de scripts de shell y montajes de volumen para desarrollo nativo en estaciones Windows.
- **Perfiles Especializados de Nuevos Ecosistemas**: Diseñar perfiles para desarrollo en C/C++ (Clang, CMake, Ninja), Rust avanzado con compilación cruzada y stacks de IA local (Ollama / HuggingFace Transformers en contenedor).
- **Mejora en Ergonomía de Daemons Remotos**: Simplificar el aprovisionamiento de devcontainers en hosts Docker remotos mediante túneles SSH automáticos y gestión de contexto Docker sin requerir configuración manual.

---

## 3. Configuración y Parámetros Candidatos

- Soporte para variables de entorno encriptadas mediante age o sops antes de inyectarlas en el `.env` generado.
- Parámetro para limitar el uso de memoria RAM y núcleos de CPU en `devcontainer.config.json` para stacks de bases de datos locales.

---

## 4. Backlog de Funcionalidades

- [ ] Soporte para perfiles remotos con etiquetas semánticas de versión además de `:latest` (ej. `:v1.2.0`).
- [ ] Exportación directa de configuración de VS Code (`.vscode/settings.json` y `.devcontainer/devcontainer.json`) para compatibilidad bidireccional con extensiones oficiales de Microsoft.
- [ ] Comando de telemetría de rendimiento opcional para medir tiempos de arranque de contenedores y descargas de capas.

---

## 5. Deuda Técnica

- [ ] Monitorear y podar la carpeta `cli-ts-legacy/` cuando el periodo de referencia histórico concluya definitivamente.
- [ ] Unificar los ayudantes de parseo de puertos en un único paquete interno compartido entre `domain/portspec` y `service/portforward`.
