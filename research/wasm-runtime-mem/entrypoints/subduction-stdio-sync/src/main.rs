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

    // Read length-prefixed sync message from stdin
    let mut len_buf = [0u8; 4];
    stdin.read_exact(&mut len_buf)?;
    let msg_len = u32::from_be_bytes(len_buf) as usize;
    let mut msg = vec![0u8; msg_len];
    stdin.read_exact(&mut msg)?;

    // Load the document from the sync message payload
    // (treat the payload as raw Automerge document bytes)
    let doc = automerge::AutoCommit::load(&msg)
        .or_else(|_| {
            // If load fails, treat as empty doc and apply as a change
            let mut d = automerge::AutoCommit::new();
            let _ = d.apply_encoded_changes(&msg);
            Ok::<_, automerge::AutomergeError>(d)
        })?;

    // Serialize the synced document
    let out = doc.save();

    // Write length-prefixed response to stdout
    let out_len = (out.len() as u32).to_be_bytes();
    stdout.write_all(&out_len)?;
    stdout.write_all(&out)?;
    stdout.flush()?;

    Ok(())
}
