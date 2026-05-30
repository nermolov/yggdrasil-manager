package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental/sock"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: wasm-executor <file.wasm>")
		os.Exit(1)
	}

	wasmPath := os.Args[1]
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read wasm file: %v\n", err)
		os.Exit(1)
	}

	// Find a free TCP port to use for the pre-opened listener.
	port, err := freePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find free port: %v\n", err)
		os.Exit(1)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Fprintf(os.Stderr, "host: pre-opening TCP listener on %s\n", addr)

	ctx := context.Background()

	// Inject a pre-opened TCP listener into the WASM module's fd table.
	// wazero's experimental/sock supports sock_accept/recv/send/shutdown but
	// not sock_open/bind/listen/connect, so the WASM module cannot create or
	// connect sockets itself — the host provides the listener and acts as client.
	sockCfg := sock.NewConfig().WithTCPListener("127.0.0.1", port)
	ctx = sock.WithConfig(ctx, sockCfg)

	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	// Start the host-side client before instantiating the module. The module
	// blocks on sock_accept(3, 0), so the client goroutine and WASM run
	// concurrently: the goroutine retries until wazero has bound the port.
	done := make(chan struct{})
	go hostClient(addr, done)

	cfg := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithStderr(os.Stderr)

	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to instantiate wasm module: %v\n", err)
		os.Exit(1)
	}
	defer mod.Close(ctx)

	<-done
}

// hostClient connects to addr (retrying until the listener is ready), sends a
// message, then reads and prints the echo. It closes done when finished.
func hostClient(addr string, done chan<- struct{}) {
	defer close(done)

	var conn net.Conn
	var err error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err = net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "host client: failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	msg := "hello from wazero host"
	fmt.Fprintf(os.Stderr, "host client: sending %q\n", msg)
	if _, err := fmt.Fprint(conn, msg); err != nil {
		fmt.Fprintf(os.Stderr, "host client: write error: %v\n", err)
		return
	}
	// Signal EOF so the WASM side's fd_read loop terminates.
	conn.(*net.TCPConn).CloseWrite()

	echo, err := io.ReadAll(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "host client: read error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "host client: received echo %q\n", string(echo))
}

// freePort asks the OS for an available TCP port on loopback.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}
