#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="${SCRIPT_DIR}/interserve-mcp"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
if [[ ! -x "$BINARY" ]]; then
    if ! command -v go &>/dev/null; then
        echo '{"error":"go not found — cannot build interserve-mcp. Install Go 1.23+ and restart."}' >&2
        exit 1
    fi
    cd "$PROJECT_ROOT"
    go build -o "$BINARY" ./cmd/interserve-mcp/ 2>&1 >&2
fi
exec "$BINARY" "$@"
