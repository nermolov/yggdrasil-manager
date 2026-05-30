#!/usr/bin/env bash
# Build native Subduction binaries: subduction_cli and automerge-subduction-ingest.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
source "$SCRIPT_DIR/lib.sh"
source "$SCRIPT_DIR/versions.env"

require_cmd cargo
ensure_dir "$ROOT_DIR/$BIN_DIR"

SUBDUCTION="$ROOT_DIR/$SUBDUCTION_DIR"
[ -d "$SUBDUCTION" ] || die "Run 00-fetch-subduction.sh first"

info "Building native Subduction binaries..."
(cd "$SUBDUCTION" && cargo build --release 2>&1)

# Copy built binaries to .cache/bin/
cp "$SUBDUCTION/target/release/subduction_cli" "$ROOT_DIR/$BIN_DIR/" 2>/dev/null || \
  info "WARN: subduction_cli not found — binary name may differ"
# automerge-subduction-ingest may be named differently; try common names
for name in automerge-subduction-ingest subduction-ingest; do
  if [ -f "$SUBDUCTION/target/release/$name" ]; then
    cp "$SUBDUCTION/target/release/$name" "$ROOT_DIR/$BIN_DIR/"
    info "Copied $name to $BIN_DIR/"
  fi
done
info "Native build complete. Contents of $BIN_DIR:"
ls -lh "$ROOT_DIR/$BIN_DIR/"
