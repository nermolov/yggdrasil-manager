//go:build wasmmem

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "runner-wazero: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: runner-wazero <module.wasm> <doc.am>")
	}
	modPath := os.Args[1]
	docPath := os.Args[2]

	wasmBytes, err := os.ReadFile(modPath)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}
	docBytes, err := os.ReadFile(docPath)
	if err != nil {
		return fmt.Errorf("read doc: %w", err)
	}

	ctx := context.Background()
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	// Build a length-prefixed stdin payload.
	var stdinBuf []byte
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(docBytes)))
	stdinBuf = append(stdinBuf, lenBuf...)
	stdinBuf = append(stdinBuf, docBytes...)

	// Capture stdout to validate sync output.
	pr, pw, _ := os.Pipe()
	var stdoutData []byte
	done := make(chan struct{})
	go func() {
		stdoutData, _ = io.ReadAll(pr)
		close(done)
	}()

	cfg := wazero.NewModuleConfig().
		WithStdin(newBytesReader(stdinBuf)).
		WithStdout(pw).
		WithStderr(os.Stderr).
		WithArgs(modPath)

	mod, err := rt.InstantiateWithConfig(ctx, wasmBytes, cfg)
	pw.Close()
	<-done

	if err != nil {
		return fmt.Errorf("instantiate: %w", err)
	}
	if mod != nil {
		_ = mod.Close(ctx)
	}

	// Forward the module's framed stdout to our own stdout so the Go harness
	// can read and validate the synced document (it captures the runner's
	// stdout, not the module's directly).
	if _, werr := os.Stdout.Write(stdoutData); werr != nil {
		return fmt.Errorf("forward stdout: %w", werr)
	}

	// Validate: response must be at least 4 bytes (length prefix)
	if len(stdoutData) < 4 {
		return fmt.Errorf("unexpected stdout length %d", len(stdoutData))
	}
	respLen := binary.BigEndian.Uint32(stdoutData[:4])
	if int(respLen) != len(stdoutData)-4 {
		fmt.Fprintf(os.Stderr, "runner-wazero: warn: length mismatch got %d want %d\n", len(stdoutData)-4, respLen)
	}
	return nil
}

// byteSliceReader wraps a byte slice as an io.Reader.
type byteSliceReader struct {
	data []byte
	pos  int
}

func newBytesReader(b []byte) io.Reader { return &byteSliceReader{data: b} }
func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
