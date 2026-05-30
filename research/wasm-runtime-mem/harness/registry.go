//go:build wasmmem

package harness

// Runtime describes a single WASM runtime under test.
type Runtime struct {
	// Name is a short identifier used in reports (e.g. "wazero", "wasmtime").
	Name string

	// Suite is 1 or 2; suite 1 = baseline host-only, suite 2 = WASM payload.
	Suite int

	// WASITarget is the WASI proposal the runner was compiled for
	// (e.g. "wasip1", "wasip2").
	WASITarget string

	// RunnerBin is the path to the compiled host-runner binary.
	RunnerBin string

	// RunnerArgs returns the argument slice passed to the runner subprocess.
	// mod is the path to the WASM module file; doc is the path to the
	// Automerge document fixture.
	RunnerArgs func(mod, doc string) []string

	// Interpreter indicates a pure-interpreter runtime (no JIT).
	Interpreter bool

	// CBound indicates the runtime uses a C API binding.
	// false for wazero which is pure Go.
	CBound bool
}

// Registry holds the set of runtimes to benchmark.
// Entries are added via init() calls from runner packages, or hardcoded below.
var Registry []Runtime

// Example entries (commented out until runner binaries are built):
//
//	Registry = append(Registry, Runtime{
//	    Name:        "wazero",
//	    Suite:       1,
//	    WASITarget:  "wasip1",
//	    RunnerBin:   "bin/runner-wazero",
//	    RunnerArgs:  func(mod, doc string) []string { return []string{"-mod", mod, "-doc", doc} },
//	    Interpreter: false,
//	    CBound:      false,
//	})
//
//	Registry = append(Registry, Runtime{
//	    Name:        "wasmtime",
//	    Suite:       2,
//	    WASITarget:  "wasip1",
//	    RunnerBin:   "bin/runner-wasmtime",
//	    RunnerArgs:  func(mod, doc string) []string { return []string{"--mod", mod, "--doc", doc} },
//	    Interpreter: false,
//	    CBound:      true,
//	})
//
//	Registry = append(Registry, Runtime{
//	    Name:        "wasmi",
//	    Suite:       2,
//	    WASITarget:  "wasip1",
//	    RunnerBin:   "bin/runner-wasmi",
//	    RunnerArgs:  func(mod, doc string) []string { return []string{mod, doc} },
//	    Interpreter: true,
//	    CBound:      true,
//	})
