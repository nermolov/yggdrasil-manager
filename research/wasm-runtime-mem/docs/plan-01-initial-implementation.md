---
title: WASM-runtime memory benchmark suite for the Subduction server
status: done
completed: 2026-05-31
branch: claude/wasm-runtime-memory-tests-wRAbA
note: >
  Suite 2 (stdio, 7 interpreters: wasm3, toywasm, wamr-classic, wamr-fast,
  wasmedge, wazero, wasmtime-pulley) fully implemented and measured end-to-end.
  Suite 1 (server) remains and is tracked separately in plan-02 (Option B) and
  plan-03 (Option C). See RESULTS.md for the measured peak-RSS table.
---

# Plan: WASM-runtime memory benchmark suite for the Subduction server

## Context

This is a research task to evaluate the **peak memory usage of interpreted WebAssembly
runtimes** when they host [Subduction](https://github.com/inkandswitch/subduction) — a Rust
P2P CRDT sync protocol — syncing an Automerge document. The work lands in the
`yggdrasil-manager` Go repo (module `github.com/nermolov/yggdrasil-manager`) on branch
`claude/wasm-runtime-memory-tests-wRAbA`, as an out-of-tree Go integration-test suite. It is
unrelated to yggdrasil's networking code; the repo is just the host.

**Why:** to learn which interpreted (non-JIT) WASM runtime runs the Subduction sync server with
the smallest memory footprint, across two workloads: a full socket-based server (WASIp2/p3) and
a simpler stdin/stdout sync executable (WASIp1).

### Key research findings (drive the design)
- Subduction is a Rust workspace. `subduction_cli server --socket 127.0.0.1:PORT` runs a
  WebSocket sync server. `automerge-subduction-ingest --server ws://HOST:PORT --ephemeral-key
  doc.am` is a **separate process** that uploads/syncs an Automerge doc. Existing WASM bindings
  are wasm-bindgen/**browser only** — *not* WASI — so WASI server entrypoints must be custom
  Rust binaries we author (the task explicitly allows this).
- **Suite 1 runtime eligibility** (wasip2/p3 + interpreter mode + C-bindable): realistically
  **only Wasmtime + its Pulley portable interpreter** qualifies today. WAMR / WasmEdge / wasm3 /
  toywasm are WASIp1-only; jco is JS-based. → Suite 1 = native baseline + Wasmtime/Pulley
  (wasip2; wasip3 best-effort), with exclusions documented. (User-confirmed.)
- **Suite 2** (any WASI version, interpreted): wasm3, WAMR (classic + fast interp), WasmEdge
  (interp), Wasmtime/Pulley, toywasm, and **wazero** (pure-Go interpreter, no cgo — included and
  flagged as not-C-bound). (User-confirmed.)
- Environment has Go 1.24, Rust 1.94 + rustup/cargo, cmake 3.28, clang/gcc, GitHub reachable.
  No runtimes preinstalled → all built from source. Plan is to **build & run everything**
  end-to-end this session (user-confirmed), sequencing cheap/high-confidence paths first.

## Measurement methodology

Each runtime runs inside its **own native host-runner subprocess** (a C program linking the
runtime's C API, or for Wasmtime a Rust binary; wazero runs in a tiny Go child for a uniform
method). The Go harness `exec`s the runner and records peak memory two ways and reports both:
- `syscall.Wait4` → `Rusage.Maxrss` (KiB on Linux, peak RSS), and
- a goroutine sampling `/proc/<pid>/status` `VmHWM` every ~5ms until exit.
Isolating the runtime in a subprocess keeps its memory separate from the Go test process.

## Directory layout (new: `research/wasm-runtime-mem/`)

```
research/wasm-runtime-mem/
├── README.md                  # how to run, prerequisites, what's measured, exclusions
├── Makefile                   # setup / subduction / wasm / runtimes / test / clean
├── harness/                   # shared Go pkg, all files //go:build wasmmem
│   ├── runner.go  rss.go      # spawn subprocess; Maxrss + VmHWM peak measurement
│   ├── registry.go            # table-driven Runtime registry (name, suite, wasi, bin, args)
│   ├── doc.go  testdata/document.am   # canonical checked-in Automerge fixture
│   └── report.go              # tablewriter log table + JSON artifact
├── suite1_server_test.go      # TestSuite1Server  (//go:build wasmmem)
├── suite2_stdio_test.go       # TestSuite2Stdio   (//go:build wasmmem)
├── runners/                   # one host-runner per runtime
│   ├── wasmtime/ (Rust, Pulley interp + wasi-sockets)   wasm3/ (C)   wamr/ (C)
│   ├── wasmedge/ (C)   toywasm/ (C)   wazero/ (Go child, wasmmem tag)
│   └── common/protocol.md     # stdio framing + exit-code contract
├── entrypoints/               # custom Rust crates (committed)
│   ├── subduction-wasi-server/    # wasip2 server: std::net first, wstd fallback → Suite 1
│   └── subduction-stdio-sync/     # wasip1 stdin/stdout sync                     → Suite 2
├── scripts/ (versions.env, lib.sh, 00-fetch-subduction, 10-build-native,
│             20-rust-wasm, 30-build-runtimes)
└── .cache/                    # gitignored: subduction clone, runtime builds, bins, .wasm
```
Add `/research/wasm-runtime-mem/.cache/` to root `.gitignore`. All Go files carry
`//go:build wasmmem` so `go test ./...` and the PostToolUse `go vet` hook never touch them; run
via `go test -tags wasmmem ./research/wasm-runtime-mem/...`.

## Work items

### A. Harness + measurement (build first, validates methodology)
- `harness/registry.go`: `Runtime{Name, Suite, WASITarget, RunnerBin, RunnerArgs(mod,doc), Interpreter, CBound}`; `var Registry`. Missing `RunnerBin` → `t.Skip` (partial builds still yield a partial table).
- `harness/runner.go`+`rss.go`: `MeasurePeakRSS(ctx, bin, args, stdin) (Result, error)` — own process group, feed stdin, capture stdout, read `Maxrss`, sample `VmHWM`. Mirror existing subprocess pattern in `integration/main.go` (`exec.CommandContext`).
- `harness/report.go`: accumulate `[]Result`, flush once (TestMain) to `.cache/bin/results-suite{1,2}.json` + a `tablewriter` table (tablewriter already in go.mod). Columns: Runtime, WASI, Interp?, C-bound?, PeakRSS, VmHWM, Wall, OK.
- `harness/testdata/document.am`: checked-in Automerge fixture for stable measurements.

### B. Build orchestration (`scripts/` + `Makefile`)
- `versions.env`: pin `SUBDUCTION_SHA`, `WASMTIME_TAG` (Pulley + wasi-sockets; recent release), `WASM3_SHA`, `WAMR_TAG`, `WASMEDGE_TAG`, `TOYWASM_SHA`, `WAZERO_VER`, `RUST_TOOLCHAIN`.
- `00-fetch-subduction.sh`: shallow clone + checkout pinned SHA into `.cache/subduction` (idempotent).
- `10-build-native.sh`: `cargo build --release` → `subduction_cli`, `automerge-subduction-ingest` → `.cache/bin/`.
- `20-rust-wasm.sh`: `rustup target add wasm32-wasip1 wasm32-wasip2`; wasip3 target gated behind a capability check (skip w/ warning if unavailable). Build both entrypoint crates → `.cache/bin/*.wasm`.
- `30-build-runtimes.sh`: clone+build each runtime at its pin with **interpreter + C API**:
  Wasmtime (Rust runner, Pulley feature, no native codegen); wasm3 (`libm3` via cmake);
  WAMR (`-DWAMR_BUILD_INTERP=1`, both classic & fast-interp libs); WasmEdge (`libwasmedge`,
  interpreter mode); toywasm (`libtoywasm`); wazero (via `go get`, compiled into its Go runner).
  Build in parallel into per-runtime `.cache/runtimes/<name>`.
- `Makefile`: `setup` (00→30), `subduction`, `wasm`, `runtimes`, `test SUITE=1|2`, `clean`.

### C. Custom Rust entrypoints (committed; depend on Subduction crates via git `rev=SUBDUCTION_SHA`)
- `entrypoints/subduction-stdio-sync` (`wasm32-wasip1`, Suite 2): read framed Automerge sync
  messages from **stdin**, drive a Subduction/automerge sync session to completion, write synced
  result to **stdout**, exit. No sockets/async — runnable in *every* interpreter. Minimal deps
  (`subduction_core` + `automerge`). **Build this second** (after harness) — fastest validation.
- `entrypoints/subduction-wasi-server` (`wasm32-wasip2`, +wasip3 best-effort, Suite 1):
  replicate `subduction_cli server` but listen over WASI TCP, run the WebSocket sync loop,
  accept one ingest client, sync, exit cleanly. Deps: `subduction_core` + websocket transport
  + `automerge`.
  - **Socket strategy (user-chosen): blocking `std::net` first, `wstd` fallback.** wasm32-wasip2
    now maps `std::net::TcpListener`/`TcpStream` onto `wasi:sockets`, so first attempt the
    thinnest crate: a synchronous, single-connection server using `std::net` blocking sockets +
    a sync WebSocket handshake (e.g. `tungstenite` in blocking mode, no tokio), driving the
    Subduction sync loop on the current thread. This keeps the entrypoint dependency-light and
    avoids pulling in an async executor.
  - **Fallback only if std::net is insufficient** (missing accept/poll support on the pinned
    Wasmtime, or the WebSocket transport needs async): switch to the wasip2-native async runtime
    [`wstd`](https://crates.io/crates/wstd) (no tokio; built on `wasi:io` poll) with its async
    TCP types. Gate the choice behind a small `#[cfg]`/feature so the fallback is a localized
    swap, not a rewrite.
  - **Riskiest deliverable** (socket + WebSocket inside a component) — build last; isolated so
    Suite 2 ships regardless.

### D. Host runners (`runners/`)
- Per-runtime: load the `.wasm`, wire WASI (stdio for Suite 2; sockets for Wasmtime Suite 1),
  run to exit. C runners link each runtime's C API (demonstrating the "C-bindable" criterion);
  wasmtime runner is Rust (Pulley); wazero runner is a pure-Go child. Shared stdio/exit-code
  contract in `runners/common/protocol.md`.

### E. Tests
- `suite1_server_test.go`: `native-baseline` subtest (start `subduction_cli server`, run
  `automerge-subduction-ingest` against it, measure server RSS) + `wasmtime-pulley` wasip2
  (start wasmtime runner hosting `subduction-wasi-server.wasm`, point the native ingest client
  at its TCP port, measure runner RSS) + wasip3 best-effort. README documents excluded runtimes.
- `suite2_stdio_test.go`: one subtest per Suite-2 runtime (wasm3, wamr-classic, wamr-fast,
  wasmedge, wasmtime-pulley, toywasm, wazero); harness pipes the `.am` doc to stdin, validates
  stdout, records RSS. Verify sync correctness, not just exit code.

## Sequencing (cheap/high-confidence first)
1. Harness + RSS measurement + `.am` fixture; validate against a known process.
2. `subduction-stdio-sync` (wasip1) + **wazero** runner — full Suite 2 path, zero C builds.
3. Remaining Suite-2 interpreters (wasm3, toywasm, WAMR, WasmEdge) — each just adds a registry row.
4. Suite 1: native baseline, then Wasmtime/Pulley + `subduction-wasi-server` (wasip2).
5. wasip3 — best-effort, capability-gated.

## Risks
- **wasip3 immaturity** — target name / Wasmtime support may not be GA → capability-gate, never block.
- **WASI sockets + WebSocket in a component** — make-or-break for Suite 1; mitigated by the
  std::net-first / `wstd`-fallback ladder (try the thinnest path, escalate only if blocked);
  isolated so Suite 2 ships regardless.
- **Subduction API drift** — pin SHA hard; entrypoint crates are the only coupling.
- **RSS attribution** — thread-spawning/mmap-heavy interpreters need the VmHWM sampler, not just Maxrss.
- **Build time/disk** — runtimes from source are slow; parallelize, cache by pin.
- Keep heavy builds **out of** the light `CLAUDE_CODE_REMOTE`-gated `session-start.sh`; they run via `make setup`.

## Verification (end-to-end)
1. `cd research/wasm-runtime-mem && make setup` — fetches Subduction, builds native bins, wasm
   modules, and all runtimes (skips capability-gated wasip3 if unavailable).
2. `make test SUITE=2` → `go test -tags wasmmem -run TestSuite2Stdio ...` — every interpreted
   runtime syncs the doc via stdin/stdout; assert correct synced output + a populated peak-RSS
   table; `results-suite2.json` written.
3. `make test SUITE=1` → native baseline + Wasmtime/Pulley wasip2 (wasip3 best-effort); the
   native `automerge-subduction-ingest` client syncs through each server; assert sync success +
   peak RSS recorded; `results-suite1.json` written.
4. Surface both JSON artifacts + the printed tables to the user as the deliverable; README
   summarizes results and documents which runtimes were excluded from Suite 1 and why.
5. Commit and push to `claude/wasm-runtime-memory-tests-wRAbA` (no PR unless requested).

---

## Implementation outcome (2026-05-31)

**Suite 2 — complete.** All seven interpreters build, run, and are measured. The
suite as committed had four classes of bugs that were found and fixed before it
ran (see git history on the branch): tablewriter v1 API in `report.go`; the
wazero runner not compiling / not forwarding the module stdout the harness
validates; the C runners (wasm3, WAMR, toywasm, WasmEdge) linking wrong/missing
libraries, never redirecting stdin, and mis-detecting the WASI exit trap; WAMR
needing `REF_TYPES` to accept modern rustc output; and the wasmtime runner
requesting a nonexistent `wasi` cargo feature and using the pre-v33 WASI API.
Measured peak RSS (smallest→largest): wasm3 ~6.8 MiB, toywasm ~7.7 MiB,
wamr-classic ~12.0 MiB, wamr-fast ~15.2 MiB, wasmedge ~39.4 MiB, wazero
~41.4 MiB, wasmtime-pulley ~77.0 MiB. See `RESULTS.md`.

**Suite 1 — not complete; carried forward.** native-baseline has a CLI-args bug
(the ingest client expects a document ID, not a file path) and the wasmtime
server path needs a wasip2 component-model rewrite. Tracked in
`plan-02-remaining-suite1.md` (Option B) and `plan-03-remaining-suite1-wasip3.md`
(Option C).
