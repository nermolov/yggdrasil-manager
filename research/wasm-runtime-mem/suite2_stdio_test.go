//go:build wasmmem

package wasmmem_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nermolov/yggdrasil-manager/research/wasm-runtime-mem/harness"
)

// TestSuite2Stdio benchmarks interpreted WASM runtimes running the
// subduction-stdio-sync wasip1 module.
//
// Each subtest:
//  1. Locates the runner binary and the .wasm module in .cache/bin/
//  2. Runs MeasurePeakRSS, feeding document.am as a length-prefixed stdin payload
//  3. Validates the length-prefixed stdout response
//  4. Records the result via harness.Record
//
// Missing binaries → t.Skip (partial builds still yield a partial table).
func TestSuite2Stdio(t *testing.T) {
	// Locate the .cache/bin directory relative to this test file.
	_, testFile, _, _ := runtime.Caller(0)
	cacheDir := filepath.Join(filepath.Dir(testFile), ".cache", "bin")
	wasmMod := filepath.Join(cacheDir, "subduction-stdio-sync.wasm")

	suite2Runtimes := []struct {
		name       string
		runner     string
		wasiTarget string
		interp     bool
		cbound     bool
		args       func(mod, doc string) []string
	}{
		{
			name: "wazero", runner: "runner-wazero",
			wasiTarget: "wasip1", interp: true, cbound: false,
			args: func(mod, doc string) []string { return []string{mod, doc} },
		},
		{
			name: "wasm3", runner: "runner-wasm3",
			wasiTarget: "wasip1", interp: true, cbound: true,
			args: func(mod, doc string) []string { return []string{mod, doc} },
		},
		{
			name: "wamr-classic", runner: "runner-wamr-classic",
			wasiTarget: "wasip1", interp: true, cbound: true,
			args: func(mod, doc string) []string { return []string{mod, doc} },
		},
		{
			name: "wamr-fast", runner: "runner-wamr-fast",
			wasiTarget: "wasip1", interp: true, cbound: true,
			args: func(mod, doc string) []string { return []string{mod, doc} },
		},
		{
			name: "wasmedge", runner: "runner-wasmedge",
			wasiTarget: "wasip1", interp: true, cbound: true,
			args: func(mod, doc string) []string { return []string{mod, doc} },
		},
		{
			name: "wasmtime-pulley", runner: "runner-wasmtime",
			wasiTarget: "wasip1", interp: true, cbound: false,
			args: func(mod, doc string) []string { return []string{"stdio", mod, doc} },
		},
		{
			name: "toywasm", runner: "runner-toywasm",
			wasiTarget: "wasip1", interp: true, cbound: true,
			args: func(mod, doc string) []string { return []string{mod, doc} },
		},
	}

	// Write the document fixture to a temp file (some runners need a path, not stdin).
	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "document.am")
	if err := os.WriteFile(docPath, harness.DocumentAM, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	for _, rt := range suite2Runtimes {
		rt := rt
		t.Run(rt.name, func(t *testing.T) {
			runnerBin := filepath.Join(cacheDir, rt.runner)
			if _, err := os.Stat(runnerBin); os.IsNotExist(err) {
				t.Skipf("runner binary not found: %s (run make setup)", runnerBin)
			}
			if _, err := os.Stat(wasmMod); os.IsNotExist(err) {
				t.Skipf("wasm module not found: %s (run make wasm)", wasmMod)
			}

			args := rt.args(wasmMod, docPath)
			ctx := context.Background()
			res, err := harness.MeasurePeakRSS(ctx, rt.name, 2, rt.wasiTarget, rt.interp, rt.cbound, runnerBin, args, nil)
			if err != nil {
				t.Fatalf("MeasurePeakRSS: %v", err)
			}
			harness.Record(res)

			if !res.OK {
				t.Errorf("runtime %s failed: %s", rt.name, res.ErrMsg)
			}

			// Validate: stdout must have a valid length-prefixed response.
			if len(res.Stdout) < 4 {
				t.Errorf("stdout too short (%d bytes); expected length-prefixed Automerge doc", len(res.Stdout))
			}

			t.Logf("%s: PeakRSS=%d KiB VmHWM=%d KiB Wall=%d ms OK=%v",
				rt.name, res.PeakRSSKiB, res.VmHWMKiB, res.WallMs, res.OK)
		})
	}
}
