# Ticket 03: Refactorización de `skills.go` y Loader Go con `//go:embed`

**Label:** `wayfinder:task`  
**Status:** Blocked by [01-skills-yaml-schema.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/01-skills-yaml-schema.md), [02-functional-categories-taxonomy.md](file:///home/joaco/projects/my-devcontainer-installer/.wayfinder/skills-profiles-refactor/tickets/02-functional-categories-taxonomy.md)

## Question
¿Cómo implementar la carga y deserialización de `skills.yml` en Go de manera ultra-limpia, segura y sin efectos colaterales de inicialización?

## Propuesta Técnica

1. **Embebido en Go:**
   ```go
   //go:embed skills.yml
   var rawSkillsYAML []byte
   ```

2. **Estructuras Tipadas de Dominio:**
   ```go
   type rawManifest struct {
       Categories []rawCategory `yaml:"categories"`
       Skills     []rawSkill    `yaml:"skills"`
   }
   ```

3. **Carga en `init()` y Construcción de Tablas Inmutables:**
   - Deserializa `rawSkillsYAML` usando `go-yaml`.
   - Popula `All []*Spec` y `Categories []*Category`.
   - Mantiene compatibilidad con `Groups []*Group` mediante adaptadores para no romper llamadas existentes.
   - Provee funciones de búsqueda O(1): `Get(id types.SkillID) *Spec`, `GetCategory(id types.SkillCategoryID) *Category`.

4. **Reducción de Código:**
   - `skills.go` pasa de ~555 líneas a ~90 líneas de código Go idiomático y auto-documentado.
