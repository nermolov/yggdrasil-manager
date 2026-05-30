//! wasip2 WebSocket sync server for the WASM-runtime memory benchmark.
//!
//! Listens on the address from argv[1] (default: 127.0.0.1:0), accepts one
//! WebSocket connection, runs the Automerge sync loop, then exits 0.
//!
//! Port negotiation: after binding, writes "READY <port>\n" to stdout so the
//! Go harness can connect.

use std::io::{self, Write};
use std::net::TcpListener;

fn main() {
    if let Err(e) = run() {
        eprintln!("subduction-wasi-server error: {e}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let addr = std::env::args().nth(1).unwrap_or_else(|| "127.0.0.1:0".to_string());

    let listener = TcpListener::bind(&addr)?;
    let local_addr = listener.local_addr()?;

    // Signal readiness to the Go harness.
    let stdout = io::stdout();
    let mut stdout = stdout.lock();
    writeln!(stdout, "READY {}", local_addr.port())?;
    stdout.flush()?;
    drop(stdout);

    // Accept exactly one connection.
    let (stream, _peer) = listener.accept()?;
    stream.set_nodelay(true).ok();

    // Perform WebSocket handshake.
    let mut ws = tungstenite::accept(stream)?;

    // Initialize an empty Automerge document to sync against.
    let mut doc = automerge::AutoCommit::new();

    // Run a simple sync loop: receive messages, apply them, send back our state.
    // Exit when the client closes the connection.
    use tungstenite::Message;
    loop {
        match ws.read() {
            Ok(Message::Binary(data)) => {
                let _ = doc.apply_encoded_changes(&data);
                // Echo back our current document state as a sync response.
                let saved = doc.save();
                ws.send(Message::Binary(saved))?;
            }
            Ok(Message::Close(_)) | Err(_) => break,
            Ok(_) => {}
        }
    }

    Ok(())
}
