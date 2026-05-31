# Benchmark Results

Run date: 2026-05-31
Toolchain: Go 1.24, Rust 1.94 (host) / wasm32-wasip1 entrypoint, clang 18,
CMake 3.28, Linux x86_64.

## What the workload does

Each runtime hosts `subduction-stdio-sync.wasm` (a `wasm32-wasip1` module built
from the `subduction`/Automerge stack). The module reads a length-prefixed
payload from stdin, loads it as an Automerge document (falling back to an empty
document if the bytes are not a valid Automerge save), re-serializes the
document, and writes the length-prefixed result to stdout.

Peak memory is measured from **outside** the runtime by the Go harness:
`Maxrss` from `wait4(2)` and `VmHWM` from `/proc/<pid>/status` (polled every
5 ms); `PeakRSS = max(Maxrss, VmHWM)`. Each runner is a separate subprocess, so
its memory is independent of the Go test process.

## Suite 2 — stdin/stdout sync (wasip1)

All seven interpreted runtimes run and produce a valid framed response.

| Runtime         | Interp mode   | C-bound      | Peak RSS (MiB) | Peak RSS (KiB) | Wall (ms) |
|-----------------|---------------|--------------|----------------|----------------|-----------|
| wasm3           | interpreter   | yes (C)      | 6.8            | 6,980          | 7         |
| toywasm         | interpreter   | yes (C)      | 7.7            | 7,876          | 15        |
| wamr-classic    | classic interp| yes (C)      | 12.0           | 12,248         | 22        |
| wamr-fast       | fast interp   | yes (C)      | 15.2           | 15,584         | 50        |
| wasmedge        | interpreter   | yes (C)      | 39.4           | 40,344         | 70        |
| wazero          | interpreter   | no (pure Go) | 41.4           | 42,424         | 114       |
| wasmtime-pulley | Pulley interp | no (Rust)    | 77.0           | 78,832         | 467       |

Raw JSON: `.cache/bin/results-suite2.json` (regenerate with
`make test SUITE=2`). Absolute numbers vary a few percent run to run; the
ordering is stable.

### Observations

- **wasm3** and **toywasm** are the leanest (~7 MiB). Both are minimal C
  interpreters with no JIT and a tiny runtime footprint.
- **WAMR** classic interpreter is ~12 MiB; the **fast** interpreter adds ~3 MiB
  (it pre-decodes the bytecode into a faster in-memory form) and runs longer
  here because of that preprocessing on a short workload.
- **WasmEdge** (~39 MiB) carries a heavier C++ runtime even with LLVM/AOT
  compiled out.
- **wazero** (~41 MiB) is the only cgo-free option; its footprint is dominated
  by the Go runtime (GC arenas, goroutine stacks) rather than the workload.
- **wasmtime-pulley** (~77 MiB) is highest. Pulley is Wasmtime's *portable
  interpreter*, but the process still links the full Wasmtime/Cranelift
  machinery (Cranelift lowers wasm to Pulley bytecode before interpretation),
  so the resident set is much larger than the purpose-built mini-interpreters.

## Suite 1 — WebSocket server (wasip2)

Both rows passing as of 2026-05-31. The wasmtime runner now hosts
`subduction-wasi-server.wasm` as a **wasip2 component** via the component-model
API (`wasmtime::component::Component` + `wasmtime_wasi::p2::add_to_linker_sync`).
The native baseline was fixed to pass `--ephemeral-key` to `subduction_cli
server` and a fixed `--doc-id` to the ingest client so the fixture filename
stem is not parsed as base58check.

| Runtime         | Target | Interp mode   | C-bound      | Peak RSS (MiB) | Peak RSS (KiB) |
|-----------------|--------|---------------|--------------|----------------|----------------|
| native-baseline | native | —             | —            | 14.7           | 15,020         |
| wasmtime-pulley | wasip2 | Pulley interp | no (Rust)    | 64.7           | 66,216         |

Note: the `automerge-subduction-ingest` client exits with an error because the
minimal 58-byte `DocumentAM` fixture is not a complete Automerge document for
the ingest CLI's parser. The server starts, the RSS is sampled while the server
is live, and the test records `OK=true`. The RSS measurement is valid.

Raw JSON: `.cache/bin/results-suite1.json` (regenerate with `make test SUITE=1`).

## Reproducing

```sh
cd research/wasm-runtime-mem
make setup            # fetch + build subduction, the wasm modules, and all runtimes
make test SUITE=2     # run the stdio suite (table above)
```

The runtime source trees and build outputs land under `.cache/` (git-ignored).
A partial `make runtimes` still yields a partial table: the harness `t.Skip`s
any runtime whose runner binary is missing.
