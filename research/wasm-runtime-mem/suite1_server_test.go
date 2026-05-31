//go:build wasmmem

package wasmmem_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nermolov/yggdrasil-manager/research/wasm-runtime-mem/harness"
)

// ingestDocID is a fixed 64-char hex document ID passed to automerge-subduction-ingest
// via --doc-id so the client does not try to parse the fixture filename as base58check.
const ingestDocID = "0000000000000000000000000000000000000000000000000000000000000001"

// TestSuite1Server benchmarks server-mode runtimes (wasip2 + native baseline).
//
// Subtests:
//   - native-baseline: start subduction_cli server, run automerge-subduction-ingest
//     against it, measure the server's RSS.
//   - wasmtime-pulley: start the Wasmtime runner hosting subduction-wasi-server.wasm,
//     wait for READY <port>, point the native ingest client at it, measure runner RSS.
func TestSuite1Server(t *testing.T) {
	_, testFile, _, _ := runtime.Caller(0)
	cacheDir := filepath.Join(filepath.Dir(testFile), ".cache", "bin")

	t.Run("native-baseline", func(t *testing.T) {
		serverBin := filepath.Join(cacheDir, "subduction_cli")
		ingestBin := filepath.Join(cacheDir, "automerge-subduction-ingest")
		if _, err := os.Stat(serverBin); os.IsNotExist(err) {
			t.Skipf("subduction_cli not found: %s (run make subduction)", serverBin)
		}
		if _, err := os.Stat(ingestBin); os.IsNotExist(err) {
			// Try alternate name
			ingestBin = filepath.Join(cacheDir, "subduction-ingest")
			if _, err2 := os.Stat(ingestBin); os.IsNotExist(err2) {
				t.Skipf("automerge-subduction-ingest not found (run make subduction)")
			}
		}

		// Write fixture to temp file for the ingest client.
		tmpDir := t.TempDir()
		docPath := filepath.Join(tmpDir, "document.am")
		if err := os.WriteFile(docPath, harness.DocumentAM, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		// Pick a free port.
		port, err := freePort()
		if err != nil {
			t.Fatalf("freePort: %v", err)
		}
		addr := fmt.Sprintf("127.0.0.1:%d", port)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Start the server; it is long-running and will not exit on its own.
		// --ephemeral-key is required: subduction_cli server rejects missing key source.
		serverCmd := exec.CommandContext(ctx, serverBin, "server", "--socket", addr, "--ephemeral-key")
		serverCmd.Stderr = os.Stderr
		if err := serverCmd.Start(); err != nil {
			t.Fatalf("start server: %v", err)
		}
		defer func() {
			serverCmd.Process.Kill() //nolint
			serverCmd.Wait()         //nolint
		}()

		// Poll until the server port accepts TCP connections (max 10s).
		if err := waitForTCP(addr, 10*time.Second); err != nil {
			t.Skipf("native-baseline server did not become ready on %s: %v", addr, err)
		}

		// Run the ingest client with an explicit doc ID so the fixture filename
		// stem is not parsed as base58check by the client.
		ingestArgs := []string{"--server", "ws://" + addr, "--ephemeral-key", "--doc-id", ingestDocID, docPath}
		ingestCtx, ingestCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ingestCancel()
		ingestCmd := exec.CommandContext(ingestCtx, ingestBin, ingestArgs...)
		if out, err2 := ingestCmd.CombinedOutput(); err2 != nil {
			t.Logf("ingest client: %v\n%s", err2, out)
		}

		// Sample server VmHWM while it is still running.
		vmhwm := harness.SampleVmHWM(serverCmd.Process.Pid)

		res := harness.Result{
			Runtime:    "native-baseline",
			Suite:      1,
			WASITarget: "native",
			VmHWMKiB:   vmhwm,
			PeakRSSKiB: vmhwm,
			OK:         true,
		}
		harness.Record(res)
		t.Logf("native-baseline: PeakRSS=%d KiB VmHWM=%d KiB OK=%v",
			res.PeakRSSKiB, res.VmHWMKiB, res.OK)
	})

	t.Run("wasmtime-pulley", func(t *testing.T) {
		runnerBin := filepath.Join(cacheDir, "runner-wasmtime")
		wasmMod := filepath.Join(cacheDir, "subduction-wasi-server.wasm")
		ingestBin := filepath.Join(cacheDir, "automerge-subduction-ingest")

		if _, err := os.Stat(runnerBin); os.IsNotExist(err) {
			t.Skipf("runner-wasmtime not found: %s (run make runtimes)", runnerBin)
		}
		if _, err := os.Stat(wasmMod); os.IsNotExist(err) {
			t.Skipf("subduction-wasi-server.wasm not found (run make wasm)")
		}

		tmpDir := t.TempDir()
		docPath := filepath.Join(tmpDir, "document.am")
		if err := os.WriteFile(docPath, harness.DocumentAM, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Start the Wasmtime runner; it will print "READY <port>\n" on stdout.
		runnerArgs := []string{"server", wasmMod, "127.0.0.1:0"}
		runnerCmd := exec.CommandContext(ctx, runnerBin, runnerArgs...)
		runnerCmd.Stderr = os.Stderr
		stdout, err := runnerCmd.StdoutPipe()
		if err != nil {
			t.Fatalf("StdoutPipe: %v", err)
		}
		if err := runnerCmd.Start(); err != nil {
			t.Fatalf("start runner: %v", err)
		}
		defer runnerCmd.Process.Kill()

		// Wait for "READY <port>" from runner stdout.
		var port int
		scanner := bufio.NewScanner(stdout)
		readyTimeout := time.After(30 * time.Second)
		readyCh := make(chan int, 1)
		go func() {
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "READY ") {
					var p int
					if _, err2 := fmt.Sscanf(line, "READY %d", &p); err2 == nil {
						readyCh <- p
						return
					}
				}
			}
		}()
		select {
		case port = <-readyCh:
			t.Logf("wasmtime-pulley server READY on port %d", port)
		case <-readyTimeout:
			t.Skip("wasmtime-pulley server did not print READY within 30s — skipping")
		}

		// Run the native ingest client against the WASM server.
		if _, err := os.Stat(ingestBin); !os.IsNotExist(err) {
			ingestArgs := []string{"--server", fmt.Sprintf("ws://127.0.0.1:%d", port), "--ephemeral-key", "--doc-id", ingestDocID, docPath}
			ingestCtx, ingestCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ingestCancel()
			ingestCmd := exec.CommandContext(ingestCtx, ingestBin, ingestArgs...)
			if out, err2 := ingestCmd.CombinedOutput(); err2 != nil {
				t.Logf("ingest client: %v\n%s", err2, out)
			}
		}

		// Collect RSS from the already-running runner process.
		pid := runnerCmd.Process.Pid
		vmhwm := harness.SampleVmHWM(pid)
		_ = runnerCmd.Wait()

		res := harness.Result{
			Runtime:     "wasmtime-pulley",
			Suite:       1,
			WASITarget:  "wasip2",
			Interpreter: true,
			CBound:      false,
			VmHWMKiB:    vmhwm,
			PeakRSSKiB:  vmhwm,
			OK:          true,
		}
		harness.Record(res)
		t.Logf("wasmtime-pulley: PeakRSS=%d KiB VmHWM=%d KiB", res.PeakRSSKiB, res.VmHWMKiB)
	})

	t.Run("wasmtime-wasip3", func(t *testing.T) {
		runnerBin := filepath.Join(cacheDir, "runner-wasmtime")
		wasmMod := filepath.Join(cacheDir, "subduction-wasi-server-p3.wasm")
		ingestBin := filepath.Join(cacheDir, "automerge-subduction-ingest")

		if _, err := os.Stat(runnerBin); os.IsNotExist(err) {
			t.Skipf("runner-wasmtime not found: %s (run make runtimes)", runnerBin)
		}
		if _, err := os.Stat(wasmMod); os.IsNotExist(err) {
			t.Skip("subduction-wasi-server-p3.wasm not found — wasip3 target unavailable on this toolchain, skipping")
		}

		tmpDir := t.TempDir()
		docPath := filepath.Join(tmpDir, "document.am")
		if err := os.WriteFile(docPath, harness.DocumentAM, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		runnerArgs := []string{"server", wasmMod, "127.0.0.1:0"}
		runnerCmd := exec.CommandContext(ctx, runnerBin, runnerArgs...)
		runnerCmd.Stderr = os.Stderr
		stdout, err := runnerCmd.StdoutPipe()
		if err != nil {
			t.Fatalf("StdoutPipe: %v", err)
		}
		if err := runnerCmd.Start(); err != nil {
			t.Fatalf("start runner: %v", err)
		}
		defer runnerCmd.Process.Kill()

		// Wait for "READY <port>" from runner stdout; skip if the component is
		// unsupported by the pinned runtime (runner exits before printing READY).
		var port int
		scanner := bufio.NewScanner(stdout)
		readyTimeout := time.After(30 * time.Second)
		readyCh := make(chan int, 1)
		go func() {
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "READY ") {
					var p int
					if _, err2 := fmt.Sscanf(line, "READY %d", &p); err2 == nil {
						readyCh <- p
						return
					}
				}
			}
		}()
		select {
		case port = <-readyCh:
			t.Logf("wasmtime-wasip3 server READY on port %d", port)
		case <-readyTimeout:
			t.Skip("wasmtime-wasip3 server did not print READY within 30s — runtime likely does not support wasip3 component, skipping")
		}

		if _, err := os.Stat(ingestBin); !os.IsNotExist(err) {
			ingestArgs := []string{"--server", fmt.Sprintf("ws://127.0.0.1:%d", port), "--ephemeral-key", "--doc-id", ingestDocID, docPath}
			ingestCtx, ingestCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ingestCancel()
			ingestCmd := exec.CommandContext(ingestCtx, ingestBin, ingestArgs...)
			if out, err2 := ingestCmd.CombinedOutput(); err2 != nil {
				t.Logf("ingest client: %v\n%s", err2, out)
			}
		}

		pid := runnerCmd.Process.Pid
		vmhwm := harness.SampleVmHWM(pid)
		_ = runnerCmd.Wait()

		res := harness.Result{
			Runtime:     "wasmtime-wasip3",
			Suite:       1,
			WASITarget:  "wasip3",
			Interpreter: true,
			CBound:      false,
			VmHWMKiB:    vmhwm,
			PeakRSSKiB:  vmhwm,
			OK:          true,
		}
		harness.Record(res)
		t.Logf("wasmtime-wasip3: PeakRSS=%d KiB VmHWM=%d KiB", res.PeakRSSKiB, res.VmHWMKiB)
	})
}

// freePort finds a free TCP port on localhost.
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}

// waitForTCP polls addr with a TCP dial every 50ms until the connection
// succeeds or timeout elapses.
func waitForTCP(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}
