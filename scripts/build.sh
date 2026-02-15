#!/usr/bin/env bash
# Build the interlock-mcp binary.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"
echo "Building interlock-mcp..."
go build -o bin/interlock-mcp ./cmd/interlock-mcp/
echo "Built: bin/interlock-mcp"
