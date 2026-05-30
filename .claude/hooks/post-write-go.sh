#!/bin/bash
set -euo pipefail

INPUT=$(cat)
FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

[[ "$FILE" == *.go ]] || exit 0

cd "$CLAUDE_PROJECT_DIR"
PKG_DIR=$(dirname "${FILE#$CLAUDE_PROJECT_DIR/}")

gofmt -w "$FILE"

VET_OUTPUT=$(go vet "./$PKG_DIR/..." 2>&1) || {
  echo "$VET_OUTPUT" >&2
  exit 2
}
