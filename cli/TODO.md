# 📝 TODO: Mejoras para devcontainer-cli

Este documento consolida todas las oportunidades de mejora identificadas en el código, UX, arquitectura y tests. Los elementos están ordenados por prioridad y esfuerzo.

---

## 🔴 Prioridad Alta - Quick Wins (Bajo Esfuerzo)
*Estas tareas aportan mucho valor y son rápidas de implementar.*

### Estrucutrar

- ui y prompts: Solo deben encargarse de la interacción con el usuario, no de la lógica de negocio.
- commands: Solo deben encargarse de parsear argumentos y llamar a los servicios correspondientes, no de la lógica de negocio y deben usar ui/prompts.
- services: Solo deben encargarse de la lógica de negocio. Tampoco de hacer prints o interactuar con el usuario. (solo tirar errores o devolver resultados
o usar el EventBus para comunicar inputs/resultados a la UI)
- infra/domain/type: Solo deben encargarse de la definición de tipos, no de lógica de negocio completa, gestion de comandos (docker,ssh), servicios, generator. De bajo nivel, no de proceso enteros. Los proceso de comandos enteros los hace Service
- modules: Solo para los módulos de generación de archivos, no para lógica de negocio, gestión de comandos o servicios.


### Categorias
- IA Tools (solo un grupo, no software en si)
- Lenguajes (agrupando software de un mismo lenguaje, ej: node, python, etc)
- Bases de datos (agrupando software de un mismo tipo, ej: mongo, postgres, redis, etc)
- Dev Tools (agrupando software de desarrollo, ej: gh, etc)
- Clientes agrupando software cliente, ej: redis-cli, pgcli, etc

### Nuevos Servicios Compose (Bases de datos)
- [ ] Añadir health checks a los servicios existentes `mongo` y `redis` (Postgres ya lo tiene). Requiere añadir campo `HealthCheck` a `ServiceDef` (`compose/types.go`).

---

## 🟡 Prioridad Media - Mejoras de UX (Esfuerzo Medio)
*Mejoras significativas en la experiencia de usuario y automatización.*

### Comandos de Visibilidad y Diagnóstico
- [ ] Implementar `devcontainer-cli doctor`: Diagnóstico de Docker, daemon, disco, configs SSH y puertos.
