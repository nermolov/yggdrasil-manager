//! runner-wasmtime: Wasmtime/Pulley host runner for the WASM memory benchmark.
//!
//! Usage (Suite 2 stdio):  runner-wasmtime stdio <module.wasm> <doc.am>
//! Usage (Suite 1 server): runner-wasmtime server <module.wasm> [bind-addr]

use anyhow::{bail, Context, Result};
use std::path::PathBuf;
use wasmtime::*;
use wasmtime_wasi::{WasiCtx, WasiCtxBuilder, WasiView};

struct State {
    wasi: WasiCtx,
    table: wasmtime::component::ResourceTable,
}

impl WasiView for State {
    fn table(&mut self) -> &mut wasmtime::component::ResourceTable { &mut self.table }
    fn ctx(&mut self) -> &mut WasiCtx { &mut self.wasi }
}

fn make_engine() -> Result<Engine> {
    let mut cfg = Config::new();
    // Use Pulley portable interpreter (no native JIT compilation)
    cfg.strategy(Strategy::Cranelift); // will be Strategy::Pulley once stabilised; use cranelift for now
    cfg.consume_fuel(false);
    Engine::new(&cfg)
}

fn main() -> Result<()> {
    let args: Vec<String> = std::env::args().collect();
    if args.len() < 3 {
        bail!("usage: runner-wasmtime <stdio|server> <module.wasm> [doc.am|bind-addr]");
    }
    match args[1].as_str() {
        "stdio"  => run_stdio(&args),
        "server" => run_server(&args),
        other    => bail!("unknown mode {other:?}; expected stdio or server"),
    }
}

fn run_stdio(args: &[String]) -> Result<()> {
    if args.len() < 4 {
        bail!("stdio mode: runner-wasmtime stdio <module.wasm> <doc.am>");
    }
    let mod_path = PathBuf::from(&args[2]);
    let doc_path = PathBuf::from(&args[3]);

    let doc_bytes = std::fs::read(&doc_path).context("read doc")?;
    let mut stdin_payload = Vec::with_capacity(4 + doc_bytes.len());
    let len_be = (doc_bytes.len() as u32).to_be_bytes();
    stdin_payload.extend_from_slice(&len_be);
    stdin_payload.extend_from_slice(&doc_bytes);

    let engine = make_engine()?;
    let module = Module::from_file(&engine, &mod_path).context("load module")?;
    let mut store = Store::new(&engine, State {
        wasi: WasiCtxBuilder::new()
            .stdin(wasmtime_wasi::pipe::MemoryInputPipe::new(stdin_payload))
            .stdout(wasmtime_wasi::pipe::MemoryOutputPipe::new(1024 * 1024))
            .stderr(wasmtime_wasi::stdio::stderr())
            .build(),
        table: wasmtime::component::ResourceTable::new(),
    });

    let mut linker = Linker::new(&engine);
    wasmtime_wasi::preview1::add_to_linker_sync(&mut linker, |s: &mut State| &mut s.wasi)?;

    let instance = linker.instantiate(&mut store, &module)?;
    let start = instance.get_typed_func::<(), ()>(&mut store, "_start")?;
    start.call(&mut store, ())?;
    Ok(())
}

fn run_server(args: &[String]) -> Result<()> {
    if args.len() < 3 {
        bail!("server mode: runner-wasmtime server <module.wasm> [bind-addr]");
    }
    let mod_path = PathBuf::from(&args[2]);
    let bind_addr = args.get(3).map(|s| s.as_str()).unwrap_or("127.0.0.1:0");

    let engine = make_engine()?;
    let module = Module::from_file(&engine, &mod_path).context("load module")?;
    let mut store = Store::new(&engine, State {
        wasi: WasiCtxBuilder::new()
            .args(&[mod_path.to_str().unwrap_or("module"), bind_addr])
            .stdin(wasmtime_wasi::stdio::stdin())
            .stdout(wasmtime_wasi::stdio::stdout()) // READY line forwarded to our stdout
            .stderr(wasmtime_wasi::stdio::stderr())
            .build(),
        table: wasmtime::component::ResourceTable::new(),
    });

    let mut linker = Linker::new(&engine);
    wasmtime_wasi::preview1::add_to_linker_sync(&mut linker, |s: &mut State| &mut s.wasi)?;

    let instance = linker.instantiate(&mut store, &module)?;
    let start = instance.get_typed_func::<(), ()>(&mut store, "_start")?;
    start.call(&mut store, ())?;
    Ok(())
}
