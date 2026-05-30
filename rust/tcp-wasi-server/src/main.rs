// This program runs under WASI preview1 with wazero's experimental/sock support.
// The host (Go) pre-opens a TCP listener and passes it as fd 3 (the first fd
// after stdin/stdout/stderr). The host also connects to that port, so from
// the WASM side we only need to accept + exchange data.
//
// wazero does not implement sock_open/bind/listen/connect, so outbound TCP
// connections and self-binding are not possible within WASM on this runtime.
// The host handles the client side.

use wasi::{Ciovec, Iovec};

fn main() {
    // fd 3 is the pre-opened TCP listener injected by wazero's experimental/sock.
    // It sits right after the three standard streams (0=stdin, 1=stdout, 2=stderr).
    let listener_fd: wasi::Fd = 3;

    eprintln!("tcp-wasi-server: waiting for connection on pre-opened fd {listener_fd}");

    // Block until the host connects.
    let conn_fd = unsafe { wasi::sock_accept(listener_fd, 0) }
        .expect("sock_accept failed");

    eprintln!("tcp-wasi-server: connection accepted on fd {conn_fd}");

    // Read until EOF (the host shuts down its write side after sending).
    let mut message = Vec::new();
    let mut tmp = [0u8; 256];
    loop {
        let iov = Iovec {
            buf: tmp.as_mut_ptr(),
            buf_len: tmp.len() as wasi::Size,
        };
        let n = unsafe { wasi::fd_read(conn_fd, &[iov]) }
            .expect("fd_read failed") as usize;
        if n == 0 {
            break;
        }
        message.extend_from_slice(&tmp[..n]);
    }

    let text = std::str::from_utf8(&message).unwrap_or("(invalid utf-8)");
    println!("tcp-wasi-server received: {text}");

    // Echo the message back.
    let ciov = Ciovec {
        buf: message.as_ptr(),
        buf_len: message.len() as wasi::Size,
    };
    unsafe { wasi::fd_write(conn_fd, &[ciov]) }.expect("fd_write failed");

    unsafe { wasi::fd_close(conn_fd) }.ok();

    println!("tcp-wasi-server: done");
}
