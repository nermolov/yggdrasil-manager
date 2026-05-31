#!/usr/bin/env bash
# Build all interpreted runtimes from source into .cache/runtimes/<name>.
# Runtimes built: wasm3, WAMR (classic+fast interp), WasmEdge, toywasm,
#                 Wasmtime/Pulley runner, wazero runner.
# C harness runners for each C-based runtime are also built here.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
source "$SCRIPT_DIR/lib.sh"
source "$SCRIPT_DIR/versions.env"

require_cmd cmake
require_cmd git
ensure_dir "$ROOT_DIR/$BIN_DIR"
ensure_dir "$ROOT_DIR/$RUNTIMES_DIR"

# ---------------------------------------------------------------------------
# wasm3
# ---------------------------------------------------------------------------
build_wasm3() {
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wasm3"
  if [ -f "$dest/build/source/libm3.a" ]; then info "wasm3 already built"; return 0; fi
  ensure_dir "$dest"
  git clone --depth=1 https://github.com/wasm3/wasm3.git "$dest/src" 2>/dev/null || git -C "$dest/src" pull
  (cd "$dest/src" && git checkout "$WASM3_SHA" 2>/dev/null || true)
  mkdir -p "$dest/build"
  cmake -S "$dest/src" -B "$dest/build" -DBUILD_WASM3_LIBS=ON -DCMAKE_BUILD_TYPE=Release
  cmake --build "$dest/build" --parallel
  info "wasm3 built"
}

build_runner_wasm3() {
  info "Building C runner for wasm3..."
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wasm3"
  (cd "$ROOT_DIR/runners/wasm3" && \
    cmake -S . -B build \
      -DWASM3_SRC="$dest/src" \
      -DWASM3_BUILD="$dest/build" \
      -DCMAKE_BUILD_TYPE=Release && \
    cmake --build build --parallel && \
    cp build/runner-wasm3 "$ROOT_DIR/$BIN_DIR/")
  info "runner-wasm3 built"
}

# ---------------------------------------------------------------------------
# WAMR (classic + fast interpreter)
# ---------------------------------------------------------------------------
build_wamr() {
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wamr"
  ensure_dir "$dest"
  git clone --depth=1 --branch "$WAMR_TAG" https://github.com/bytecodealliance/wasm-micro-runtime.git "$dest/src" 2>/dev/null || true
  # Classic interpreter
  mkdir -p "$dest/build-classic"
  cmake -S "$dest/src/product-mini/platforms/linux" -B "$dest/build-classic" \
    -DWAMR_BUILD_INTERP=1 -DWAMR_BUILD_JIT=0 -DWAMR_BUILD_FAST_INTERP=0 \
    -DWAMR_BUILD_LIBC_WASI=1 -DWAMR_BUILD_REF_TYPES=1 -DWAMR_BUILD_TAIL_CALL=1 \
    -DCMAKE_BUILD_TYPE=Release
  cmake --build "$dest/build-classic" --parallel
  # Fast interpreter
  mkdir -p "$dest/build-fast"
  cmake -S "$dest/src/product-mini/platforms/linux" -B "$dest/build-fast" \
    -DWAMR_BUILD_INTERP=1 -DWAMR_BUILD_JIT=0 -DWAMR_BUILD_FAST_INTERP=1 \
    -DWAMR_BUILD_LIBC_WASI=1 -DWAMR_BUILD_REF_TYPES=1 -DWAMR_BUILD_TAIL_CALL=1 \
    -DCMAKE_BUILD_TYPE=Release
  cmake --build "$dest/build-fast" --parallel
  info "WAMR built (classic + fast interp)"
}

build_runner_wamr_classic() {
  info "Building C runner for wamr-classic..."
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wamr"
  (cd "$ROOT_DIR/runners/wamr" && \
    cmake -S . -B build-classic \
      -DWAMR_SRC="$dest/src" \
      -DWAMR_BUILD="$dest/build-classic" \
      -DWAMR_FAST_INTERP=0 \
      -DCMAKE_BUILD_TYPE=Release && \
    cmake --build build-classic --parallel && \
    cp build-classic/runner-wamr "$ROOT_DIR/$BIN_DIR/runner-wamr-classic")
  info "runner-wamr-classic built"
}

build_runner_wamr_fast() {
  info "Building C runner for wamr-fast..."
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wamr"
  (cd "$ROOT_DIR/runners/wamr" && \
    cmake -S . -B build-fast \
      -DWAMR_SRC="$dest/src" \
      -DWAMR_BUILD="$dest/build-fast" \
      -DWAMR_FAST_INTERP=1 \
      -DCMAKE_BUILD_TYPE=Release && \
    cmake --build build-fast --parallel && \
    cp build-fast/runner-wamr "$ROOT_DIR/$BIN_DIR/runner-wamr-fast")
  info "runner-wamr-fast built"
}

# ---------------------------------------------------------------------------
# WasmEdge (interp-only, no AOT)
# ---------------------------------------------------------------------------
build_wasmedge() {
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wasmedge"
  ensure_dir "$dest"
  git clone --depth=1 --branch "$WASMEDGE_TAG" https://github.com/WasmEdge/WasmEdge.git "$dest/src" 2>/dev/null || true
  mkdir -p "$dest/build"
  cmake -S "$dest/src" -B "$dest/build" \
    -DWASMEDGE_BUILD_AOT_RUNTIME=OFF \
    -DWASMEDGE_BUILD_SHARED_LIB=ON \
    -DCMAKE_BUILD_TYPE=Release
  cmake --build "$dest/build" --parallel
  info "WasmEdge built"
}

build_runner_wasmedge() {
  info "Building C runner for wasmedge..."
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wasmedge"
  (cd "$ROOT_DIR/runners/wasmedge" && \
    cmake -S . -B build \
      -DWASMEDGE_SRC="$dest/src" \
      -DWASMEDGE_BUILD="$dest/build" \
      -DCMAKE_BUILD_TYPE=Release && \
    cmake --build build --parallel && \
    cp build/runner-wasmedge "$ROOT_DIR/$BIN_DIR/")
  info "runner-wasmedge built"
}

# ---------------------------------------------------------------------------
# toywasm
# ---------------------------------------------------------------------------
build_toywasm() {
  local dest="$ROOT_DIR/$RUNTIMES_DIR/toywasm"
  ensure_dir "$dest"
  git clone --depth=1 https://github.com/yamt/toywasm.git "$dest/src" 2>/dev/null || true
  (cd "$dest/src" && git checkout "$TOYWASM_SHA" 2>/dev/null || true)
  mkdir -p "$dest/build"
  cmake -S "$dest/src" -B "$dest/build" -DCMAKE_BUILD_TYPE=Release
  cmake --build "$dest/build" --parallel
  info "toywasm built"
}

build_runner_toywasm() {
  info "Building C runner for toywasm..."
  local dest="$ROOT_DIR/$RUNTIMES_DIR/toywasm"
  (cd "$ROOT_DIR/runners/toywasm" && \
    cmake -S . -B build \
      -DTOYWASM_SRC="$dest/src" \
      -DTOYWASM_BUILD="$dest/build" \
      -DCMAKE_BUILD_TYPE=Release && \
    cmake --build build --parallel && \
    cp build/runner-toywasm "$ROOT_DIR/$BIN_DIR/")
  info "runner-toywasm built"
}

# ---------------------------------------------------------------------------
# Wasmtime / Pulley runner (Rust binary)
# ---------------------------------------------------------------------------
build_wasmtime_runner() {
  local dest="$ROOT_DIR/$RUNTIMES_DIR/wasmtime"
  ensure_dir "$dest"
  info "Building wasmtime/Pulley runner..."
  (cd "$ROOT_DIR/runners/wasmtime" && cargo build --release 2>&1)
  cp "$ROOT_DIR/runners/wasmtime/target/release/runner-wasmtime" "$ROOT_DIR/$BIN_DIR/" 2>/dev/null || true
  info "Wasmtime runner built"
}

# ---------------------------------------------------------------------------
# wazero runner (Go binary)
# ---------------------------------------------------------------------------
build_wazero_runner() {
  info "Building wazero runner (Go)..."
  (cd "$ROOT_DIR/runners/wazero" && go build -o "$ROOT_DIR/$BIN_DIR/runner-wazero" . 2>&1)
  info "wazero runner built"
}

# ---------------------------------------------------------------------------
# Run all builds in parallel, then build C runners after their runtimes
# ---------------------------------------------------------------------------
info "Starting parallel runtime builds..."

build_wasm3 &
PID_WASM3=$!

build_wamr &
PID_WAMR=$!

build_wasmedge &
PID_WASMEDGE=$!

build_toywasm &
PID_TOYWASM=$!

build_wasmtime_runner &
PID_WASMTIME=$!

build_wazero_runner &
PID_WAZERO=$!

# Wait for C-library runtimes before building their runners
wait $PID_WASM3     && build_runner_wasm3 &
wait $PID_WAMR      && { build_runner_wamr_classic & build_runner_wamr_fast & }
wait $PID_WASMEDGE  && build_runner_wasmedge &
wait $PID_TOYWASM   && build_runner_toywasm &

# Wait for all remaining background jobs
wait $PID_WASMTIME
wait $PID_WAZERO
wait

info "All runtimes and runners built. Contents of $BIN_DIR:"
ls -lh "$ROOT_DIR/$BIN_DIR/"
