//go:build wasmmem

package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/olekukonko/tablewriter"
)

var mu sync.Mutex
var results []Result

// Record adds a result to the in-memory accumulator (thread-safe).
func Record(r Result) {
	mu.Lock()
	defer mu.Unlock()
	results = append(results, r)
}

// Flush writes accumulated results to outDir/results-suite<N>.json (one file per
// suite) and prints a tablewriter table to w with columns:
//
//	Runtime | WASI | Interp? | C-bound? | PeakRSS(KiB) | VmHWM(KiB) | Wall(ms) | OK
func Flush(w io.Writer, outDir string) error {
	mu.Lock()
	snapshot := make([]Result, len(results))
	copy(snapshot, results)
	mu.Unlock()

	// Group by suite number.
	bySuite := make(map[int][]Result)
	for _, r := range snapshot {
		bySuite[r.Suite] = append(bySuite[r.Suite], r)
	}

	// Write one JSON file per suite.
	for suite, rs := range bySuite {
		fname := filepath.Join(outDir, fmt.Sprintf("results-suite%d.json", suite))
		data, err := json.MarshalIndent(rs, "", "  ")
		if err != nil {
			return fmt.Errorf("Flush: marshal suite %d: %w", suite, err)
		}
		if err := os.WriteFile(fname, data, 0o644); err != nil {
			return fmt.Errorf("Flush: write %s: %w", fname, err)
		}
	}

	// Print table. tablewriter v1.x uses Header/Append/Render; formatting
	// defaults are fine for this report.
	table := tablewriter.NewWriter(w)
	table.Header([]string{
		"Runtime", "WASI", "Interp?", "C-bound?",
		"PeakRSS(KiB)", "VmHWM(KiB)", "Wall(ms)", "OK",
	})

	for _, r := range snapshot {
		interp := "no"
		if r.Interpreter {
			interp = "yes"
		}
		cbound := "no"
		if r.CBound {
			cbound = "yes"
		}
		ok := "yes"
		if !r.OK {
			ok = "no"
			if r.ErrMsg != "" {
				ok = "no: " + r.ErrMsg
			}
		}
		if err := table.Append([]string{
			r.Runtime,
			r.WASITarget,
			interp,
			cbound,
			strconv.FormatInt(r.PeakRSSKiB, 10),
			strconv.FormatInt(r.VmHWMKiB, 10),
			strconv.FormatInt(r.WallMs, 10),
			ok,
		}); err != nil {
			return fmt.Errorf("Flush: append row: %w", err)
		}
	}

	if err := table.Render(); err != nil {
		return fmt.Errorf("Flush: render table: %w", err)
	}
	return nil
}
