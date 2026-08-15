# Ticket 05: Suite de Pruebas y Verificación Integral

**Label:** `wayfinder:task`  
**Status:** Blocked by [04-profiles-and-categories-integration.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/04-profiles-and-categories-integration.md)

## Question
¿Qué pruebas unitarias y de integración son necesarias para asegurar cero regresiones y verificar la robustez del manifest YAML y las categorías?

## Plan de Pruebas

1. **`skills_test.go`:**
   - **`TestSkillsManifestIntegrity`**: Valida que todas las skills en `skills.yml` tengan ID válido, `Ref` no vacío, y que todas las referencias en `categories` apunten a skills existentes.
   - **`TestCategoryExpansion`**: Valida que expandir cualquier categoría (`core-agents`, `frontend-design`, etc.) o alias de suite (`mattpocock-skills`, `anthropic-skills`, `n8n-all`) devuelva exactamente los `SkillID` correctos.
   - **`TestMissingSkillModules`**: Valida que los requerimientos de módulos (`nodejs`, `python`) se detecten y resuelvan correctamente.

2. **`profiles_test.go`:**
   - Valida que todos los perfiles planos (`base`, `go`, `python`, etc.) contengan las 6 skills base universales.
   - Valida que el perfil `scraper` mantenga sus skills específicas.

3. **Verificación Toolchain:**
   - `gofmt -l .` limpio.
   - `go vet ./...` limpio.
   - `go test -v ./...` con 100% de tests pasando.
   - Actualización del grafo con `graphify update .`.
