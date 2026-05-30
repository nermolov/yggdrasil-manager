//go:build wasmmem

package wasmmem_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nermolov/yggdrasil-manager/research/wasm-runtime-mem/harness"
)

func TestMain(m *testing.M) {
	code := m.Run()

	// Flush results table + JSON artifacts to .cache/bin/
	_, testFile, _, _ := runtime.Caller(0)
	outDir := filepath.Join(filepath.Dir(testFile), ".cache", "bin")
	_ = os.MkdirAll(outDir, 0o755)
	_ = harness.Flush(os.Stdout, outDir)

	os.Exit(code)
}
