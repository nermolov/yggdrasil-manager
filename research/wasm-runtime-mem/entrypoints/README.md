# WASM Entrypoints

Two custom Rust crates compiled to WASM for the memory benchmark suite.

## subduction-stdio-sync (wasm32-wasip1 — Suite 2)

Reads a length-prefixed Automerge document from stdin, performs an in-memory
sync, and writes the result to stdout. No sockets or async — runs on every
WASI interpreter.

Build: `cargo build --release --target wasm32-wasip1`

## subduction-wasi-server (wasm32-wasip2 — Suite 1)

Synchronous WebSocket sync server using `std::net` blocking TCP (maps to
wasi:sockets on wasip2). Accepts one client, syncs an Automerge document,
then exits cleanly.

Primary strategy: blocking `std::net` + `tungstenite` (no tokio).
Fallback: `--features wstd-fallback` for wstd-based async TCP if std::net
proves insufficient.

Build: `cargo build --release --target wasm32-wasip2`
