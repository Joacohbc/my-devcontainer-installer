#!/usr/bin/env bash
# Data-science profile — the tools that are not catalog modules.
#
# This runs as devuser in a build layer, after every module, so uv and the
# declared environment (PATH, UV_SYSTEM_PYTHON, UV_BREAK_SYSTEM_PACKAGES) are
# already in place: the block is emitted after renderEnvironmentBlocks and
# executed through a login shell, which sources ~/.dc-env.sh.
#
# Two install targets, on purpose (same split as the scraper profile):
#   uv tool → a CLI in its own environment, shimmed into ~/.local/bin (owned by
#             devuser, already on PATH). JupyterLab is a server, not a library
#             a project imports, so it belongs here.
#   uv pip  → libraries a project imports, so they have to live in the system
#             interpreter. The python module's own chown is what makes this
#             work without sudo — see cli/internal/domain/modules/dockerfile/python.go.
set -euo pipefail

log() { echo "==> $*"; }

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: $1 is required by the data-science profile but is not installed" >&2
        exit 1
    fi
}

require uv

log "Installing JupyterLab (uv tool)"
uv tool install jupyterlab

log "Installing the Python data-science/analysis stack (system interpreter)"
uv pip install --system --break-system-packages \
    pandas numpy scipy matplotlib seaborn scikit-learn polars duckdb pyarrow openpyxl

log "Data-science toolchain installed:"
for tool in jupyter jupyter-lab; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "    $tool -> $(command -v "$tool")"
    else
        echo "    WARNING: $tool is not on PATH after install" >&2
    fi
done
python3 -c 'import pandas, numpy, scipy, matplotlib, sklearn, polars, duckdb; print("    pandas/numpy/scipy/matplotlib/scikit-learn/polars/duckdb importable from system python")'

echo "==> Start JupyterLab with: jupyter lab --ip=0.0.0.0 --no-browser (published on :8888)"
