# Wayfinder Map: Refactorización de Skills & Profiles a YAML y Categorías Funcionales

## Destination
Una arquitectura modular y basada en datos donde:
1. Todos los metadatos de las skills viven en un manifest declarativo embebido (`skills.yml`).
2. El código Go en `modules/skills/` contiene únicamente tipos, identificadores y lógica pura de resolución (~90 líneas vs 550+ anteriores).
3. Las skills se agrupan estrictamente en 6 Categorías funcionales según `code-design` (`core-agents`, `frontend-design`, `architecture-planning`, `quality-testing`, `docs-content`, `automation-n8n`).
4. Se eliminan por completo las suites monolíticas de repositorios (`mattpocock-skills`, `anthropic-skills`, `n8n-all`).
5. Los perfiles estándar (`plainBuiltinProfiles`) no instalan skills por defecto (las skills se seleccionan a demanda).

## Notes
- **Reglas Principales:** Obedecido `AGENTS.md` (Go toolchain, capas `cli -> service -> {domain, infra}`, tests con 100% de éxito).
- **Diseño de Código:** Cumplidas las directivas de `code-design` (código plano, sin comentarios redundantes, tipado auto-documentado).

## Decisions so far
- [01-skills-yaml-schema.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/01-skills-yaml-schema.md) — Definido el esquema `skills.yml` con categories y skills tipadas.
- [02-functional-categories-taxonomy.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/02-functional-categories-taxonomy.md) — Mapeadas todas las ~80 skills a las 6 categorías funcionales.
- [03-go-domain-skills-loader.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/03-go-domain-skills-loader.md) — `skills.go` refactorizado a ~90 líneas con deserialización segura en `init()`.
- [04-profiles-and-categories-integration.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/04-profiles-and-categories-integration.md) — `plainBuiltinProfiles` limpios sin skills por defecto y `ExpandSkillGroups` actualizado.
- [05-test-suite-and-verification.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/05-test-suite-and-verification.md) — Suite completa de pruebas unitarias ejecutada con éxito al 100%.

## Open Tickets (Frontier)
*(Todos los tickets han sido completados y resueltos exitosamente)*

## Out of scope
- Mantener compatibilidad con nombres de suites por autor (`mattpocock-skills`, etc.), ya que se decidió unificar bajo categorías funcionales.
