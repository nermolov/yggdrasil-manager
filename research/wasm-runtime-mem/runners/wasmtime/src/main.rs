//! runner-wasmtime: Wasmtime/Pulley host runner for the WASM memory benchmark.
//!
//! Usage (Suite 2 stdio):  runner-wasmtime stdio <module.wasm> <doc.am>
//! Usage (Suite 1 server): runner-wasmtime server <module.wasm> [bind-addr]
//!
//! The engine is configured to execute via the Pulley portable interpreter
//! (no native JIT) so the measured memory reflects interpretation, not
//! compiled code.

use anyhow::{bail, Context, Result};
use std::io::Write;
use std::path::PathBuf;
use wasmtime::{Config, Engine, Linker, Module, Store};
use wasmtime_wasi::p2::pipe::{MemoryInputPipe, MemoryOutputPipe};
use wasmtime_wasi::p2::WasiCtxBuilder;
use wasmtime_wasi::preview1::{self, WasiP1Ctx};

fn make_engine() -> Result<Engine> {
    let mut cfg = Config::new();
    // Select the Pulley portable interpreter target. Cranelift compiles the
    // module to Pulley bytecode, which the interpreter then executes — no
    // native machine code is generated or run.
    cfg.target("pulley64")
        .context("select pulley64 target")?;
    cfg.consume_fuel(false);
    Engine::new(&cfg)
}

fn main() -> Result<()> {
    let args: Vec<String> = std::env::args().collect();
    if args.len() < 3 {
        bail!("usage: runner-wasmtime <stdio|server> <module.wasm> [doc.am|bind-addr]");
    }
    match args[1].as_str() {
        "stdio" => run_stdio(&args),
        "server" => run_server(&args),
        other => bail!("unknown mode {other:?}; expected stdio or server"),
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
    stdin_payload.extend_from_slice(&(doc_bytes.len() as u32).to_be_bytes());
    stdin_payload.extend_from_slice(&doc_bytes);

    // Capture the module's stdout so we can forward the framed response to our
    // own stdout (the harness validates the runner's stdout).
    let stdout_pipe = MemoryOutputPipe::new(1024 * 1024);

    let wasi = WasiCtxBuilder::new()
        .stdin(MemoryInputPipe::new(stdin_payload))
        .stdout(stdout_pipe.clone())
        .inherit_stderr()
        .build_p1();

    let engine = make_engine()?;
    let module = Module::from_file(&engine, &mod_path).context("load module")?;
    let mut store = Store::new(&engine, wasi);

    let mut linker: Linker<WasiP1Ctx> = Linker::new(&engine);
    preview1::add_to_linker_sync(&mut linker, |s| s)?;

    let instance = linker.instantiate(&mut store, &module)?;
    let start = instance.get_typed_func::<(), ()>(&mut store, "_start")?;
    let _ = start.call(&mut store, ());

    // Drop the store so the only remaining clone of the pipe is ours, then
    // forward the captured framed output.
    drop(store);
    let out = stdout_pipe.contents();
    std::io::stdout().write_all(&out).context("forward stdout")?;
    std::io::stdout().flush().ok();
    Ok(())
}

fn run_server(args: &[String]) -> Result<()> {
    if args.len() < 3 {
        bail!("server mode: runner-wasmtime server <module.wasm> [bind-addr]");
    }
    let mod_path = PathBuf::from(&args[2]);
    let bind_addr = args.get(3).map(|s| s.as_str()).unwrap_or("127.0.0.1:0");

    let wasi = WasiCtxBuilder::new()
        .arg(mod_path.to_str().unwrap_or("module"))
        .arg(bind_addr)
        .inherit_stdin()
        .inherit_stdout() // READY line forwarded to our stdout
        .inherit_stderr()
        .build_p1();

    let engine = make_engine()?;
    let module = Module::from_file(&engine, &mod_path).context("load module")?;
    let mut store = Store::new(&engine, wasi);

    let mut linker: Linker<WasiP1Ctx> = Linker::new(&engine);
    preview1::add_to_linker_sync(&mut linker, |s| s)?;

    let instance = linker.instantiate(&mut store, &module)?;
    let start = instance.get_typed_func::<(), ()>(&mut store, "_start")?;
    start.call(&mut store, ())?;
    Ok(())
}
