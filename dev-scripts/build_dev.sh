#!/usr/bin/env bash
set -eu

echo "==> Compilando devcontainer-cli dentro del contenedor..."

# Nos movemos a la raíz del proyecto para que todos los comandos detecten el workspace
cd "$(dirname "$0")/.."

# Verificamos que tengamos la herramienta disponible en el host
if ! command -v devcontainer-cli >/dev/null 2>&1; then
    echo "error: devcontainer-cli no está instalado en el host."
    exit 1
fi

# Nos aseguramos de que el directorio temp-dir exista
mkdir -p temp-dir

# Ejecutamos la compilación dentro del devcontainer
# Como la carpeta está montada como volumen, el binario aparecerá automáticamente en temp-dir/
devcontainer-cli agent exec -w -- bash -c '
  cd cli
  VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
  
  # Compilamos y dejamos el binario en la carpeta compartida temp-dir/
  go build -ldflags "-X main.version=$VERSION" -o ../temp-dir/devcontainer-cli-test ./cmd/devcontainer-cli
  
  chmod +x ../temp-dir/devcontainer-cli-test
'

echo "==> ¡Listo! Binario generado localmente en temp-dir/."
echo "==> Pruébalo ejecutando: ./temp-dir/devcontainer-cli-test --help"
