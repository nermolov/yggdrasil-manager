package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
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

	ctx := context.Background()
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	defer rt.Close(ctx)

	// Compile first so the builder can inspect the module's imports to
	// auto-detect which WasmEdge socket extension variant is in use.
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to compile wasm module: %v\n", err)
		os.Exit(1)
	}
	defer compiled.Close(ctx)

	// wasi-go replaces wazero's bundled wasi_snapshot_preview1 and adds the
	// WasmEdge socket extension (sock_open, sock_bind, sock_listen,
	// sock_connect, sock_accept, ...) so the WASM module can create and
	// connect sockets itself, without needing the host to pre-open them.
	ctx, system, err := imports.NewBuilder().
		WithSocketsExtension("auto", compiled).
		Instantiate(ctx, rt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to instantiate WASI: %v\n", err)
		os.Exit(1)
	}
	defer system.Close(ctx)

	modName := filepath.Base(wasmPath)
	mod, err := rt.InstantiateModule(ctx, compiled,
		wazero.NewModuleConfig().WithName(modName))
	if err != nil {
		if exitErr, ok := err.(*sys.ExitError); ok && exitErr.ExitCode() == 0 {
			return // clean proc_exit(0)
		}
		fmt.Fprintf(os.Stderr, "wasm module error: %v\n", err)
		os.Exit(1)
	}
	if mod != nil {
		mod.Close(ctx)
	}
}
