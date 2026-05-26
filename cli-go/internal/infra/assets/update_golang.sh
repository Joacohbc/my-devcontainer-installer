#!/bin/bash
# This script updates the Go installation by sourcing a utility script
# and calling the update_golang function.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/golang_utils.sh"
update_golang