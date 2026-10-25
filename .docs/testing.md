# Estrategia y Patrones de Pruebas

Este documento detalla la filosofía de pruebas, las técnicas de simulación de infraestructura y las pruebas de regresión de invariantes en `devcontainer-cli`.

---

## 1. Filosofía y Estructura de Pruebas

- **Herramienta Estándar**: Se utiliza exclusivamente el paquete `testing` de la biblioteca estándar de Go. Quedan prohibidos los frameworks de aserción de terceros.
- **Pruebas Basadas en Tablas (*Table-Driven Tests*)**: Estructura recomendada para validar combinaciones de entradas, opciones y casos de error.
- **Obligatoriedad de Cobertura**: Ningún cambio o adición a módulos Dockerfile (`internal/domain/modules/dockerfile`), servicios Compose (`internal/domain/modules/compose`), funciones de generador/resolución (`internal/domain`) o métodos de servicio (`internal/service`) puede fusionarse sin pruebas unitarias que cubran la funcionalidad y sus casos límite.

---

## 2. Simulación de Docker e Infraestructura

Las pruebas nunca deben requerir un daemon de Docker real en ejecución. Toda interacción se intercepta en memoria mediante mocks dedicados.

### Sustitución de Runner en Capa de Servicio
```go
func TestDownService(t *testing.T) {
    fake := &fakeRunner{}
    docker.SetRunner(fake)
    defer docker.ResetRunner()
    docker.ResetDockerCache()

    svc := service.LifecycleService{
        Report: nopReporter{},
    }
    err := svc.Down("/fake/compose.yml", "test-ws", true)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

### Ayudantes Compartidos de Prueba (`internal/service/helpers_test.go`)
- `nopReporter`: Implementación silenciosa de `service.Reporter` para pruebas que no validan mensajes de salida.
- `fakeRunner`: Simula respuestas y códigos de salida de llamadas a `exec.Command` hacia Docker.
- `scriptedPrompter`: Entrega respuestas pregrabadas a las consultas del asistente interactivo sin requerir intervención de usuario.
- `useFakeDocker(t)`: Configura el entorno temporal y restablece automáticamente el runner de Docker al finalizar el test.

### Pruebas de Dominio con `domain.CaptureFunc`
Las funciones puras de dominio reciben clausuras simuladas para capturar salidas de comandos sin importar paquetes de infraestructura:

```go
fakeCapture := func(args []string) (int, string, string) {
    return 0, "172.25.0.0/28\n172.25.0.16/28", ""
}
subnets, err := domain.ListUsedSubnets(fakeCapture)
```

---

## 3. Pruebas de Invariantes y Contratos Críticos

El repositorio contiene una serie de pruebas automatizadas diseñadas específicamente para evitar regresiones arquitectónicas sutiles:

| Nombre de la Prueba | Archivo de Prueba | Invariante que Protege |
|---|---|---|
| `TestEntrypointStageOrder` | `assets_test.go` | Verifica el orden exacto de las funciones de etapa en `entrypoint.sh` y asegura que ninguna llamada invoque funciones inexistentes. |
| `TestSharedConfigCatalogueIsDeclaredOnlyInGo` | `sharedconfig_test.go` | Valida que la lista de configuraciones compartidas se declare únicamente en Go (`types.SharedConfigEntries`) y no exista duplicación en scripts de shell. |
| `TestErrorMessagesDoNotPointAtTheWizard` | `recovery_hints_test.go` | Analiza el código fuente de comandos y servicios para garantizar que ninguna sugerencia de error recomiende ejecutar el comando raíz interactivo. |
| `TestEntrypointDoesNotTouchAgentMemoryFiles` | `assets_test.go` | Comprueba que el script de entrypoint jamás escriba directamente en archivos de memoria de agentes (`~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`). |
| `TestAptInstallsSkipRecommends` | `render_test.go` | Inspecciona todos los comandos `apt-get install` generados por los módulos para asegurar la inclusión de `--no-install-recommends`. |
| `TestBuiltinProfileRemoteFlagMatchesPublishedVariants` | `profile_test.go` | Mantiene sincronizada la propiedad `Remote: true` de los perfiles con la matriz de publicación en CI (`types.RemoteVariants`). |
| `TestAgentGroup_DoesNotReplaceTheHumanCommands` | `agent_test.go` | Garantiza que la fachada de agentes mantenga disponibles todos los comandos equivalentes para usuarios humanos. |
