use std::io::{self, Read, Write};

fn main() {
    if let Err(e) = run() {
        eprintln!("subduction-stdio-sync error: {e}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let stdin = io::stdin();
    let mut stdin = stdin.lock();
    let stdout = io::stdout();
    let mut stdout = stdout.lock();

    // Read length-prefixed payload from stdin: [4-byte BE length][bytes]
    let mut len_buf = [0u8; 4];
    stdin.read_exact(&mut len_buf)?;
    let msg_len = u32::from_be_bytes(len_buf) as usize;
    let mut msg = vec![0u8; msg_len];
    stdin.read_exact(&mut msg)?;

    // Load the Automerge document, or start with an empty one if invalid.
    let doc = automerge::Automerge::load(&msg).unwrap_or_else(|_| automerge::Automerge::new());

    // Serialize the (potentially synced) document.
    let out = doc.save();

    // Write length-prefixed response to stdout.
    let out_len = (out.len() as u32).to_be_bytes();
    stdout.write_all(&out_len)?;
    stdout.write_all(&out)?;
    stdout.flush()?;

    Ok(())
}
