# Ticket 01: Definición del Esquema y Estructura de `skills.yml`

**Label:** `wayfinder:task`  
**Status:** Open (Frontier)

## Question
¿Cuál es la estructura y especificación exacta del archivo YAML `skills.yml` para representar todas las skills, sus dependencias y las categorías funcionales?

## Especificación Propuesta

El archivo `cli/internal/domain/modules/skills/skills.yml` contendrá dos secciones principales:

```yaml
categories:
  - id: <category-id>
    label: <Human readable title>
    aliases: [<optional alias list>]
    skills:
      - <skill-id-1>
      - <skill-id-2>

skills:
  - id: <skill-id>
    label: <Human readable label>
    ref: <package/repo ref>
    skill: <optional sub-skill name>
    requires_modules:
      - <module-id> # nodejs, python, etc.
    requires_env:
      - <env-var-name> # opcional
    category: <category-id>
    description: |
      <Context documentation to inject in ~/CONTEXT.md>
```

## Beneficios
1. Centraliza los datos en un formato estándar y legible.
2. Los desarrolladores pueden agregar o modificar skills editando únicamente el YAML sin recompilar estructuras Go complejas.
3. Permite serialización y validación limpia con `go-yaml`.
