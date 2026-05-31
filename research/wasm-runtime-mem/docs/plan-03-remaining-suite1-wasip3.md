---
title: "Suite 1 completion + wasip3 best-effort (Option C)"
status: done
option: C
branch: claude/wasm-runtime-memory-tests-wRAbA
builds_on: plan-02-remaining-suite1.md (Option B)
alternatives:
  - plan-02-remaining-suite1.md (Option B — native baseline + wasip2 only)
---

# Plan: complete Suite 1 (Option B) + add a wasip3 best-effort path

## Context

Same goal as Option B — complete Suite 1 so the native `automerge-subduction-ingest`
client syncs through both the native baseline and the Wasmtime/Pulley wasip2
server, with peak RSS recorded — **plus** the original plan's full ambition: a
capability-gated **wasip3** server path that is attempted when the toolchain and
runtime support it and cleanly skipped otherwise.

This is **Option C**. It is a strict superset of `plan-02-remaining-suite1.md`
(Option B): do everything there first, then add the wasip3 wiring below. Pick
this only if wasip3 coverage matters more than the near-certain skip on the
current pinned toolchain.

## Prerequisite: complete Option B

Execute every work item in `plan-02-remaining-suite1.md` (native ingest fix,
wasmtime `run_server` component rewrite, std::net/wstd de-risk, wasmtime-pulley
ingest fix, RESULTS rows) and pass its verification. wasip3 work starts only
after Suite 1 is green for native + wasip2.

## Additional work — wasip3 (best-effort, never blocks)

### 1. Build the server for wasip3 when available
File: `scripts/20-rust-wasm.sh` (already gates `rustup target add wasm32-wasip3`
behind a capability check that warns + skips if the target is absent).
- When the target exists, also build `subduction-wasi-server` for
  `wasm32-wasip3` → `.cache/bin/subduction-wasi-server-p3.wasm`.
- When absent, skip with a warning; produce no p3 `.wasm`. Never fail the build.

### 2. Add a wasip3 subtest
File: `suite1_server_test.go`.
- Add a `wasmtime-wasip3` subtest mirroring `wasmtime-pulley`: start the
  wasmtime runner in `server` mode against `subduction-wasi-server-p3.wasm`,
  parse `READY <port>`, point the native ingest client (same `--doc-id` fix) at
  it, sample RSS.
- `t.Skip` if any of: the p3 `.wasm` is missing, the runner reports the
  component is unsupported, or READY is not seen within the timeout. Skipping
  must be silent-pass, mirroring the existing `t.Skip` discipline so a partial
  build still yields a partial table.

### 3. Runner support for wasip3 (only if the host API differs)
File: `runners/wasmtime/src/main.rs`.
- If wasip3 components instantiate through the same `wasmtime_wasi::p2`
  component path as wasip2 on the pinned wasmtime, no runner change is needed —
  the existing `server` mode handles it.
- If the pinned wasmtime exposes a distinct wasip3 host surface, add a minimal
  branch; otherwise leave the runner as-is and rely on step 2's skip.

### 4. Results
File: `RESULTS.md`. Add a wasip3 row with peak RSS if it ran, or an explicit
"skipped — wasip3 target/runtime support unavailable on pinned toolchain" note.

## Files touched
- Everything in Option B's file list, plus:
- `scripts/20-rust-wasm.sh` — p3 build branch.
- `suite1_server_test.go` — `wasmtime-wasip3` subtest.
- `runners/wasmtime/src/main.rs` — only if a distinct p3 host API is required.
- `RESULTS.md` — wasip3 row or skip note.

## Verification (end-to-end)
1. Option B's full verification must pass first (native + wasip2 green).
2. `make wasm` — builds the p3 `.wasm` if the target exists; otherwise logs a
   skip and continues.
3. `make test SUITE=1` — records a `wasmtime-wasip3` row **or** cleanly skips it;
   never blocks or fails on missing wasip3 support.
4. Update `RESULTS.md`, commit, and push to
   `claude/wasm-runtime-memory-tests-wRAbA` (no PR unless requested).

## Risk
Highest of the three options. wasip3 target/runtime support is likely
unavailable on the pinned Rust + wasmtime, so the realistic outcome is a clean
skip with a documented note. The capability gates ensure this never blocks the
native + wasip2 results delivered by Option B.

## Toolchain research findings (2026-05-31)

`wasm32-wasip3` is Tier 3 on all Rust channels (no prebuilt stdlib via rustup).
The working build path is `cargo +nightly-2026-02-01 -Z build-std=std,panic_abort`.
The February 2026 nightly was chosen deliberately: it predates wasip3 stdlib support,
so the component only imports `wasi:*@0.3.0-rc-2026-01-06` (from `wasip3-0.4.0` locked
by getrandom/automerge). Later nightlies add `wasi:*@0.3.0-rc-2026-03-15` stdlib
imports alongside the January RC imports, making the component unrunnable on any
single wasmtime release. The wasmtime v41.0.0 CLI (released 2026-01-20) is the
first release that provides the January RC host APIs via `-S p3`.

Peak RSS measured: **28.8 MiB (29,540 KiB)** — Cranelift JIT mode, not Pulley.
