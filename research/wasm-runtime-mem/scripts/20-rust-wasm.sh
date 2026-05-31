#!/usr/bin/env bash
# Add WASM targets and build the two entrypoint crates.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
source "$SCRIPT_DIR/lib.sh"
source "$SCRIPT_DIR/versions.env"

require_cmd rustup
require_cmd cargo
ensure_dir "$ROOT_DIR/$BIN_DIR"

info "Installing Rust WASM targets..."
rustup target add wasm32-wasip1 || true
rustup target add wasm32-wasip2 || true

# wasip3 is experimental — skip with a warning if unavailable
if rustup target add wasm32-wasip3 2>/dev/null; then
  WASIP3_AVAILABLE=1
  info "wasip3 target available"
else
  WASIP3_AVAILABLE=0
  info "WARN: wasip3 target not available — Suite 1 wasip3 path will be skipped"
fi
export WASIP3_AVAILABLE

ENTRYPOINTS_DIR="$ROOT_DIR/entrypoints"

# Build subduction-stdio-sync (wasip1) — Suite 2
info "Building subduction-stdio-sync (wasm32-wasip1)..."
(cd "$ENTRYPOINTS_DIR/subduction-stdio-sync" && \
  cargo build --release --target wasm32-wasip1 2>&1)
cp "$ENTRYPOINTS_DIR/subduction-stdio-sync/target/wasm32-wasip1/release/subduction-stdio-sync.wasm" \
   "$ROOT_DIR/$BIN_DIR/"
info "subduction-stdio-sync.wasm built"

# Build subduction-wasi-server (wasip2) — Suite 1
info "Building subduction-wasi-server (wasm32-wasip2)..."
(cd "$ENTRYPOINTS_DIR/subduction-wasi-server" && \
  cargo build --release --target wasm32-wasip2 2>&1)
cp "$ENTRYPOINTS_DIR/subduction-wasi-server/target/wasm32-wasip2/release/subduction-wasi-server.wasm" \
   "$ROOT_DIR/$BIN_DIR/"
info "subduction-wasi-server.wasm built"

# Build subduction-wasi-server (wasip3) — Suite 1 wasip3 best-effort
if [ "${WASIP3_AVAILABLE:-0}" -eq 1 ]; then
  info "Building subduction-wasi-server (wasm32-wasip3)..."
  (cd "$ENTRYPOINTS_DIR/subduction-wasi-server" && \
    cargo build --release --target wasm32-wasip3 2>&1)
  cp "$ENTRYPOINTS_DIR/subduction-wasi-server/target/wasm32-wasip3/release/subduction-wasi-server.wasm" \
     "$ROOT_DIR/$BIN_DIR/subduction-wasi-server-p3.wasm"
  info "subduction-wasi-server-p3.wasm built"
else
  info "WARN: Skipping wasip3 server build — wasm32-wasip3 not available on this toolchain"
fi

info "WASM build complete. .wasm files:"
ls -lh "$ROOT_DIR/$BIN_DIR/"*.wasm 2>/dev/null || info "No .wasm files found"
