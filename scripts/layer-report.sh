#!/usr/bin/env bash
set -euo pipefail

# layer-report.sh: Inspects OCI image manifests and calculates layer sizes,
# shared layer counts with a base image, and layer reuse efficiency.
#
# Usage:
#   scripts/layer-report.sh [BASE_IMAGE] [VARIANT_IMAGE_1] ...

BASE_IMAGE="${1:-ghcr.io/joacohbc/devcontainer-base:latest}"
if [ $# -gt 0 ]; then
  shift
fi

VARIANTS=("$@")
if [ ${#VARIANTS[@]} -eq 0 ]; then
  VARIANTS=(
    "ghcr.io/joacohbc/devcontainer-nodejs:latest"
    "ghcr.io/joacohbc/devcontainer-python:latest"
    "ghcr.io/joacohbc/devcontainer-go:latest"
    "ghcr.io/joacohbc/devcontainer-node-go:latest"
    "ghcr.io/joacohbc/devcontainer-node-python:latest"
  )
fi

echo "=========================================================="
echo "Layer Inspection and Cache Efficiency Report"
echo "Base Image: $BASE_IMAGE"
echo "=========================================================="

BASE_LAYERS_JSON=$(docker buildx imagetools inspect --raw "$BASE_IMAGE" 2>/dev/null || echo "{}")
BASE_LAYER_DIGESTS=()
if [ "$BASE_LAYERS_JSON" != "{}" ]; then
  mapfile -t BASE_LAYER_DIGESTS < <(echo "$BASE_LAYERS_JSON" | jq -r '.layers[].digest // empty' 2>/dev/null || true)
fi

echo "Base image has ${#BASE_LAYER_DIGESTS[@]} layers."
echo ""

for VARIANT in "${VARIANTS[@]}"; do
  echo "--- Analyzing: $VARIANT ---"
  RAW_MANIFEST=$(docker buildx imagetools inspect --raw "$VARIANT" 2>/dev/null || echo "{}")
  if [ "$RAW_MANIFEST" = "{}" ]; then
    echo "  [WARN] Unable to inspect manifest for $VARIANT"
    continue
  fi

  TOTAL_SIZE=0
  SHARED_SIZE=0
  SHARED_LAYERS=0
  TOTAL_LAYERS=0

  while IFS=$'\t' read -r DIGEST SIZE; do
    if [ -n "$DIGEST" ] && [ -n "$SIZE" ]; then
      TOTAL_LAYERS=$((TOTAL_LAYERS + 1))
      TOTAL_SIZE=$((TOTAL_SIZE + SIZE))

      for B_DIGEST in "${BASE_LAYER_DIGESTS[@]}"; do
        if [ "$DIGEST" = "$B_DIGEST" ]; then
          SHARED_LAYERS=$((SHARED_LAYERS + 1))
          SHARED_SIZE=$((SHARED_SIZE + SIZE))
          break
        fi
      done
    fi
  done < <(echo "$RAW_MANIFEST" | jq -r '.layers[] | "\(.digest)\t\(.size)"' 2>/dev/null || true)

  TOTAL_MB=$(awk "BEGIN {printf \"%.2f\", $TOTAL_SIZE / 1048576}")
  SHARED_MB=$(awk "BEGIN {printf \"%.2f\", $SHARED_SIZE / 1048576}")
  REUSE_PCT=0
  if [ "$TOTAL_SIZE" -gt 0 ]; then
    REUSE_PCT=$(awk "BEGIN {printf \"%.1f\", ($SHARED_SIZE / $TOTAL_SIZE) * 100}")
  fi

  echo "  Total layers: $TOTAL_LAYERS ($TOTAL_MB MB)"
  echo "  Shared with base: $SHARED_LAYERS layers ($SHARED_MB MB)"
  echo "  Reused byte ratio: ${REUSE_PCT}%"
  echo ""
done
