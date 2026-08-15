# Ticket 04: Integración con Profiles y Perfiles Planos Limpios

**Label:** `wayfinder:task`  
**Status:** Blocked by [03-go-domain-skills-loader.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/03-go-domain-skills-loader.md)

## Decisiones Confirmadas
1. **Perfiles Planos (`plainBuiltinProfiles`):** `defaultPlainProfileSkills = nil`. Los perfiles base (`base`, `go`, `python`, `nodejs`, `bun`, `java-temurin`, etc.) no instalan ninguna skill por defecto. Traen únicamente sus herramientas de desarrollo.
2. **Selección a Demanda:** Las skills y categorías se agregan únicamente si el usuario las especifica explícitamente en el wizard interactivo o mediante flags (`--skill frontend-design`, `--skill architecture-planning`, etc.).
3. **Perfiles Especializados (`profiles/*.yml`):**
   - El perfil `scraper` mantiene sus skills requeridas: `[firecrawl, agent-browser, webapp-testing, graphify]`.
   - El perfil `n8n` incluye la categoría `automation-n8n`.

## Propuesta Técnica
- En `profiles.go`: eliminar `defaultPlainProfileSkills` y dejar `Skills: nil` en `plainBuiltinProfiles`.
- En `catalog.go`: actualizar `ExpandSkillGroups` para expandir las 6 categorías funcionales de `skills.Categories`.
