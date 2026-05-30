package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
)

const maxMemoryBytes int64 = 25 * 1024 * 1024 // 25 MB

var (
	buildOnce    sync.Once
	executorPath string
	buildErr     error
	buildTmpDir  string
)

// wasmExecutor builds (once) and returns the path to the wasm-executor binary.
func wasmExecutor(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		d, err := os.MkdirTemp("", "wasm-executor-test-*")
		if err != nil {
			buildErr = err
			return
		}
		buildTmpDir = d
		bin := filepath.Join(d, "wasm-executor")

		cmd := exec.Command("go", "build", "-o", bin, "./cmd/wasm-executor")
		cmd.Dir = ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build wasm-executor: %w\n%s", err, out)
			return
		}
		executorPath = bin
	})
	if buildErr != nil {
		t.Fatalf("executor build failed: %v", buildErr)
	}
	return executorPath
}

// buildWasm compiles a Rust program to WASM and returns the path to the .wasm file.
func buildWasm(t *testing.T, program string) string {
	t.Helper()
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not in PATH")
	}

	dir := filepath.Join("..", "wasm", "rust", program)
	cmd := exec.Command("cargo", "build", "--target", "wasm32-wasip1", "--release")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cargo build %s: %v\n%s", program, err, out)
	}
	return filepath.Join(dir, "target", "wasm32-wasip1", "release", program+".wasm")
}

// runWasm executes a .wasm file with the executor, asserting zero exit and ≤25 MB peak RSS.
func runWasm(t *testing.T, wasmPath string) {
	t.Helper()
	cmd := exec.Command(wasmExecutor(t), wasmPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasm-executor %s: %v\n%s", filepath.Base(wasmPath), err, out)
	}

	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		t.Fatal("SysUsage did not return *syscall.Rusage")
	}

	// Maxrss is in kilobytes on Linux, bytes on macOS.
	var rssBytes int64
	if runtime.GOOS == "darwin" {
		rssBytes = usage.Maxrss
	} else {
		rssBytes = usage.Maxrss * 1024
	}
	if rssBytes > maxMemoryBytes {
		t.Errorf("peak RSS %d bytes (%.1f MB) exceeds 25 MB limit", rssBytes, float64(rssBytes)/(1024*1024))
	}
}
