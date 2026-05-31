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

# wasip3 build via nightly + -Z build-std (Tier 3; no prebuilt artifacts on any channel).
# Strategy: use a pinned nightly whose stdlib predates wasip3-specific WASI APIs.  That way
# the compiled component imports only wasi-0.3.0-rc-2026-01-06 interfaces (from the locked
# wasip3-0.4.0 crate inside the dep graph), which wasmtime v41 satisfies via -S p3.
# Using a newer nightly would add wasi-0.3.0-rc-2026-03-15 stdlib imports alongside the
# January RC imports from getrandom/automerge, making the component unrunnable on any
# single wasmtime release.
bootstrap_wasip3_sysroot() {
  local toolchain="$1"
  local rustlib
  rustlib="$(rustup run "$toolchain" rustc --print sysroot)/lib/rustlib"
  local p3_sc="$rustlib/wasm32-wasip3/lib/self-contained"
  if [ -f "$p3_sc/crt1-command.o" ]; then return 0; fi
  # Copy the wasip2 self-contained CRT objects as the wasip3 sysroot.  The wasip3
  # component model startup is identical at the C-ABI level; only the WASI WIT
  # interfaces differ and those are provided by build-std at compile time.
  local p2_sc="$rustlib/wasm32-wasip2/lib/self-contained"
  if [ ! -f "$p2_sc/crt1-command.o" ]; then
    rustup target add wasm32-wasip2 --toolchain "$toolchain" 2>/dev/null || true
  fi
  if [ ! -f "$p2_sc/crt1-command.o" ]; then
    info "WARN: wasip2 self-contained sysroot not found for $toolchain; wasip3 build will be skipped"
    return 1
  fi
  mkdir -p "$p3_sc"
  cp "$p2_sc/"* "$p3_sc/"
  info "Bootstrapped wasm32-wasip3 sysroot for $toolchain from wasip2 self-contained"
}

WASIP3_AVAILABLE=0
if rustup toolchain install "$RUST_NIGHTLY_P3" --no-self-update 2>&1 | tail -1 >&2 && \
   rustup component add rust-src --toolchain "$RUST_NIGHTLY_P3" 2>/dev/null && \
   bootstrap_wasip3_sysroot "$RUST_NIGHTLY_P3"; then
  WASIP3_AVAILABLE=1
  info "wasip3 nightly toolchain ($RUST_NIGHTLY_P3) ready"
else
  info "WARN: wasip3 nightly setup failed — Suite 1 wasip3 path will be skipped"
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
  info "Building subduction-wasi-server (wasm32-wasip3) with $RUST_NIGHTLY_P3 + build-std..."
  (cd "$ENTRYPOINTS_DIR/subduction-wasi-server" && \
    cargo "+$RUST_NIGHTLY_P3" build -Z build-std=std,panic_abort \
      --target wasm32-wasip3 --release 2>&1)
  cp "$ENTRYPOINTS_DIR/subduction-wasi-server/target/wasm32-wasip3/release/subduction-wasi-server.wasm" \
     "$ROOT_DIR/$BIN_DIR/subduction-wasi-server-p3.wasm"
  info "subduction-wasi-server-p3.wasm built"
else
  info "WARN: Skipping wasip3 server build — nightly+build-std setup failed"
fi

info "WASM build complete. .wasm files:"
ls -lh "$ROOT_DIR/$BIN_DIR/"*.wasm 2>/dev/null || info "No .wasm files found"
