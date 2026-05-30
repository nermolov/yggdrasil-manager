# WASM Runtime Memory Benchmark Suite

Measures the **peak memory usage** of interpreted (non-JIT) WebAssembly runtimes
hosting [Subduction](https://github.com/inkandswitch/subduction) — a Rust P2P CRDT
sync protocol — syncing an Automerge document.

## What is measured

Each runtime runs inside its own native host-runner subprocess. The Go harness:
1. Spawns the runner via `exec.Cmd` with its own process group
2. Records `Maxrss` from `syscall.Wait4` (peak RSS from the kernel, KiB on Linux)
3. Samples `/proc/<pid>/status` `VmHWM` every 5 ms until the process exits
4. Reports `PeakRSS = max(Maxrss, VmHWM)` — both values are surfaced for transparency

## Suites

### Suite 1 — WebSocket sync server (wasip2)

Runtimes that support WASIp2 with TCP sockets in interpreter mode:

| Runtime | WASI | Notes |
|---------|------|-------|
| native-baseline | — | `subduction_cli server` (native binary, no WASM) |
| wasmtime-pulley | wasip2 | Pulley portable interpreter (no JIT) |
| wasip3 | wasip3 | Best-effort; skipped if target unavailable |

**Excluded runtimes and why:**
- wasm3, WAMR, WasmEdge, toywasm — WASIp1 only; no socket/component support
- wazero — WASIp1 only at time of writing; no wasip2 component model support
- jco — JavaScript-based; out of scope for pure-interpreter comparison

### Suite 2 — stdin/stdout sync (wasip1)

Every interpreter that supports WASIp1 stdio:

| Runtime | Interpreter | C-bound? |
|---------|-------------|----------|
| wazero | yes | no (pure Go) |
| wasm3 | yes | yes |
| wamr-classic | yes | yes |
| wamr-fast | yes (fast interp) | yes |
| wasmedge | yes | yes |
| wasmtime-pulley | yes (Pulley) | no (Rust) |
| toywasm | yes | yes |

## Prerequisites

- Go 1.24+
- Rust 1.87+ with `rustup`
- CMake 3.16+, `clang`/`gcc`
- Internet access to clone Subduction and runtime sources

## Running

```sh
cd research/wasm-runtime-mem

# Build everything from source (≈20-40 min first run)
make setup

# Run Suite 2 (stdio, all interpreters)
make test SUITE=2

# Run Suite 1 (server, native + wasmtime/pulley)
make test SUITE=1
```

Or run directly with Go:
```sh
go test -tags wasmmem -v -timeout 30m -run TestSuite2Stdio ./...
go test -tags wasmmem -v -timeout 30m -run TestSuite1Server ./...
```

## Output

Results are written to `.cache/bin/results-suite{1,2}.json` and printed as a
table after the test run:

```
+------------------+--------+---------+----------+--------------+------------+----------+-----+
| Runtime          | WASI   | Interp? | C-bound? | PeakRSS(KiB) | VmHWM(KiB) | Wall(ms) | OK  |
+------------------+--------+---------+----------+--------------+------------+----------+-----+
| wazero           | wasip1 | yes     | no       | ...          | ...        | ...      | yes |
| wasm3            | wasip1 | yes     | yes      | ...          | ...        | ...      | yes |
| ...              | ...    | ...     | ...      | ...          | ...        | ...      | ... |
+------------------+--------+---------+----------+--------------+------------+----------+-----+
```

## Directory layout

```
research/wasm-runtime-mem/
├── README.md
├── Makefile
├── suite1_server_test.go     # TestSuite1Server  (//go:build wasmmem)
├── suite2_stdio_test.go      # TestSuite2Stdio   (//go:build wasmmem)
├── suite_main_test.go        # TestMain flush    (//go:build wasmmem)
├── harness/                  # Go measurement package
│   ├── registry.go           # Runtime type + Registry var
│   ├── runner.go             # MeasurePeakRSS subprocess wrapper
│   ├── rss.go                # SampleVmHWM / PollVmHWM
│   ├── report.go             # Record / Flush (tablewriter + JSON)
│   ├── fixture.go            # DocumentAM byte literal
│   └── testdata/document.am  # Binary fixture
├── entrypoints/
│   ├── subduction-stdio-sync/   # wasip1: length-prefixed stdin/stdout sync
│   └── subduction-wasi-server/  # wasip2: blocking std::net WebSocket server
├── runners/
│   ├── common/protocol.md    # Subprocess contract
│   ├── wazero/               # Pure-Go runner
│   ├── wasm3/                # C runner (libm3)
│   ├── wamr/                 # C runner (libiwasm, classic + fast)
│   ├── wasmedge/             # C runner (libwasmedge)
│   ├── toywasm/              # C runner (libtoywasm)
│   └── wasmtime/             # Rust runner (Pulley/Cranelift)
└── scripts/
    ├── versions.env          # Pinned SHAs
    ├── lib.sh                # Shell helpers
    ├── 00-fetch-subduction.sh
    ├── 10-build-native.sh
    ├── 20-rust-wasm.sh
    └── 30-build-runtimes.sh
```

## Design notes

- **Subprocess isolation**: each runner is a separate process so its memory is
  independent of the Go test harness.
- **Dual measurement**: `Maxrss` from `wait4(2)` captures the kernel-tracked peak;
  `VmHWM` from `/proc/<pid>/status` is sampled every 5 ms to catch transient peaks
  that the kernel might miss for thread-heavy runtimes.
- **Build tag**: `//go:build wasmmem` on every Go file keeps `go test ./...` and
  linters from touching this directory.
- **Partial builds**: missing runner binaries produce `t.Skip` not `t.Fatal`, so a
  partial `make runtimes` still yields a partial table.
