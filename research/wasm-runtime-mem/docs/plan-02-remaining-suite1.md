---
title: "Suite 1 completion — native baseline + wasip2 component server (Option B)"
status: done
option: B
branch: claude/wasm-runtime-memory-tests-wRAbA
supersedes_scope_of: plan-01-initial-implementation.md (Suite 1 only)
alternatives:
  - plan-03-remaining-suite1-wasip3.md (Option C — this plan plus wasip3 best-effort)
---

# Plan: complete Suite 1 — native baseline + wasip2 component server

## Context

Suite 2 of the WASM-runtime memory benchmark is done (7 interpreters measured,
see `RESULTS.md`). **Suite 1 (server mode) does not pass.** This plan closes it:
fix the native baseline and make the Wasmtime/Pulley runner host the wasip2
WebSocket server component, so the native `automerge-subduction-ingest` client
syncs through each server and we record peak RSS for both. This is the workload
the project was originally built to measure.

This is **Option B**. Option A (native baseline only) and Option C (Option B +
wasip3) are the other two scopes that were considered; Option C lives in
`plan-03-remaining-suite1-wasip3.md`.

## Root cause (verified against fetched source under `.cache/subduction`)

1. **native-baseline** — `suite1_server_test.go:74` passes the fixture *path* as
   the positional arg while setting `--ephemeral-key`. In
   `automerge_subduction_ingest/src/main.rs`: `--ephemeral-key` is a **boolean**
   (line 143), and the positional `file`'s **stem** is parsed as a base58check
   document ID (line 288 → `sed_id_from_bs58check`, line 210). Stem `"document"`
   is not valid base58check → `invalid base58check document ID: document`. The
   `server --socket <addr>` invocation is itself correct. The client accepts an
   explicit `--doc-id` as `automerge:<base58check>` or raw 64-char hex.

2. **wasmtime-pulley** — `runners/wasmtime/src/main.rs::run_server` loads the
   `.wasm` via `Module::from_file` + preview1, but `subduction-wasi-server.wasm`
   is a **wasip2 component** → `expected a module but was given a component`.
   The fix is the component-model host API, verified present in
   `wasmtime-wasi-33.0.2`:
   - `wasmtime_wasi::p2::view::{WasiView, IoView}` (traits; `ctx()` / `table()`),
   - `wasmtime_wasi::p2::add_to_linker_sync<T: WasiView>(&mut component::Linker<T>)`,
   - `WasiCtxBuilder::{inherit_network, allow_tcp, socket_addr_check, build}`
     (note `build()`, not `build_p1()`).

## Work items

### 1. Native baseline ingest fix
File: `suite1_server_test.go` (native-baseline subtest, ~line 74).
- Add an explicit `--doc-id` so ID derivation no longer depends on the temp
  filename. Use a fixed raw-hex constant (the client accepts 64-char hex):
  `--doc-id 0000000000000000000000000000000000000000000000000000000000000001`.
- Args become: `{"--server", "ws://"+addr, "--ephemeral-key", "--doc-id", <hex>, docPath}`.
- Confirm the server's RSS is actually captured: the server is long-running, so
  follow the `wasmtime-pulley` subtest's lifecycle (start server, sample
  `harness.SampleVmHWM(pid)` while the ingest client connects, then
  `Process.Kill()` + `Wait`) rather than blocking on `Wait4` for a process that
  never exits on its own. Adjust the native-baseline subtest to that pattern if
  the current `MeasurePeakRSS` call returns before ingest connects.

### 2. wasmtime runner — host the wasip2 component
File: `runners/wasmtime/src/main.rs`, `run_server` only (`run_stdio` is correct
and unchanged; it backs all of Suite 2).
- Define `struct ServerState { table: ResourceTable, wasi: WasiCtx }` and impl
  `wasmtime_wasi::p2::IoView` (`fn table`) + `WasiView` (`fn ctx`).
- Build WASI with sockets enabled:
  ```rust
  let wasi = WasiCtxBuilder::new()
      .arg(mod_path_str).arg(bind_addr)
      .inherit_stdin().inherit_stdout().inherit_stderr()
      .inherit_network().allow_tcp(true)
      .build();           // NOT build_p1()
  ```
- Load + instantiate as a component:
  ```rust
  let component = wasmtime::component::Component::from_file(&engine, &mod_path)?;
  let mut linker = wasmtime::component::Linker::new(&engine);
  wasmtime_wasi::p2::add_to_linker_sync(&mut linker)?;
  ```
- Invoke the command entrypoint. A `wasm32-wasip2` Rust bin exports
  `wasi:cli/run`, not `_start`. Preferred:
  `wasmtime_wasi::p2::bindings::sync::Command::instantiate(&mut store, &component, &linker)?`
  then `cmd.wasi_cli_run().call_run(&mut store)?`. Confirm the exact binding
  path against the installed crate during impl; if it differs, resolve the
  `wasi:cli/run` export by name and call it. Keep `make_engine()`'s Pulley
  target (no JIT).

### 3. De-risk `std::net` on wasip2 at runtime
The server uses blocking `std::net::TcpListener`. `add_to_linker_sync` services
wasi-sockets via an internal async runtime, so blocking should work — possibly
needing `WasiCtxBuilder::allow_blocking_current_thread(true)`. **If the module
traps at bind/accept** (std net unsupported under the pinned wasmtime),
re-introduce the **wstd async fallback** an earlier commit removed:
- `entrypoints/subduction-wasi-server/Cargo.toml`: optional `wstd` dep behind a
  `wstd-fallback` feature.
- `src/main.rs`: `#[cfg(feature = "wstd-fallback")]` async TCP path; default
  stays blocking `std::net`.
- `scripts/20-rust-wasm.sh`: build the server with whichever feature set works.
This ladder is the original plan's designed mitigation; it is only exercised if
the default path traps.

### 4. wasmtime-pulley subtest ingest fix
File: `suite1_server_test.go` (wasmtime-pulley subtest, ~line 147). Apply the
same `--doc-id` addition to its ingest invocation. The READY-line parsing and
RSS sampling already work and need no structural change.

### 5. Results
File: `RESULTS.md`. Fill in both Suite 1 rows (native-baseline + wasmtime-pulley
wasip2) with peak RSS; remove the "future work" note for the server path.

## Files touched
- `suite1_server_test.go` — ingest args for both subtests, native-baseline lifecycle.
- `runners/wasmtime/src/main.rs` — `run_server` component rewrite.
- `entrypoints/subduction-wasi-server/{Cargo.toml,src/main.rs}` — only if step 3 fallback is needed.
- `scripts/20-rust-wasm.sh` — only if step 3 fallback is needed.
- `RESULTS.md`.
- This plan's `status` → `in-progress` → `done` as work proceeds.

## Verification (end-to-end)
1. `cd research/wasm-runtime-mem && make subduction && make wasm && make runtimes`
   (or `make setup`) — builds native bins, the wasip2 server `.wasm`, and the
   wasmtime runner.
2. Direct runner smoke test:
   `./.cache/bin/runner-wasmtime server .cache/bin/subduction-wasi-server.wasm 127.0.0.1:0`
   prints `READY <port>` and accepts a `ws://` connection without trapping.
3. `make test SUITE=1` → `go test -tags wasmmem -run TestSuite1Server ...`:
   - native-baseline PASS with non-zero peak RSS,
   - wasmtime-pulley PASS — READY seen, native ingest client syncs and exits 0,
   - `results-suite1.json` has both rows.
4. Re-run `make test SUITE=2` to confirm no regression in the shared
   `run_stdio` path.
5. Update `RESULTS.md`, commit, and push to
   `claude/wasm-runtime-memory-tests-wRAbA` (no PR unless requested).

## Risk
Moderate. The component rewrite is mechanical against a verified API. The one
unknown is `std::net`-on-wasip2 host support under the pinned wasmtime, which
cannot be fully pre-verified offline — it is exercised live in step 2/3, with
the wstd fallback as mitigation.
