# Runner Protocol

Each host runner is a standalone executable that loads a WASM module and
measures memory from the outside.

## Suite 2 — stdin/stdout sync (wasip1)

Invocation: `runner-<name> <module.wasm> <doc-path>`

1. The runner opens `<doc-path>` and writes it to the WASM module's stdin as a
   4-byte big-endian length prefix followed by the file contents.
2. WASM module processes the document and writes a length-prefixed response to
   stdout.
3. Runner exits with the WASM module's exit code.

## Suite 1 — WebSocket server (wasip2, Wasmtime/Pulley only)

Invocation: `runner-wasmtime-server <module.wasm> [bind-addr]`

1. The runner starts the WASM module with the given bind address (default
   `127.0.0.1:0`).
2. The WASM module writes `READY <port>\n` to stdout when it is listening.
3. The runner forwards that line to its own stdout so the Go harness can read
   the port and connect.
4. After the WASM module exits, the runner exits with the same code.

## Exit codes
- 0: success
- 1: runner internal error (see stderr)
- Other: forwarded from WASM module
