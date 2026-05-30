use std::io::{Read, Write};
use wasmedge_wasi_socket::{Shutdown, TcpListener, TcpStream};

fn main() {
    // Bind to a random loopback port. The WasmEdge socket extension routes
    // this through the wasi-go host, which calls the real OS bind/listen.
    let listener = TcpListener::bind("127.0.0.1:0", /*nonblocking=*/ false)
        .expect("bind failed");
    let addr = listener.local_addr().expect("local_addr failed");
    println!("listening on {addr}");

    // Connect to ourselves. The kernel completes the TCP three-way handshake
    // on the loopback interface without needing accept() to run first, so
    // connect() returns and the established connection waits in the accept
    // backlog.
    let mut client = TcpStream::connect(addr).expect("connect failed");
    println!("client connected to {addr}");

    // Dequeue the already-established connection.
    let (mut server, peer) = listener.accept(/*nonblocking=*/ false)
        .expect("accept failed");
    println!("server accepted from {peer}");

    // Client sends a message then signals EOF.
    let msg = b"hello from inside wasm";
    client.write_all(msg).expect("client write failed");
    client.shutdown(Shutdown::Write).expect("client shutdown failed");

    // Server reads until EOF then echoes.
    let mut buf = Vec::new();
    server.read_to_end(&mut buf).expect("server read failed");
    println!("server received: {}", String::from_utf8_lossy(&buf));
    server.write_all(&buf).expect("server write failed");
    server.shutdown(Shutdown::Write).expect("server shutdown failed");

    // Client reads the echo.
    let mut echo = Vec::new();
    client.read_to_end(&mut echo).expect("client read echo failed");
    println!("client received echo: {}", String::from_utf8_lossy(&echo));
}
